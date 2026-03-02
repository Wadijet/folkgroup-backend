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

// HandleRebuildCrm xử lý POST /customers/rebuild — API hợp nhất sync + backfill, đưa job vào queue.
// Query hoặc Body: sources=pos,fb,order,conversation,note (rỗng = tất cả nguồn).
//   - pos, fb: đồng bộ profile từ pc_pos_customers, fb_customers
//   - order, conversation, note: backfill activity từ orders, conversations, notes
// Body: ownerOrganizationId, limit (0 = không giới hạn).
func (h *CrmCustomerHandler) HandleRebuildCrm(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input struct {
			OwnerOrganizationId string `json:"ownerOrganizationId"`
			Limit               int    `json:"limit"`
			Sources             string `json:"sources"`
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
		// sources: pos,fb,order,conversation,note — rỗng = tất cả. Ưu tiên query, fallback body.
		sourcesRaw := c.Query("sources")
		if sourcesRaw == "" {
			sourcesRaw = input.Sources
		}
		sources := splitAndTrim(sourcesRaw, ",")
		syncSources, backfillTypes := splitSourcesIntoSyncAndBackfill(sources)
		params := bson.M{}
		if input.Limit > 0 {
			params["limit"] = input.Limit
		}
		// Khi user chỉ định sources: truyền cả syncSources và backfillTypes. [] = bỏ qua phần đó.
		if len(sources) > 0 {
			params["sources"] = syncSources
			params["types"] = backfillTypes
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

// splitSourcesIntoSyncAndBackfill tách sources thành sync (pos,fb) và backfill (order,conversation,note).
// Rỗng = tất cả. Giá trị không hợp lệ bị bỏ qua.
func splitSourcesIntoSyncAndBackfill(sources []string) (syncSources, backfillTypes []string) {
	if len(sources) == 0 {
		return nil, nil // nil = tất cả (worker sẽ chạy full)
	}
	for _, v := range sources {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "pos":
			syncSources = append(syncSources, "pos")
		case "fb":
			syncSources = append(syncSources, "fb")
		case "order":
			backfillTypes = append(backfillTypes, "order")
		case "conversation":
			backfillTypes = append(backfillTypes, "conversation")
		case "note":
			backfillTypes = append(backfillTypes, "note")
		}
	}
	return syncSources, backfillTypes
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
