// Package aidecisionhdl — Handler đóng case thủ công (closed_manual).
package aidecisionhdl

import (
	"github.com/gofiber/fiber/v3"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
)

// HandleCloseCase POST /ai-decision/cases/:decisionCaseId/close — đóng case thủ công (closed_manual).
func HandleCloseCase(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		decisionCaseID := c.Params("decisionCaseId")
		if decisionCaseID == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "decisionCaseId bắt buộc", "status": "error",
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
		err := svc.CloseCaseWithOrgCheck(c.Context(), decisionCaseID, aidecisionmodels.ClosureManual, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Đóng case thất bại")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã đóng case", "data": fiber.Map{"decisionCaseId": decisionCaseID, "closureType": aidecisionmodels.ClosureManual}, "status": "success",
		})
		return nil
	})
}
