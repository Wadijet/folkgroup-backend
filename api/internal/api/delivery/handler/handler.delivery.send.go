// Package deliveryhdl chứa HTTP handler cho domain Delivery (send, tracking).
// File: basehdl.delivery.send.go - giữ tên cấu trúc cũ (basehdl.<domain>.<entity>.go).
package deliveryhdl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	deliverydto "meta_commerce/internal/api/delivery/dto"
	deliverymodels "meta_commerce/internal/api/delivery/models"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifsvc "meta_commerce/internal/api/notification/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
	"meta_commerce/internal/delivery"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/notification"

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
	return &DeliverySendHandler{queue: queue}, nil
}

// HandleSend xử lý request gửi notification trực tiếp
func (h *DeliverySendHandler) HandleSend(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var req deliverydto.DeliverySendRequest
		if err := c.Bind().Body(&req); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeValidationFormat.Code,
				"message": fmt.Sprintf("Dữ liệu gửi lên không đúng định dạng JSON. Chi tiết: %v", err),
				"status":  "error",
			})
			return nil
		}
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

		ctaJSONs := make([]string, 0, len(req.CTAs))
		for _, cta := range req.CTAs {
			ctaJSON, err := json.Marshal(cta)
			if err != nil {
				continue
			}
			ctaJSONs = append(ctaJSONs, string(ctaJSON))
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
		sender, senderID, err := findSenderForChannelType(c.Context(), senderService, req.ChannelType, orgID)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không tìm thấy sender cho channelType '%s': %v", req.ChannelType, err),
				"status":  "error",
			})
			return nil
		}

		var encryptedSenderConfig string
		if sender != nil {
			senderConfigJSON, err := json.Marshal(sender)
			if err == nil {
				encryptedSenderConfig, err = delivery.EncryptSenderConfig(senderConfigJSON)
				if err != nil {
					logger.GetAppLogger().WithError(err).WithField("senderId", sender.ID.Hex()).Warn("📦 [DELIVERY] Không thể encrypt sender config, sẽ dùng fallback")
					encryptedSenderConfig = ""
				}
			}
		}

		severity := notification.GetSeverityFromEventType(req.EventType)
		priority := notification.GetPriorityFromSeverity(severity)
		maxRetries := notification.GetMaxRetriesFromSeverity(severity)

		queueItem := &deliverymodels.DeliveryQueueItem{
			ID:                  primitive.NewObjectID(),
			EventType:           req.EventType,
			OwnerOrganizationID: orgID,
			SenderID:            senderID,
			SenderConfig:        encryptedSenderConfig,
			ChannelType:         req.ChannelType,
			Recipient:           req.Recipient,
			Subject:             req.Subject,
			Content:             req.Content,
			CTAs:                ctaJSONs,
			Payload:             req.Metadata,
			Status:              "pending",
			RetryCount:          0,
			MaxRetries:          maxRetries,
			Priority:            priority,
			CreatedAt:           time.Now().Unix(),
			UpdatedAt:           time.Now().Unix(),
		}

		if err := h.queue.Enqueue(c.Context(), []*deliverymodels.DeliveryQueueItem{queueItem}); err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code":    common.ErrCodeBusinessOperation.Code,
				"message": fmt.Sprintf("Không thể thêm vào queue: %v", err),
				"status":  "error",
			})
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Notification đã được thêm vào queue",
			"data": deliverydto.DeliverySendResponse{
				MessageID: queueItem.ID.Hex(),
				Status:    "queued",
				QueuedAt:  queueItem.CreatedAt,
			},
			"status": "success",
		})
		return nil
	})
}

func findSenderForChannelType(ctx context.Context, senderService *notifsvc.NotificationSenderService, channelType string, organizationID primitive.ObjectID) (*notifmodels.NotificationChannelSender, primitive.ObjectID, error) {
	filter := bson.M{
		"channelType": channelType,
		"isActive":    true,
		"$or": []bson.M{
			{"ownerOrganizationId": organizationID},
			{"ownerOrganizationId": nil},
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
			if s.OwnerOrganizationID == nil {
				return &s, s.ID, nil
			}
		}
		return &senders[0], senders[0].ID, nil
	}
	return nil, primitive.NilObjectID, fmt.Errorf("no active sender found for channel type %s", channelType)
}
