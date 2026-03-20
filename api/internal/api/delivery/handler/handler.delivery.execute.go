// Package deliveryhdl — Handler cho Execution Engine.
//
// HandleExecute POST /executor/execute — nhận ExecutionActionInput, validate, route, thực thi.
// Gate: Mọi action phải qua Executor. Gọi POST /executor/actions/propose hoặc POST /ai-decision/execute.
// Khi DELIVERY_ALLOW_DIRECT_USE=true (env) cho phép gọi trực tiếp (deprecated, backward compat).
package deliveryhdl

import (
	"fmt"
	"os"
	"strings"
	"time"

	deliverydto "meta_commerce/internal/api/delivery/dto"
	deliverymodels "meta_commerce/internal/api/delivery/models"
	notifsvc "meta_commerce/internal/api/notification/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/notification"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ExecuteRequest body cho POST /delivery/execute.
type ExecuteRequest struct {
	Actions []deliverydto.ExecutionActionInput `json:"actions"`
}

// ExecuteResult kết quả thực thi từng action.
type ExecuteResult struct {
	ActionID   string `json:"actionId"`
	ActionType string `json:"actionType"`
	Status     string `json:"status"` // queued | rejected | unsupported
	Message    string `json:"message,omitempty"`
	QueueID    string `json:"queueId,omitempty"`
}

// HandleExecute POST /delivery/execute — Execution Engine: nhận actions, route, thực thi.
// Gate: Chỉ nhận từ Executor. Gọi Propose/ai-decision/execute thay vì gọi trực tiếp.
func (h *DeliverySendHandler) HandleExecute(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		// Gate: mọi action phải qua Executor — không có đường đi tắt
		if !allowDirectDeliveryUse() {
			c.Status(403).JSON(fiber.Map{
				"code":    common.ErrCodeValidationInput.Code,
				"message": "Mọi action phải qua Executor. Gọi POST /executor/actions/propose hoặc POST /ai-decision/execute. (DELIVERY_ALLOW_DIRECT_USE=true để tạm cho phép — deprecated)",
				"status":  "error",
			})
			return nil
		}
		logger.GetAppLogger().Warn("DELIVERY_ALLOW_DIRECT_USE=true — deprecated, mọi action nên qua Executor")

		var req ExecuteRequest
		if err := c.Bind().JSON(&req); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Body JSON không hợp lệ", "status": "error",
			})
			return nil
		}
		if len(req.Actions) == 0 {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "actions không được rỗng", "status": "error",
			})
			return nil
		}
		orgIDStr, ok := c.Locals("active_organization_id").(string)
		if !ok || orgIDStr == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		orgID, err := primitive.ObjectIDFromHex(orgIDStr)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Organization ID không hợp lệ", "status": "error",
			})
			return nil
		}

		// Phase 3 Delivery Gate: Validate source=APPROVAL_GATE và actionPendingId (ActionID) khi cho phép direct
		if code, msg := validateDeliveryExecuteActionsForGate(req.Actions); code != 0 {
			c.Status(code).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": msg, "status": "error",
			})
			return nil
		}

		results := make([]ExecuteResult, 0, len(req.Actions))
		var queueItems []*deliverymodels.DeliveryQueueItem

		for _, act := range req.Actions {
			if act.ActionType == deliverydto.ActionTypeSendMessage {
				recipient := ""
				if act.Payload != nil {
					if r, ok := act.Payload["recipient"].(string); ok {
						recipient = r
					}
				}
				if recipient == "" {
					recipient = act.Target.CustomerID // fallback: dùng customerId làm recipient (cần resolve sau)
				}
				content := ""
				if act.Payload != nil {
					if ct, ok := act.Payload["content"].(string); ok {
						content = ct
					}
				}
				channelType := act.Target.Channel
				if channelType == "" {
					channelType = "messenger"
				}
				senderID := primitive.NilObjectID
				if senderSvc, err := notifsvc.NewNotificationSenderService(); err == nil {
					_, sid, _ := findSenderForChannelType(c.Context(), senderSvc, channelType, orgID)
					senderID = sid
				}
				severity := "info"
				priority := notification.GetPriorityFromSeverity(severity)
				maxRetries := notification.GetMaxRetriesFromSeverity(severity)
				item := &deliverymodels.DeliveryQueueItem{
					ID:                  primitive.NewObjectID(),
					EventType:           "execution_send_message",
					OwnerOrganizationID: orgID,
					SenderID:            senderID,
					ChannelType:         channelType,
					Recipient:           recipient,
					Content:             content,
					Payload:             act.Payload,
					Status:              "pending",
					RetryCount:          0,
					MaxRetries:          maxRetries,
					Priority:            priority,
					CreatedAt:           time.Now().Unix(),
					UpdatedAt:           time.Now().Unix(),
				}
				queueItems = append(queueItems, item)
				results = append(results, ExecuteResult{
					ActionID:   act.ActionID,
					ActionType: act.ActionType,
					Status:     "queued",
					QueueID:    item.ID.Hex(),
				})
			} else {
				results = append(results, ExecuteResult{
					ActionID:   act.ActionID,
					ActionType: act.ActionType,
					Status:     "unsupported",
					Message:    fmt.Sprintf("action_type %s chưa hỗ trợ", act.ActionType),
				})
			}
		}

		if len(queueItems) > 0 {
			if err := h.queue.Enqueue(c.Context(), queueItems); err != nil {
				c.Status(common.StatusInternalServerError).JSON(fiber.Map{
					"code": common.ErrCodeBusinessOperation.Code, "message": "Không thể thêm vào queue: " + err.Error(), "status": "error",
				})
				return nil
			}
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã xử lý", "data": fiber.Map{"results": results}, "status": "success",
		})
		return nil
	})
}

// allowDirectDeliveryUse cho phép gọi POST /delivery/execute trực tiếp (deprecated).
// Mặc định false — mọi action phải qua Executor.
func allowDirectDeliveryUse() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("DELIVERY_ALLOW_DIRECT_USE")), "true")
}

// validateDeliveryExecuteActionsForGate Phase 3: validate source=APPROVAL_GATE và actionId.
// Trả (0, "") nếu hợp lệ; (statusCode, message) nếu không.
func validateDeliveryExecuteActionsForGate(actions []deliverydto.ExecutionActionInput) (int, string) {
	for _, act := range actions {
		if act.Source != deliverydto.SourceApprovalGate {
			return 403, "Action phải có source=" + deliverydto.SourceApprovalGate + " khi gọi Delivery qua HTTP. Hiện: " + act.Source
		}
		if act.ActionID == "" {
			return 403, "Action phải có actionId (actionPendingId) khi source=" + deliverydto.SourceApprovalGate
		}
	}
	return 0, ""
}
