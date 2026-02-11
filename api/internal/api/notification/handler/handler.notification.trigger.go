package notifhdl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	deliverymodels "meta_commerce/internal/api/delivery/models"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifsvc "meta_commerce/internal/api/notification/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
	"meta_commerce/internal/cta"
	"meta_commerce/internal/delivery"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/notification"

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
	return basehdl.SafeHandlerWrapper(c, func() error {
		var requestID string
		if rid := c.Locals("requestid"); rid != nil {
			if ridStr, ok := rid.(string); ok {
				requestID = ridStr
			}
		}
		if requestID == "" {
			requestID = c.Get("X-Request-ID")
		}
		if requestID == "" {
			requestID = c.GetRespHeader("X-Request-ID")
		}
		clientIP := c.IP()
		userID := ""
		if userIDStr, ok := c.Locals("user_id").(string); ok {
			userID = userIDStr
		}

		var req TriggerNotificationRequest
		if err := c.Bind().Body(&req); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON. Chi tiết: %v", err),
				"status":  "error",
			})
			return nil
		}

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

		domain := notification.GetDomainFromEventType(req.EventType)
		severity := notification.GetSeverityFromEventType(req.EventType)

		var organizationID *primitive.ObjectID
		if orgIDStr, ok := c.Locals("active_organization_id").(string); ok && orgIDStr != "" {
			if orgID, err := primitive.ObjectIDFromHex(orgIDStr); err == nil {
				organizationID = &orgID
			}
		}

		log := logger.GetAppLogger()

		routes, err := h.router.FindRoutes(c.Context(), req.EventType, domain, severity, organizationID)
		if err != nil {
			log.WithError(err).WithField("eventType", req.EventType).Error("🔔 [NOTIFICATION] Lỗi khi tìm routes")
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể tìm routes cho eventType '%s': %v", req.EventType, err),
				"status":  "error",
			})
			return nil
		}

		if len(routes) == 0 {
			log.WithField("eventType", req.EventType).Warn("🔔 [NOTIFICATION] Không có routes nào cho eventType này")
			c.Status(common.StatusOK).JSON(fiber.Map{
				"code":    common.StatusOK,
				"message": "Không có routing rule nào cho eventType này",
				"data": map[string]interface{}{
					"eventType": req.EventType,
					"queued":    0,
				},
				"status": "success",
			})
			return nil
		}

		baseURL := fmt.Sprintf("%s://%s", c.Protocol(), c.Hostname())
		if c.Port() != "" && c.Port() != "80" && c.Port() != "443" {
			baseURL = fmt.Sprintf("%s://%s:%s", c.Protocol(), c.Hostname(), c.Port())
		}

		if _, exists := req.Payload["baseUrl"]; !exists {
			req.Payload["baseUrl"] = baseURL
		}

		queueItems := make([]*deliverymodels.DeliveryQueueItem, 0)
		renderErrors := make([]string, 0)
		channelService, err := notifsvc.NewNotificationChannelService()
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể tạo channel service: %v", err),
				"status":  "error",
			})
			return nil
		}

		senderService, err := notifsvc.NewNotificationSenderService()
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể tạo sender service: %v", err),
				"status":  "error",
			})
			return nil
		}

		for _, route := range routes {
			channel, err := channelService.FindOneById(c.Context(), route.ChannelID)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"channelId": route.ChannelID.Hex(),
					"eventType": req.EventType,
				}).Warn("🔔 [NOTIFICATION] Không tìm thấy channel, bỏ qua route")
				continue
			}

			template, err := h.template.FindTemplate(c.Context(), req.EventType, channel.ChannelType, route.OrganizationID)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"eventType":      req.EventType,
					"channelType":    channel.ChannelType,
					"organizationId": route.OrganizationID.Hex(),
				}).Warn("🔔 [NOTIFICATION] Không tìm thấy template, bỏ qua route")
				continue
			}

			rendered, err := h.template.Render(c.Context(), template, req.Payload, route.OrganizationID, baseURL)
			if err != nil {
				errorMsg := fmt.Sprintf("Lỗi khi render template cho channel %s (type: %s, templateId: %s): %v",
					channel.ID.Hex(), channel.ChannelType, template.ID.Hex(), err)
				renderErrors = append(renderErrors, errorMsg)
				log.WithError(err).WithFields(map[string]interface{}{
					"eventType":      req.EventType,
					"channelType":    channel.ChannelType,
					"organizationId": route.OrganizationID.Hex(),
					"templateId":     template.ID.Hex(),
				}).Error("🔔 [NOTIFICATION] Lỗi khi render template")
				continue
			}

			ctaJSONs := make([]string, 0, len(rendered.CTAs))
			for _, cta := range rendered.CTAs {
				ctaJSON, err := json.Marshal(cta)
				if err != nil {
					continue
				}
				ctaJSONs = append(ctaJSONs, string(ctaJSON))
			}

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

			sender, senderID, err := findSenderForChannel(c.Context(), senderService, &channel, route.OrganizationID)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"channelId":   channel.ID.Hex(),
					"channelType": channel.ChannelType,
				}).Warn("🔔 [NOTIFICATION] Không tìm thấy sender, bỏ qua route")
				continue
			}

			var encryptedSenderConfig string
			if sender != nil {
				senderConfigJSON, err := json.Marshal(sender)
				if err == nil {
					encryptedSenderConfig, err = delivery.EncryptSenderConfig(senderConfigJSON)
					if err != nil {
						log.WithError(err).WithField("senderId", sender.ID.Hex()).Warn("🔔 [NOTIFICATION] Không thể encrypt sender config, sẽ dùng fallback")
						encryptedSenderConfig = ""
					}
				}
			}

			priority := notification.GetPriorityFromSeverity(severity)
			maxRetries := notification.GetMaxRetriesFromSeverity(severity)

			for _, recipient := range recipients {
				queueItems = append(queueItems, &deliverymodels.DeliveryQueueItem{
					ID:                  primitive.NewObjectID(),
					EventType:           req.EventType,
					OwnerOrganizationID: route.OrganizationID,
					SenderID:            senderID,
					SenderConfig:        encryptedSenderConfig,
					ChannelType:         channel.ChannelType,
					Recipient:           recipient,
					Subject:             rendered.Subject,
					Content:             rendered.Content,
					CTAs:                ctaJSONs,
					Payload:             req.Payload,
					Status:              "pending",
					RetryCount:          0,
					MaxRetries:          maxRetries,
					Priority:            priority,
					CreatedAt:           time.Now().Unix(),
					UpdatedAt:           time.Now().Unix(),
				})
			}
		}

		if len(renderErrors) > 0 {
			errorMessage := fmt.Sprintf("Không thể render template cho %d route(s). Chi tiết: %s",
				len(renderErrors), strings.Join(renderErrors, "; "))
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": errorMessage,
				"errors":  renderErrors,
				"status":  "error",
			})
			return nil
		}

		if len(queueItems) > 0 {
			err = h.queue.Enqueue(c.Context(), queueItems)
			if err != nil {
				log.WithError(err).WithFields(map[string]interface{}{
					"requestId":  requestID,
					"clientIp":   clientIP,
					"userId":     userID,
					"eventType":  req.EventType,
					"queueItems": len(queueItems),
				}).Error("🔔 [NOTIFICATION] Lỗi khi enqueue items")
				c.Status(common.StatusInternalServerError).JSON(fiber.Map{
					"code":    common.ErrCodeBusinessOperation.Code,
					"message": fmt.Sprintf("Không thể thêm items vào queue: %v", err),
					"status":  "error",
				})
				return nil
			}
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Notification đã được thêm vào queue",
			"data": map[string]interface{}{
				"eventType": req.EventType,
				"queued":    len(queueItems),
			},
			"status": "success",
		})
		return nil
	})
}

