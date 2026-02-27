// Package crmhdl - Handler profile khách hàng CRM.
package crmhdl

import (
	"errors"
	"fmt"
	"strings"

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
// Query: sources=pos,fb (rỗng = tất cả). Body: ownerOrganizationId.
func (h *CrmCustomerHandler) HandleSyncCustomers(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input struct {
			OwnerOrganizationId string `json:"ownerOrganizationId"`
		}
		_ = c.Bind().Body(&input)
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
		sources := splitAndTrim(c.Query("sources"), ",")
		posCount, fbCount, err := h.CustomerService.SyncAllCustomers(c.Context(), *orgID, sources)
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
// Query: types=order,conversation,note (rỗng = tất cả). Body: ownerOrganizationId, limit.
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
		types := splitAndTrim(c.Query("types"), ",")
		result, err := h.CustomerService.BackfillActivity(c.Context(), *orgID, input.Limit, types)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi backfill: " + err.Error(), "status": "error",
			})
			return nil
		}
		msg := "Backfill hoàn tất"
		if result.ConversationsSkippedNoResolve > 0 && result.ConversationsSkippedNoResolve == result.ConversationsProcessed {
			msg = "Backfill hoàn tất. Lưu ý: tất cả conversations không resolve được — chạy trước: go run scripts/backfill_fb_customers_from_conversations.go " + orgID.Hex()
		} else if result.OrdersProcessed == 0 && result.ConversationsProcessed == 0 && result.NotesProcessed == 0 {
			msg = "Backfill hoàn tất. Không có dữ liệu để xử lý — kiểm tra: 1) ownerOrganizationId đúng; 2) fb_conversations/pc_pos_orders có ownerOrganizationId (chạy scripts/backfill_conversation_ownerorg.go nếu thiếu); 3) fb_customers đã có (chạy scripts/backfill_fb_customers_from_conversations.go)"
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": msg,
			"data": result,
			"status": "success",
		})
		return nil
	})
}

// HandleRebuildCrm xử lý POST /customers/rebuild — sync rồi backfill.
// Query: sources=pos,fb (rỗng=tất cả), types=order,conversation,note (rỗng=tất cả).
// Body: ownerOrganizationId, limit.
func (h *CrmCustomerHandler) HandleRebuildCrm(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input struct {
			OwnerOrganizationId string `json:"ownerOrganizationId"`
			Limit               int    `json:"limit"`
		}
		_ = c.Bind().Body(&input)
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
		sources := splitAndTrim(c.Query("sources"), ",")
		types := splitAndTrim(c.Query("types"), ",")
		result, err := h.CustomerService.RebuildCrm(c.Context(), *orgID, input.Limit, sources, types)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi rebuild CRM: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Rebuild hoàn tất",
			"data": result,
			"status": "success",
		})
		return nil
	})
}

// HandleRecalculateCustomer xử lý POST /customers/:unifiedId/recalculate — cập nhật toàn bộ thông tin khách từ tất cả nguồn.
func (h *CrmCustomerHandler) HandleRecalculateCustomer(c fiber.Ctx) error {
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
		result, err := h.CustomerService.RecalculateCustomerFromAllSources(c.Context(), unifiedId, *orgID)
		if err != nil {
			if errors.Is(err, common.ErrNotFound) {
				c.Status(common.StatusNotFound).JSON(fiber.Map{
					"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy khách hàng", "status": "error",
				})
				return nil
			}
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi cập nhật khách hàng: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Đã cập nhật toàn bộ thông tin khách hàng",
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
		opts := &crmvc.GetFullProfileOpts{
			ClientIp: c.IP(),
			UserAgent: c.Get("User-Agent"),
		}
		if domains := c.Query("domain"); domains != "" {
			opts.Domains = splitAndTrim(domains, ",")
		}
		profile, err := h.CustomerService.GetFullProfile(c.Context(), unifiedId, *orgID, opts)
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

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
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
