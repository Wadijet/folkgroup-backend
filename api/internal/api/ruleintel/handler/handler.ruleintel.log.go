// Package handler — Handler cho Rule Intelligence API.
package handler

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"

	basehdl "meta_commerce/internal/api/base/handler"
	ruleintelsvc "meta_commerce/internal/api/ruleintel/service"
	"meta_commerce/internal/common"
)

// NewGetTraceLogHandler tạo handler cho GET /rule-intelligence/logs/:traceId.
// Trả về rule execution log theo trace_id — dùng cho link "Xem log tạo đề xuất" từ proposal.
func NewGetTraceLogHandler() (fiber.Handler, error) {
	svc, err := ruleintelsvc.NewRuleEngineService()
	if err != nil {
		return nil, fmt.Errorf("tạo RuleEngineService: %w", err)
	}
	return func(c fiber.Ctx) error {
		return handleGetTraceLogWithService(c, svc)
	}, nil
}

func handleGetTraceLogWithService(c fiber.Ctx, svc *ruleintelsvc.RuleEngineService) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		traceID := c.Params("traceId")
		if traceID == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "traceId bắt buộc", "status": "error",
			})
			return nil
		}

		orgID, ok := c.Locals("active_organization_id").(string)
		if !ok || orgID == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}

		trace, err := svc.FindTraceByTraceID(c.Context(), traceID)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy log với traceId này", "status": "error",
				})
				return nil
			}
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Lấy log thất bại")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}

		// Chỉ trả log cho org sở hữu entity
		if trace.EntityRef.OwnerOrganizationID != "" && trace.EntityRef.OwnerOrganizationID != orgID {
			c.Status(common.StatusForbidden).JSON(fiber.Map{
				"code": common.ErrCodeAuth.Code, "message": "Không có quyền xem log này", "status": "error",
			})
			return nil
		}

		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": trace, "status": "success",
		})
		return nil
	})
}
