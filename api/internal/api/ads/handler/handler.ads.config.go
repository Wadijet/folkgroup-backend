// Package adshdl — Handler cấu hình approval (approvalConfig).
package adshdl

import (
	"github.com/gofiber/fiber/v3"

	adssvc "meta_commerce/internal/api/ads/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"
)

// HandleGetApprovalConfig lấy approvalConfig của ad account.
// GET /ads/config/approval?adAccountId=act_xxx
func HandleGetApprovalConfig(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		adAccountId := c.Query("adAccountId")
		if adAccountId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "adAccountId không được để trống", "status": "error",
			})
			return nil
		}
		config, err := adssvc.GetApprovalConfig(c.Context(), adAccountId, *orgID)
		if err != nil {
			c.Status(common.StatusNotFound).JSON(fiber.Map{
				"code": common.ErrCodeDatabaseQuery.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": config, "status": "success",
		})
		return nil
	})
}

// HandleUpdateApprovalConfig cập nhật approvalConfig.
// PUT /ads/config/approval
func HandleUpdateApprovalConfig(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var body struct {
			AdAccountId   string                 `json:"adAccountId"`
			ApprovalConfig map[string]interface{} `json:"approvalConfig"`
		}
		if err := c.Bind().JSON(&body); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		if body.AdAccountId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "adAccountId không được để trống", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		err := adssvc.UpdateApprovalConfig(c.Context(), body.AdAccountId, *orgID, body.ApprovalConfig)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeInternalServer.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã cập nhật approvalConfig", "status": "success",
		})
		return nil
	})
}