// findSenderForChannel tìm sender cho channel (logic tương tự như delivery processor cũ)
func findSenderForChannel(ctx context.Context, senderService *notifsvc.NotificationSenderService, channel *notifmodels.NotificationChannel, organizationID primitive.ObjectID) (*notifmodels.NotificationChannelSender, primitive.ObjectID, error) {
	if len(channel.SenderIDs) > 0 {
		for _, senderID := range channel.SenderIDs {
			sender, err := senderService.FindOneById(ctx, senderID)
			if err == nil && sender.IsActive && sender.ChannelType == channel.ChannelType {
				return &sender, senderID, nil
			}
		}
	}

	systemOrgID, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		return nil, primitive.NilObjectID, fmt.Errorf("failed to get system organization ID: %w", err)
	}

	filter := bson.M{
		"channelType": channel.ChannelType,
		"isActive":    true,
		"$or": []bson.M{
			{"ownerOrganizationId": organizationID},
			{"ownerOrganizationId": systemOrgID},
		},
	}

	senders, err := senderService.Find(ctx, filter, nil)
	if err == nil && len(senders) > 0 {
		for _, s := range senders {
			if s.OwnerOrganizationID != nil && s.OwnerOrganizationID.Hex() == organizationID.Hex() {
				return &s, s.ID, nil
			}
		}
		for _, s := range senders {
			if s.OwnerOrganizationID != nil && s.OwnerOrganizationID.Hex() == systemOrgID.Hex() {
				return &s, s.ID, nil
			}
		}
		return &senders[0], senders[0].ID, nil
	}

	return nil, primitive.NilObjectID, fmt.Errorf("no active sender found for channel type %s", channel.ChannelType)
}
