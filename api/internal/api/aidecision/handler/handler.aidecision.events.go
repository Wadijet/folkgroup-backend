// Package aidecisionhdl — Handler cho AI Decision events API.
package aidecisionhdl

import (
	"github.com/gofiber/fiber/v3"

	aidecisiondto "meta_commerce/internal/api/aidecision/dto"
	"meta_commerce/internal/api/aidecision/eventopstier"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
)

// HandleIngestEvent POST /ai-decision/events — Nhận event, ghi vào decision_events_queue.
func HandleIngestEvent(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var req aidecisiondto.IngestEventRequest
		if err := c.Bind().JSON(&req); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Body JSON không hợp lệ", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}

		// Priority, lane mặc định
		priority := req.Priority
		if priority == "" {
			priority = "normal"
		}
		lane := req.Lane
		if lane == "" {
			lane = aidecisionsvc.DefaultLaneForEventType(req.EventType)
		}

		svc := aidecisionsvc.NewAIDecisionService()
		resp, err := svc.EmitEvent(c.Context(), &aidecisionsvc.EmitEventInput{
			EventType:     req.EventType,
			EventSource:   req.EventSource,
			EntityType:    req.EntityType,
			EntityID:      req.EntityID,
			OrgID:         req.OrgID,
			OwnerOrgID:    *orgID,
			Priority:      priority,
			Lane:          lane,
			TraceID:       req.TraceID,
			CorrelationID: req.CorrelationID,
			Payload:       req.Payload,
		})
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Ghi event thất bại")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}

		tier, tierLbl := eventopstier.ClassifyEventType(req.EventType)
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã nhận event", "data": aidecisiondto.IngestEventResponse{
				EventID:        resp.EventID,
				Status:         resp.Status,
				W3CTraceID:     resp.W3CTraceID,
				OpsTier:        tier,
				OpsTierLabelVi: tierLbl,
			}, "status": "success",
		})
		return nil
	})
}
