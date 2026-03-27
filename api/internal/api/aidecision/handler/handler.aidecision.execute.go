// Package aidecisionhdl — Handler cho AI Decision API.
package aidecisionhdl

import (
	"github.com/gofiber/fiber/v3"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func getActiveOrgID(c fiber.Ctx) *primitive.ObjectID {
	orgIDStr, ok := c.Locals("active_organization_id").(string)
	if !ok || orgIDStr == "" {
		return nil
	}
	oid, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		return nil
	}
	return &oid
}

// HandleExecute POST /ai-decision/execute — enqueue aidecision.execute_requested (chỉ event-driven, worker xử lý).
func HandleExecute(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var req aidecisionsvc.ExecuteRequest
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
		svc := aidecisionsvc.NewAIDecisionService()
		emitRes, err := svc.EmitExecuteRequested(c.Context(), &req, *orgID, orgID.Hex(), "")
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Xếp hàng ra quyết định thất bại")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Đã xếp hàng — worker AI Decision sẽ xử lý", "data": fiber.Map{
				"eventId": emitRes.EventID, "status": emitRes.Status, "traceId": emitRes.TraceID,
			}, "status": "success",
		})
		return nil
	})
}
