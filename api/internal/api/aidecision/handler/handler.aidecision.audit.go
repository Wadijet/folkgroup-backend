// Package aidecisionhdl — API đọc list case / queue phục vụ màn trace & audit.
package aidecisionhdl

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	basehdl "meta_commerce/internal/api/base/handler"
	"meta_commerce/internal/common"

)

// HandleListDecisionCases GET /ai-decision/cases — danh sách decision_cases_runtime theo org (phân trang).
// Query: page, limit (mặc định 20, tối đa 100), status?, caseType?, traceId?, fromUpdatedMs?, toUpdatedMs? (Unix ms).
func HandleListDecisionCases(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		page := queryPositiveInt(c, "page", 1)
		limit := queryPositiveInt(c, "limit", aidecisionsvc.AuditDefaultListLimit())
		f := aidecisionsvc.ListDecisionCasesFilter{
			OwnerOrganizationID: *orgID,
			Page:                page,
			Limit:               limit,
			Status:              c.Query("status"),
			CaseType:            c.Query("caseType"),
			TraceID:             c.Query("traceId"),
		}
		if v, ok := queryInt64Ptr(c, "fromUpdatedMs"); ok {
			f.FromUpdatedMs = v
		}
		if v, ok := queryInt64Ptr(c, "toUpdatedMs"); ok {
			f.ToUpdatedMs = v
		}
		svc := aidecisionsvc.NewAIDecisionService()
		items, total, err := svc.ListDecisionCases(c.Context(), f)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Không đọc được danh sách case")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "status": "success",
			"data": fiber.Map{
				"items": items,
				"pagination": fiber.Map{
					"page": page, "limit": limit, "total": total,
					"totalPages": paginationTotalPages(total, limit),
				},
			},
		})
		return nil
	})
}

// HandleGetDecisionCase GET /ai-decision/cases/:decisionCaseId — chi tiết một case (cùng org).
func HandleGetDecisionCase(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		decisionCaseID := strings.TrimSpace(c.Params("decisionCaseId"))
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
		doc, err := svc.FindCaseByDecisionCaseID(c.Context(), decisionCaseID, *orgID)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Không đọc được case")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}
		if doc == nil {
			c.Status(common.StatusNotFound).JSON(fiber.Map{
				"code": common.ErrCodeDatabaseQuery.Code, "message": "Không tìm thấy case trong tổ chức hiện tại", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "status": "success",
			"data": doc,
		})
		return nil
	})
}

// HandleListQueueEvents GET /ai-decision/queue-events — danh sách decision_events_queue theo org.
// Query: page, limit, status?, eventType?, traceId?, fromCreatedMs?, toCreatedMs? (Unix ms), includePayload (true/false, mặc định false).
func HandleListQueueEvents(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrgID(c)
		if orgID == nil {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Chưa chọn tổ chức", "status": "error",
			})
			return nil
		}
		page := queryPositiveInt(c, "page", 1)
		limit := queryPositiveInt(c, "limit", aidecisionsvc.AuditDefaultListLimit())
		includePayload := strings.EqualFold(strings.TrimSpace(c.Query("includePayload")), "true") ||
			c.Query("includePayload") == "1"
		f := aidecisionsvc.ListQueueEventsFilter{
			OwnerOrganizationID: *orgID,
			Page:                page,
			Limit:               limit,
			Status:              c.Query("status"),
			EventType:           c.Query("eventType"),
			TraceID:             c.Query("traceId"),
			IncludePayload:      includePayload,
		}
		if v, ok := queryInt64Ptr(c, "fromCreatedMs"); ok {
			f.FromCreatedMs = v
		}
		if v, ok := queryInt64Ptr(c, "toCreatedMs"); ok {
			f.ToCreatedMs = v
		}
		svc := aidecisionsvc.NewAIDecisionService()
		items, total, err := svc.ListQueueEvents(c.Context(), f)
		if err != nil {
			errCode, msg, statusCode := common.GetErrorResponseInfo(err, "Không đọc được danh sách queue")
			c.Status(statusCode).JSON(fiber.Map{"code": errCode, "message": msg, "status": "error"})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "OK", "status": "success",
			"data": fiber.Map{
				"items": items,
				"pagination": fiber.Map{
					"page": page, "limit": limit, "total": total,
					"totalPages": paginationTotalPages(total, limit),
				},
				"includePayload": includePayload,
			},
		})
		return nil
	})
}

func queryPositiveInt(c fiber.Ctx, key string, def int) int {
	s := strings.TrimSpace(c.Query(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func queryInt64Ptr(c fiber.Ctx, key string) (*int64, bool) {
	s := strings.TrimSpace(c.Query(key))
	if s == "" {
		return nil, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, false
	}
	return &v, true
}

func paginationTotalPages(total int64, limit int) int64 {
	if limit <= 0 {
		return 0
	}
	pages := total / int64(limit)
	if total%int64(limit) != 0 {
		pages++
	}
	return pages
}
