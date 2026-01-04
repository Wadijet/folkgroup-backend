package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/delivery"
	"meta_commerce/core/logger"
	"meta_commerce/core/notification"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationTriggerHandler xử lý việc trigger notification (Hệ thống 2)
type NotificationTriggerHandler struct {
	router   *notification.Router
	template *notification.Template
	queue    *delivery.Queue
}

// NewNotificationTriggerHandler tạo mới NotificationTriggerHandler
func NewNotificationTriggerHandler() (*NotificationTriggerHandler, error) {
	router, err := notification.NewRouter()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification router: %w", err)
	}

	template, err := notification.NewTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to create notification template: %w", err)
	}

	queue, err := delivery.NewQueue()
	if err != nil {
		return nil, fmt.Errorf("failed to create delivery queue: %w", err)
	}

	return &NotificationTriggerHandler{
		router:   router,
		template: template,
		queue:    queue,
	}, nil
}

// TriggerNotificationRequest là request body để trigger notification
type TriggerNotificationRequest struct {
	EventType string                 `json:"eventType" validate:"required"`
	Payload   map[string]interface{} `json:"payload" validate:"required"`
}

// HandleTriggerNotification xử lý request trigger notification
func (h *NotificationTriggerHandler) HandleTriggerNotification(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		var req TriggerNotificationRequest
		if err := c.Bind().Body(&req); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON. Chi tiết: %v", err),
				"status":  "error",
			})
			return nil
		}

		// Validate
		if req.EventType == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "eventType không được để trống",
				"status":  "error",
			})
			return nil
		}

		if req.Payload == nil {
			req.Payload = make(map[string]interface{})
		}

		// Tìm routes cho eventType
		routes, err := h.router.FindRoutes(c.Context(), req.EventType)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể tìm routes cho eventType '%s': %v", req.EventType, err),
				"status":  "error",
			})
			return nil
		}

		if len(routes) == 0 {
			c.JSON(map[string]interface{}{
				"message":   "Không có routing rule nào cho eventType này",
				"eventType": req.EventType,
				"queued":    0,
			})
			return nil
		}

		// Lấy baseURL từ request hoặc dùng default
		baseURL := fmt.Sprintf("%s://%s", c.Protocol(), c.Hostname())
		if c.Port() != "" && c.Port() != "80" && c.Port() != "443" {
			baseURL = fmt.Sprintf("%s://%s:%s", c.Protocol(), c.Hostname(), c.Port())
		}

		// Đảm bảo baseUrl có trong payload để render {{baseUrl}} trong template
		if _, exists := req.Payload["baseUrl"]; !exists {
			req.Payload["baseUrl"] = baseURL
		}

		// Tạo queue items cho mỗi route
		queueItems := make([]*models.NotificationQueueItem, 0)
		channelService, err := services.NewNotificationChannelService()
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể tạo channel service: %v", err),
				"status":  "error",
			})
			return nil
		}

		senderService, err := services.NewNotificationSenderService()
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể tạo sender service: %v", err),
				"status":  "error",
			})
			return nil
		}

		log := logger.GetAppLogger()
		for _, route := range routes {
			// Lấy channel để biết recipients và channel type
			channel, err := channelService.FindOneById(c.Context(), route.ChannelID)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"channelId": route.ChannelID.Hex(),
					"eventType": req.EventType,
				}).Warn("🔔 [NOTIFICATION] Không tìm thấy channel, bỏ qua route")
				continue
			}

			// Tìm template cho eventType và channelType
			template, err := h.template.FindTemplate(c.Context(), req.EventType, channel.ChannelType, route.OrganizationID)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"eventType":      req.EventType,
					"channelType":    channel.ChannelType,
					"organizationId": route.OrganizationID.Hex(),
				}).Warn("🔔 [NOTIFICATION] Không tìm thấy template, bỏ qua route")
				continue
			}

			// Render template (subject, content, CTAs)
			rendered, err := h.template.Render(c.Context(), template, req.Payload, route.OrganizationID, baseURL)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"eventType":      req.EventType,
					"channelType":    channel.ChannelType,
					"organizationId": route.OrganizationID.Hex(),
					"templateId":     template.ID.Hex(),
				}).Error("🔔 [NOTIFICATION] Lỗi khi render template, bỏ qua route")
				continue
			}

			// Convert CTAs sang JSON strings
			ctaJSONs := make([]string, 0, len(rendered.CTAs))
			for _, cta := range rendered.CTAs {
				ctaJSON, err := json.Marshal(cta)
				if err != nil {
					continue
				}
				ctaJSONs = append(ctaJSONs, string(ctaJSON))
			}

			// Xác định recipients dựa trên channel type
			var recipients []string
			switch channel.ChannelType {
			case "email":
				recipients = channel.Recipients
			case "telegram":
				recipients = channel.ChatIDs
				if len(recipients) == 0 {
					log.WithFields(map[string]interface{}{
						"channelId":   channel.ID.Hex(),
						"channelType": channel.ChannelType,
					}).Warn("🔔 [NOTIFICATION] Telegram channel không có ChatIDs, bỏ qua")
					continue
				}
			case "webhook":
				if channel.WebhookURL != "" {
					recipients = []string{channel.WebhookURL}
				}
			default:
				log.WithField("channelType", channel.ChannelType).Warn("🔔 [NOTIFICATION] Channel type không được hỗ trợ, bỏ qua")
				continue
			}

			// Tìm sender cho channel (Option C: Hybrid - tìm sender và encrypt config)
			sender, senderID, err := findSenderForChannel(c.Context(), senderService, &channel, route.OrganizationID)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"channelId":   channel.ID.Hex(),
					"channelType": channel.ChannelType,
				}).Warn("🔔 [NOTIFICATION] Không tìm thấy sender, bỏ qua route")
				continue
			}

			// Encrypt sender config (fast path)
			var encryptedSenderConfig string
			if sender != nil {
				senderConfigJSON, err := json.Marshal(sender)
				if err == nil {
					encryptedSenderConfig, err = delivery.EncryptSenderConfig(senderConfigJSON)
					if err != nil {
						log.WithError(err).WithField("senderId", sender.ID.Hex()).Warn("🔔 [NOTIFICATION] Không thể encrypt sender config, sẽ dùng fallback")
						encryptedSenderConfig = "" // Fallback về query từ SenderID
					}
				}
			}

			log.WithFields(map[string]interface{}{
				"eventType":         req.EventType,
				"channelType":       channel.ChannelType,
				"channelId":         channel.ID.Hex(),
				"senderId":           senderID.Hex(),
				"hasSenderConfig":   encryptedSenderConfig != "",
				"recipientCount":    len(recipients),
				"ctaCount":          len(rendered.CTAs),
			}).Info("🔔 [NOTIFICATION] Đã render template thành công, tạo queue items")

			// Tạo queue item cho mỗi recipient (với content đã render và sender config đã encrypt)
			for _, recipient := range recipients {
				queueItems = append(queueItems, &models.NotificationQueueItem{
					ID:                  primitive.NewObjectID(),
					EventType:           req.EventType,
					OwnerOrganizationID: route.OrganizationID,
					SenderID:            senderID,
					SenderConfig:        encryptedSenderConfig, // Optional, encrypted (fast path)
					ChannelType:         channel.ChannelType,
					Recipient:           recipient,
					Subject:             rendered.Subject,
					Content:             rendered.Content,
					CTAs:                ctaJSONs,
					Payload:             req.Payload,
					Status:              "pending",
					RetryCount:          0,
					MaxRetries:          3,
					CreatedAt:           time.Now().Unix(),
					UpdatedAt:           time.Now().Unix(),
				})
			}
		}

		// Enqueue items
		if len(queueItems) > 0 {
			err = h.queue.Enqueue(c.Context(), queueItems)
			if err != nil {
				c.Status(common.StatusInternalServerError).JSON(fiber.Map{
					"code":    common.ErrCodeBusinessOperation.Code,
					"message": fmt.Sprintf("Không thể thêm items vào queue: %v", err),
					"status":  "error",
				})
				return nil
			}
		}

		c.JSON(map[string]interface{}{
			"message":   "Notification đã được thêm vào queue",
			"eventType": req.EventType,
			"queued":    len(queueItems),
		})
		return nil
	})
}

