// Package crmhdl - Handler profile khách hàng CRM.
package crmhdl

import (
	"errors"
	"fmt"
	"strings"

	basehdl "meta_commerce/internal/api/base/handler"
	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerHandler xử lý API profile khách hàng và CRUD (find, find-one, find-by-id, find-with-pagination, count).
type CrmCustomerHandler struct {
	*basehdl.BaseHandler[crmmodels.CrmCustomer, crmdto.CrmCustomerCreateInput, crmdto.CrmCustomerUpdateInput]
	CustomerService *crmvc.CrmCustomerService
}

// NewCrmCustomerHandler tạo CrmCustomerHandler mới.
func NewCrmCustomerHandler() (*CrmCustomerHandler, error) {
	svc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmCustomerService: %w", err)
	}
	hdl := &CrmCustomerHandler{
		BaseHandler:     basehdl.NewBaseHandler[crmmodels.CrmCustomer, crmdto.CrmCustomerCreateInput, crmdto.CrmCustomerUpdateInput](svc.BaseServiceMongoImpl),
		CustomerService: svc,
	}
	// Filter cho CRUD: cho phép filter theo classification và unifiedId (dashboard, bảng khách).
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$in", "$gt", "$gte", "$lt", "$lte", "$exists", "$regex", "$or"},
		MaxFields:        15,
	})
	return hdl, nil
}

// HandleSyncCustomers xử lý POST /customers/sync — đưa job đồng bộ vào queue, worker sẽ xử lý.
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
		params := bson.M{}
		if len(sources) > 0 {
			params["sources"] = sources
		}
		jobID, err := crmvc.EnqueueCrmBulkJob(c.Context(), crmmodels.CrmBulkJobSync, *orgID, params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi đưa job vào queue: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Job đồng bộ đã được đưa vào queue, worker sẽ xử lý",
			"data": fiber.Map{"jobId": jobID.Hex(), "status": "queued"},
			"status": "success",
		})
		return nil
	})
}

// HandleBackfillActivity xử lý POST /customers/backfill-activity — đưa job backfill vào queue, worker sẽ xử lý.
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
		params := bson.M{}
		if input.Limit > 0 {
			params["limit"] = input.Limit
		}
		if len(types) > 0 {
			params["types"] = types
		}
		jobID, err := crmvc.EnqueueCrmBulkJob(c.Context(), crmmodels.CrmBulkJobBackfill, *orgID, params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi đưa job vào queue: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Job backfill đã được đưa vào queue, worker sẽ xử lý",
			"data": fiber.Map{"jobId": jobID.Hex(), "status": "queued"},
			"status": "success",
		})
		return nil
	})
}

// HandleRebuildCrm xử lý POST /customers/rebuild — đưa job rebuild vào queue, worker sẽ xử lý (sync rồi backfill).
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
		params := bson.M{}
		if input.Limit > 0 {
			params["limit"] = input.Limit
		}
		if len(sources) > 0 {
			params["sources"] = sources
		}
		if len(types) > 0 {
			params["types"] = types
		}
		jobID, err := crmvc.EnqueueCrmBulkJob(c.Context(), crmmodels.CrmBulkJobRebuild, *orgID, params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi đưa job vào queue: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Job rebuild đã được đưa vào queue, worker sẽ xử lý",
			"data": fiber.Map{"jobId": jobID.Hex(), "status": "queued"},
			"status": "success",
		})
		return nil
	})
}

// HandleRecalculateAllCustomers xử lý POST /customers/recalculate-all — đưa job tính toán lại tất cả khách vào queue.
// Body: ownerOrganizationId, limit (0 = tất cả).
func (h *CrmCustomerHandler) HandleRecalculateAllCustomers(c fiber.Ctx) error {
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
		params := bson.M{}
		if input.Limit > 0 {
			params["limit"] = input.Limit
		}
		jobID, err := crmvc.EnqueueCrmBulkJob(c.Context(), crmmodels.CrmBulkJobRecalculateAll, *orgID, params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi đưa job vào queue: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Job tính toán lại tất cả khách đã được đưa vào queue, worker sẽ xử lý",
			"data": fiber.Map{"jobId": jobID.Hex(), "status": "queued"},
			"status": "success",
		})
		return nil
	})
}

// HandleRecalculateCustomer xử lý POST /customers/:unifiedId/recalculate — đưa job tính toán lại 1 khách vào queue.
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
		params := bson.M{"unifiedId": unifiedId}
		jobID, err := crmvc.EnqueueCrmBulkJob(c.Context(), crmmodels.CrmBulkJobRecalculateOne, *orgID, params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi đưa job vào queue: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Job tính toán lại khách hàng đã được đưa vào queue, worker sẽ xử lý",
			"data": fiber.Map{"jobId": jobID.Hex(), "status": "queued"},
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
