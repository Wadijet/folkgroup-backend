// Package crmhdl - Handler profile khách hàng CRM.
package crmhdl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	basehdl "meta_commerce/internal/api/base/handler"
	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
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
	CustomerService   *crmvc.CrmCustomerService
	BulkJobService    *crmvc.CrmBulkJobService
	IntelRunService   *crmvc.CrmCustomerIntelRunService
}

// NewCrmCustomerHandler tạo CrmCustomerHandler mới.
func NewCrmCustomerHandler() (*CrmCustomerHandler, error) {
	svc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmCustomerService: %w", err)
	}
	bulkJobSvc, err := crmvc.NewCrmBulkJobService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmBulkJobService: %w", err)
	}
	intelRunSvc, err := crmvc.NewCrmCustomerIntelRunService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmCustomerIntelRunService: %w", err)
	}
	hdl := &CrmCustomerHandler{
		BaseHandler:     basehdl.NewBaseHandler[crmmodels.CrmCustomer, crmdto.CrmCustomerCreateInput, crmdto.CrmCustomerUpdateInput](svc.BaseServiceMongoImpl),
		CustomerService: svc,
		BulkJobService:  bulkJobSvc,
		IntelRunService: intelRunSvc,
	}
	// Filter cho CRUD: cho phép filter theo classification và unifiedId (dashboard, bảng khách).
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$in", "$gt", "$gte", "$lt", "$lte", "$exists", "$regex", "$or"},
		MaxFields:        15,
	})
	return hdl, nil
}

// HandleRebuildCrm xử lý POST /customers/rebuild — tạo 2 job riêng: sync rồi backfill (đúng logic, dễ checkpoint).
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
			IsPriority          bool   `json:"isPriority"` // Job ưu tiên: bắt buộc chạy ngay, không bị throttle
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
		limit := input.Limit
		jobIDs := make([]string, 0, 2)

		// Job 1: Sync (chạy trước) — tạo nếu cần sync
		needSync := len(syncSources) > 0 || (len(sources) == 0)
		if needSync {
			syncParams := bson.M{}
			if len(syncSources) > 0 {
				syncParams["sources"] = syncSources
			}
			syncJobID, err := h.BulkJobService.Enqueue(c.Context(), crmmodels.CrmBulkJobSync, *orgID, syncParams, input.IsPriority)
			if err != nil {
				c.Status(common.StatusInternalServerError).JSON(fiber.Map{
					"code": common.ErrCodeDatabase.Code, "message": "Lỗi đưa job sync vào queue: " + err.Error(), "status": "error",
				})
				return nil
			}
			jobIDs = append(jobIDs, syncJobID.Hex())
		}

		// Job 2: Backfill (chạy sau sync) — tạo nếu cần backfill
		needBackfill := len(backfillTypes) > 0 || (len(sources) == 0)
		if needSync && needBackfill {
			time.Sleep(1 * time.Second) // Đảm bảo backfill có createdAt sau sync → chạy đúng thứ tự
		}
		if needBackfill {
			backfillParams := bson.M{}
			if limit > 0 {
				backfillParams["limit"] = limit
			}
			if len(backfillTypes) > 0 {
				backfillParams["types"] = backfillTypes
			}
			backfillJobID, err := h.BulkJobService.Enqueue(c.Context(), crmmodels.CrmBulkJobBackfill, *orgID, backfillParams, input.IsPriority)
			if err != nil {
				c.Status(common.StatusInternalServerError).JSON(fiber.Map{
					"code": common.ErrCodeDatabase.Code, "message": "Lỗi đưa job backfill vào queue: " + err.Error(), "status": "error",
				})
				return nil
			}
			jobIDs = append(jobIDs, backfillJobID.Hex())
		}

		msg := "Các job rebuild (sync + backfill) đã được đưa vào queue"
		if len(jobIDs) == 1 {
			msg = "Job đã được đưa vào queue"
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": msg,
			"data": fiber.Map{"jobIds": jobIDs, "status": "queued"},
			"status": "success",
		})
		return nil
	})
}

