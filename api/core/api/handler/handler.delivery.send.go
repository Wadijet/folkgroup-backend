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

// DeliverySendHandler xử lý gửi notification trực tiếp (Hệ thống 1)
type DeliverySendHandler struct {
	queue *delivery.Queue
}

// NewDeliverySendHandler tạo mới DeliverySendHandler
func NewDeliverySendHandler() (*DeliverySendHandler, error) {
	queue, err := delivery.NewQueue()
	if err != nil {
		return nil, fmt.Errorf("failed to create delivery queue: %w", err)
	}

	return &DeliverySendHandler{
		queue: queue,
	}, nil
}

// DeliverySendRequest là request để gửi notification trực tiếp
type DeliverySendRequest struct {
	ChannelType string                 `json:"channelType" validate:"required"`
	Recipient   string                 `json:"recipient" validate:"required"`
	Subject     string                 `json:"subject,omitempty"`
	Content     string                 `json:"content" validate:"required"`
	CTAs        []DeliverySendCTA      `json:"ctas,omitempty"`
	EventType   string                 `json:"eventType,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DeliverySendCTA là CTA đã render
type DeliverySendCTA struct {
	Label       string `json:"label"`
	Action      string `json:"action"`      // URL (có thể đã có tracking URL)
	OriginalURL string `json:"originalUrl"` // Original URL (nếu có)
	Style       string `json:"style,omitempty"`
}

// DeliverySendResponse là response sau khi gửi
type DeliverySendResponse struct {
	MessageID string `json:"messageId"` // History ID
	Status    string `json:"status"`    // queued
	QueuedAt  int64  `json:"queuedAt"`
}

// HandleSend xử lý request gửi notification trực tiếp
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Logic nghiệp vụ phức tạp (gửi notification trực tiếp):
//    - Tìm sender cho channelType
//    - Convert CTAs sang JSON strings
//    - Tạo DeliveryHistory record
//    - Gửi notification qua sender (email/telegram/webhook)
//    - Update DeliveryHistory với kết quả gửi
// 2. Cross-service operations:
//    - Sử dụng NotificationSenderService để tìm sender
//    - Sử dụng DeliveryHistoryService để tạo history
//    - Gọi sender.Send() để gửi notification thực tế
// 3. Real-time operation:
//    - Gửi notification ngay lập tức (không queue)
//    - Có thể block cho đến khi gửi xong
//    - Update history với status và kết quả
// 4. Response format đặc biệt:
//    - Trả về thông tin về notification đã gửi
//    - Có thể có error nếu gửi thất bại
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì đây là workflow action (send) với logic nghiệp vụ phức tạp,
//           cross-service operations, và real-time gửi notification
func (h *DeliverySendHandler) HandleSend(c fiber.Ctx) error {
	return SafeHandlerWrapper(c, func() error {
		var req DeliverySendRequest
		if err := c.Bind().Body(&req); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON. Chi tiết: %v", err),
				"status":  "error",
			})
			return nil
		}

		// Validate
		if req.ChannelType == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "channelType không được để trống",
				"status":  "error",
			})
			return nil
		}

		if req.Content == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "content không được để trống",
				"status":  "error",
			})
			return nil
		}

		// Lấy organization ID từ context
		orgIDStr, ok := c.Locals("active_organization_id").(string)
		if !ok || orgIDStr == "" {
			c.Status(common.StatusUnauthorized).JSON(fiber.Map{
				"code":    common.ErrCodeAuthRole.Code,
				"message": "Organization context required",
				"status":  "error",
			})
			return nil
		}

		orgID, err := primitive.ObjectIDFromHex(orgIDStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": "Invalid organization ID",
				"status":  "error",
			})
			return nil
		}

		// Convert CTAs sang JSON strings
		ctaJSONs := make([]string, 0, len(req.CTAs))
		for _, cta := range req.CTAs {
			ctaJSON, err := json.Marshal(cta)
			if err != nil {
				continue
			}
			ctaJSONs = append(ctaJSONs, string(ctaJSON))
		}

		// Tìm sender cho channelType (tương tự Notification System)
		senderService, err := services.NewNotificationSenderService()
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể tạo sender service: %v", err),
				"status":  "error",
			})
			return nil
		}

		sender, senderID, err := findSenderForChannelType(c.Context(), senderService, req.ChannelType, orgID)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không tìm thấy sender cho channelType '%s': %v", req.ChannelType, err),
				"status":  "error",
			})
			return nil
		}

		// Encrypt sender config (fast path - Option C Hybrid)
		var encryptedSenderConfig string
		if sender != nil {
			senderConfigJSON, err := json.Marshal(sender)
			if err == nil {
				encryptedSenderConfig, err = delivery.EncryptSenderConfig(senderConfigJSON)
				if err != nil {
					logger.GetAppLogger().WithError(err).WithField("senderId", sender.ID.Hex()).Warn("📦 [DELIVERY] Không thể encrypt sender config, sẽ dùng fallback")
					encryptedSenderConfig = "" // Fallback về query từ SenderID
				}
			}
		}

		// Infer Severity từ EventType để tính Priority và MaxRetries
		severity := notification.GetSeverityFromEventType(req.EventType)
		priority := notification.GetPriorityFromSeverity(severity)
		maxRetries := notification.GetMaxRetriesFromSeverity(severity)

		// Tạo queue item
		queueItem := &models.DeliveryQueueItem{
			ID:                  primitive.NewObjectID(),
			EventType:           req.EventType,
			OwnerOrganizationID: orgID,
			SenderID:            senderID,
			SenderConfig:        encryptedSenderConfig, // Optional, encrypted (fast path)
			ChannelType:         req.ChannelType,
			Recipient:           req.Recipient,
			Subject:             req.Subject,
			Content:             req.Content,
			CTAs:                ctaJSONs,
			Payload:             req.Metadata,
			Status:              "pending",
			RetryCount:          0,
			MaxRetries:          maxRetries, // Tính từ Severity
			Priority:            priority,    // Tính từ Severity
			CreatedAt:           time.Now().Unix(),
			UpdatedAt:           time.Now().Unix(),
		}

		// Enqueue
		err = h.queue.Enqueue(c.Context(), []*models.DeliveryQueueItem{queueItem})
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể thêm vào queue: %v", err),
				"status":  "error",
			})
			return nil
		}

		// Response (messageId sẽ là history ID sau khi processor xử lý)
		// Tạm thời dùng queueItem ID
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Notification đã được thêm vào queue",
			"data": DeliverySendResponse{
				MessageID: queueItem.ID.Hex(),
				Status:    "queued",
				QueuedAt:  queueItem.CreatedAt,
			},
			"status": "success",
		})
		return nil
	})
}

// findSenderForChannelType tìm sender cho channelType và organization (dùng cho direct delivery)
// Trả về: sender, senderID, error
func findSenderForChannelType(ctx context.Context, senderService *services.NotificationSenderService, channelType string, organizationID primitive.ObjectID) (*models.NotificationChannelSender, primitive.ObjectID, error) {
	// Tìm sender active cho organization và channel type
	filter := bson.M{
		"channelType": channelType,
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

	return nil, primitive.NilObjectID, fmt.Errorf("no active sender found for channel type %s", channelType)
}
