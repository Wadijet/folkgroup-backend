// Package aidecisionhdl — Handler CRUD decision_context_policy_overrides.
package aidecisionhdl

import (
	"github.com/gofiber/fiber/v3"

	aidecisiondto "meta_commerce/internal/api/aidecision/dto"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/mongo"
)

// HandleListContextPolicyOverrides GET /ai-decision/context-policy-overrides
func HandleListContextPolicyOverrides(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		svc := aidecisionsvc.NewAIDecisionService()
		list, err := svc.ListContextPolicyOverrides(c.Context(), *orgID)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "data": list, "status": "success",
		})
		return nil
	})
}

// HandleUpsertContextPolicyOverride POST /ai-decision/context-policy-overrides
func HandleUpsertContextPolicyOverride(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var req aidecisiondto.ContextPolicyOverrideUpsertRequest
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
		item, err := svc.UpsertContextPolicyOverride(c.Context(), *orgID, req)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã lưu override context policy", "data": item, "status": "success",
		})
		return nil
	})
}

// HandleDeleteContextPolicyOverride DELETE /ai-decision/context-policy-overrides/:id
func HandleDeleteContextPolicyOverride(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		id := c.Params("id")
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		svc := aidecisionsvc.NewAIDecisionService()
		err := svc.DeleteContextPolicyOverride(c.Context(), *orgID, id)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy bản ghi", "status": "error",
				})
				return nil
			}
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã xóa override context policy", "data": nil, "status": "success",
		})
		return nil
	})
}