// HandleRecalculateAllCustomers xử lý POST /customers/recalculate-all — một event AI Decision recalculate toàn bộ khách theo org (limit=0).
// Body: ownerOrganizationId, batchSize (tùy chọn — dùng làm poolSize worker; mặc định 12).
func (h *CrmCustomerHandler) HandleRecalculateAllCustomers(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		var input struct {
			OwnerOrganizationId string `json:"ownerOrganizationId"`
			BatchSize           int    `json:"batchSize"`   // Số khách mỗi batch (mặc định 200)
			IsPriority          bool   `json:"isPriority"`  // Job ưu tiên: bắt buộc chạy ngay, không bị throttle
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
		poolSize := input.BatchSize
		if poolSize <= 0 {
			poolSize = 12
		}
		eventID, err := crmqueue.EmitCrmIntelligenceRecalculateAllRequested(c.Context(), *orgID, 0, poolSize)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi ghi event AI Decision: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Đã đưa tính toán lại toàn bộ khách (theo org) vào queue AI Decision",
			"data": fiber.Map{"eventId": eventID, "status": "queued_ai_decision"},
			"status": "success",
		})
		return nil
	})
}

// HandleRecalculateCustomer xử lý POST /customers/:unifiedId/recalculate — ghi event recalculate một khách vào queue AI Decision.
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
		_ = c.Bind().Body(&struct {
			IsPriority bool `json:"isPriority"`
		}{})
		eventID, err := crmqueue.EmitCrmIntelligenceRecalculateOneRequested(c.Context(), unifiedId, *orgID)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi ghi event AI Decision: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusAccepted).JSON(fiber.Map{
			"code": common.StatusAccepted, "message": "Đã đưa tính toán lại khách hàng vào queue AI Decision",
			"data": fiber.Map{"eventId": eventID, "status": "queued_ai_decision"},
			"status": "success",
		})
		return nil
	})
}

// HandleListIntelRuns xử lý GET /customers/:unifiedId/intel-runs — phân trang lịch sử intel (crm_customer_intel_runs).
// Query: page (mặc 1), limit (mặc 20, tối đa 100), newestFirst (mặc true — mới theo causal/sequence lên trước).
func (h *CrmCustomerHandler) HandleListIntelRuns(c fiber.Ctx) error {
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
		page, _ := strconv.ParseInt(c.Query("page", "1"), 10, 64)
		if page < 1 {
			page = 1
		}
		limit, _ := strconv.ParseInt(c.Query("limit", "20"), 10, 64)
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		newestFirst := true
		switch strings.ToLower(strings.TrimSpace(c.Query("newestFirst", "true"))) {
		case "false", "0", "no":
			newestFirst = false
		}

		res, err := h.IntelRunService.ListIntelRunsByUnifiedID(c.Context(), *orgID, unifiedId, page, limit, newestFirst)
		if err != nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": err.Error(), "status": "error",
			})
			return nil
		}
		items := make([]crmdto.CrmCustomerIntelRunListItem, 0, len(res.Items))
		for _, r := range res.Items {
			items = append(items, mapCrmIntelRunToListItem(r))
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code":    common.StatusOK,
			"message": "Thành công",
			"data": fiber.Map{
				"page":       res.Page,
				"limit":      res.Limit,
				"itemCount":  res.ItemCount,
				"total":      res.Total,
				"totalPage":  res.TotalPage,
				"items":      items,
				"unifiedId":  unifiedId,
				"newestFirst": newestFirst,
			},
			"status": "success",
		})
		return nil
	})
}

func mapCrmIntelRunToListItem(r crmmodels.CrmCustomerIntelRun) crmdto.CrmCustomerIntelRunListItem {
	item := crmdto.CrmCustomerIntelRunListItem{
		Id:                    r.ID.Hex(),
		Operation:             r.Operation,
		Status:                r.Status,
		ComputedAt:            r.ComputedAt,
		CausalOrderingAt:      r.CausalOrderingAt,
		IntelSequence:         r.IntelSequence,
		ErrorMessage:          r.ErrorMessage,
		ParentDecisionEventId: r.ParentDecisionEventID,
	}
	if !r.ParentIntelJobID.IsZero() {
		item.ParentIntelJobId = r.ParentIntelJobID.Hex()
	}
	if len(r.MetricsSummary) > 0 {
		item.MetricsSummary = map[string]interface{}(r.MetricsSummary)
	}
	return item
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
