// Package handler — Handler cho Rule Intelligence API.
package handler

import (
	"github.com/gofiber/fiber/v3"

	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/api/ruleintel/dto"
	"meta_commerce/internal/api/ruleintel/models"
	ruleintelsvc "meta_commerce/internal/api/ruleintel/service"
	"meta_commerce/internal/common"
)

var ruleEngineSvc *ruleintelsvc.RuleEngineService

func init() {
	var err error
	ruleEngineSvc, err = ruleintelsvc.NewRuleEngineService()
	if err != nil {
		panic("RuleEngineService: " + err.Error())
	}
}

func getActiveOrgID(c fiber.Ctx) string {
	orgIDStr, ok := c.Locals("active_organization_id").(string)
	if !ok || orgIDStr == "" {
		return ""
	}
	return orgIDStr
}

// HandleRunRule POST /rule-intelligence/run — Chạy rule với context.
func HandleRunRule(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var req dto.RunRuleRequest
		if err := c.Bind().JSON(&req); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Body JSON không hợp lệ", "status": "error",
			})
			return nil
		}

		if req.RuleID == "" || req.Domain == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "rule_id và domain bắt buộc", "status": "error",
			})
			return nil
		}

		orgID := getActiveOrgID(c)
		if orgID == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}

		// Đảm bảo entity_ref có ownerOrganizationId
		if req.EntityRef.OwnerOrganizationID == "" {
			req.EntityRef.OwnerOrganizationID = orgID
		}

		input := &ruleintelsvc.RunInput{
			RuleID:         req.RuleID,
			Domain:         req.Domain,
			EntityRef:      models.EntityRef{
				Domain:              req.EntityRef.Domain,
				ObjectType:          req.EntityRef.ObjectType,
				ObjectID:            req.EntityRef.ObjectID,
				OwnerOrganizationID: req.EntityRef.OwnerOrganizationID,
			},
			Layers:         req.Layers,
			ParamsOverride: req.ParamsOverride,
		}

		if input.Layers == nil {
			input.Layers = map[string]interface{}{}
		}

		result, err := ruleEngineSvc.Run(c.Context(), input)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Chạy rule thất bại")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}