// findSenderForChannel tìm sender cho channel (logic tương tự như delivery processor cũ)
// Trả về: sender, senderID, error
func findSenderForChannel(ctx context.Context, senderService *services.NotificationSenderService, channel *models.NotificationChannel, organizationID primitive.ObjectID) (*models.NotificationChannelSender, primitive.ObjectID, error) {
	// 1. Nếu channel có SenderIDs, dùng sender đầu tiên active
	if len(channel.SenderIDs) > 0 {
		for _, senderID := range channel.SenderIDs {
			sender, err := senderService.FindOneById(ctx, senderID)
			if err == nil && sender.IsActive && sender.ChannelType == channel.ChannelType {
				return &sender, senderID, nil
			}
		}
	}

	// 2. Tìm sender active cho organization và channel type
	filter := bson.M{
		"channelType": channel.ChannelType,
		"isActive":    true,
		"$or": []bson.M{
			{"ownerOrganizationId": organizationID},
			{"ownerOrganizationId": nil}, // System sender
		},
	}

	senders, err := senderService.Find(ctx, filter, nil)
	if err == nil && len(senders) > 0 {
		// Ưu tiên organization-specific sender
		for _, s := range senders {
			if s.OwnerOrganizationID != nil && s.OwnerOrganizationID.Hex() == organizationID.Hex() {
				return &s, s.ID, nil
			}
		}
		// Fallback về system sender
		for _, s := range senders {
			if s.OwnerOrganizationID == nil {
				return &s, s.ID, nil
			}
		}
		// Nếu không có system sender, dùng sender đầu tiên
		return &senders[0], senders[0].ID, nil
	}

	return nil, primitive.NilObjectID, fmt.Errorf("no active sender found for channel type %s", channel.ChannelType)
}

// SafeHandlerWrapper wrapper để xử lý errors
func SafeHandlerWrapper(c fiber.Ctx, fn func() error) error {
	if err := fn(); err != nil {
		return err
	}
	return nil
}
