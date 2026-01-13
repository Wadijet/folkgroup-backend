package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/cta"
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
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Logic nghiệp vụ phức tạp (workflow trigger notification):
//    - Tìm routing rules cho eventType, domain, severity
//    - Tìm channels phù hợp với rules
//    - Tạo notification queue items cho từng channel
//    - Có thể trigger nhiều notifications cùng lúc (một event → nhiều channels)
// 2. Cross-service operations:
//    - Sử dụng NotificationRoutingService để tìm rules
//    - Sử dụng NotificationChannelService để tìm channels
//    - Sử dụng NotificationQueueService để tạo queue items
//    - Logic phức tạp: infer domain và severity từ eventType
// 3. Response format đặc biệt:
//    - Trả về thông tin về eventType, số lượng queue items đã tạo
//    - Không phải format CRUD chuẩn (tạo một document)
// 4. Tracking và logging:
//    - Lấy requestID, clientIP, userID để tracking
//    - Log chi tiết quá trình trigger notification
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì đây là workflow action (trigger) với logic nghiệp vụ phức tạp,
//           cross-service operations, và có thể tạo nhiều queue items từ một event
func (h *NotificationTriggerHandler) HandleTriggerNotification(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		// Lấy thông tin request để tracking
		// Request ID middleware set vào Locals với key "requestid" (lowercase)
		var requestID string
		if rid := c.Locals("requestid"); rid != nil {
			if ridStr, ok := rid.(string); ok {
				requestID = ridStr
			}
		}
		// Fallback: lấy từ header nếu không có trong Locals
		if requestID == "" {
			requestID = c.Get("X-Request-ID")
		}
		// Fallback: lấy từ response header nếu middleware đã set
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

		// Infer Domain và Severity từ EventType
		domain := notification.GetDomainFromEventType(req.EventType)
		severity := notification.GetSeverityFromEventType(req.EventType)

		// Lấy organizationID từ context (nếu có) để filter rules
		var organizationID *primitive.ObjectID
		if orgIDStr, ok := c.Locals("active_organization_id").(string); ok && orgIDStr != "" {
			if orgID, err := primitive.ObjectIDFromHex(orgIDStr); err == nil {
				organizationID = &orgID
			}
		}

		// Tìm routes cho eventType với domain và severity
		// Lưu ý: Chỉ tìm rules của organization trigger event (hoặc system rules)
		log := logger.GetAppLogger()
		// Đã tắt log Info để giảm log

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

		// Đã tắt log Info và Debug để giảm log

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
		queueItems := make([]*models.DeliveryQueueItem, 0)
		renderErrors := make([]string, 0) // Thu thập lỗi render để trả về cho client
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
				// Thu thập lỗi render để trả về cho client
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

			// Đã tắt log Info để giảm log (recipients có thể chứa thông tin nhạy cảm)

			// Tính Priority và MaxRetries từ Severity
			priority := notification.GetPriorityFromSeverity(severity)
			maxRetries := notification.GetMaxRetriesFromSeverity(severity)

			// Tạo queue item cho mỗi recipient (với content đã render và sender config đã encrypt)
			for _, recipient := range recipients {
				queueItems = append(queueItems, &models.DeliveryQueueItem{
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
					MaxRetries:          maxRetries, // Tính từ Severity
					Priority:            priority,   // Tính từ Severity
					CreatedAt:           time.Now().Unix(),
					UpdatedAt:           time.Now().Unix(),
				})
			}
		}

		// Nếu có lỗi render, trả về lỗi cho client
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

		// Enqueue items
		if len(queueItems) > 0 {
			// Đã tắt log Info để giảm log

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

			// Đã tắt log Info để giảm log
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
	// Lấy System Organization ID để tìm system sender
	systemOrgID, err := cta.GetSystemOrganizationID(ctx)
	if err != nil {
		return nil, primitive.NilObjectID, fmt.Errorf("failed to get system organization ID: %w", err)
	}

	filter := bson.M{
		"channelType": channel.ChannelType,
		"isActive":    true,
		"$or": []bson.M{
			{"ownerOrganizationId": organizationID},
			{"ownerOrganizationId": systemOrgID}, // System sender (thuộc System Organization)
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
		// Fallback về system sender (thuộc System Organization)
		for _, s := range senders {
			if s.OwnerOrganizationID != nil && s.OwnerOrganizationID.Hex() == systemOrgID.Hex() {
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
