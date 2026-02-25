// Package crmhdl - Handler profile khách hàng CRM.
package crmhdl

import (
	"errors"
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerHandler xử lý API profile khách hàng.
type CrmCustomerHandler struct {
	CustomerService *crmvc.CrmCustomerService
}

// NewCrmCustomerHandler tạo CrmCustomerHandler mới.
func NewCrmCustomerHandler() (*CrmCustomerHandler, error) {
	svc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmCustomerService: %w", err)
	}
	return &CrmCustomerHandler{CustomerService: svc}, nil
}

// HandleSyncCustomers xử lý POST /customers/sync — đồng bộ crm_customers từ POS và FB.
func (h *CrmCustomerHandler) HandleSyncCustomers(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức", "status": "error",
			})
			return nil
		}
		posCount, fbCount, err := h.CustomerService.SyncAllCustomers(c.Context(), *orgID)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi đồng bộ CRM: "+err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đồng bộ thành công",
			"data": fiber.Map{"posProcessed": posCount, "fbProcessed": fbCount},
			"status": "success",
		})
		return nil
	})
}

// HandleBackfillActivity xử lý POST /customers/backfill-activity — job bên ngoài gọi để backfill activity từ dữ liệu cũ.
func (h *CrmCustomerHandler) HandleBackfillActivity(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input struct {
			OwnerOrganizationId string `json:"ownerOrganizationId"`
			Limit               int    `json:"limit"`
		}
		if err := c.Bind().Body(&input); err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationFormat.Code, "message": "Dữ liệu gửi lên không đúng định dạng JSON", "status": "error",
			})
			return nil
		}
		// Ưu tiên ownerOrganizationId từ body, fallback từ context
		orgID := getActiveOrganizationID(c)
		if input.OwnerOrganizationId != "" {
			parsed, err := primitive.ObjectIDFromHex(input.OwnerOrganizationId)
			if err != nil {
				c.Status(common.StatusBadRequest).JSON(fiber.Map{
					"code": common.ErrCodeValidationInput.Code, "message": "ownerOrganizationId không hợp lệ", "status": "error",
				})
				return nil
			}
			orgID = &parsed
		}
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng cung cấp ownerOrganizationId hoặc chọn tổ chức", "status": "error",
			})
			return nil
		}
		result, err := h.CustomerService.BackfillActivity(c.Context(), *orgID, input.Limit)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi backfill: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Backfill hoàn tất",
			"data": result,
			"status": "success",
		})
		return nil
	})
}

// HandleGetProfile xử lý GET /customers/:unifiedId/profile.
func (h *CrmCustomerHandler) HandleGetProfile(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		unifiedId := c.Params("unifiedId")
		if unifiedId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu unifiedId", "status": "error",
			})
			return nil
		}
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		profile, err := h.CustomerService.GetFullProfile(c.Context(), unifiedId, *orgID)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy khách hàng", "status": "error",
				})
				return nil
			}
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn profile khách hàng", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": profile, "status": "success",
		})
		return nil
	})
}

// getActiveOrganizationID lấy active organization ID từ context.
func getActiveOrganizationID(c fiber.Ctx) *primitive.ObjectID {
	orgIDStr, ok := c.Locals("active_organization_id").(string)
	if !ok || orgIDStr == "" {
		return nil
	}
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		return nil
	}
	return &orgID
}
