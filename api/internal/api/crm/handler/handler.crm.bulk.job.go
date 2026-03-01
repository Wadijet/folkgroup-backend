// Package crmhdl - Handler CRUD cho CrmBulkJob (queue crm_bulk_jobs).
package crmhdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
)

// CrmBulkJobHandler xử lý CRUD cho crm_bulk_jobs (queue sync, backfill, recalculate).
// Chỉ hỗ trợ đọc (find, find-one, find-by-id, find-with-pagination, count) — queue được ghi bởi sync/backfill handlers.
type CrmBulkJobHandler struct {
	*basehdl.BaseHandler[crmmodels.CrmBulkJob, crmdto.CrmBulkJobCreateInput, crmdto.CrmBulkJobUpdateInput]
}

// NewCrmBulkJobHandler tạo mới CrmBulkJobHandler.
func NewCrmBulkJobHandler() (*CrmBulkJobHandler, error) {
	svc, err := crmvc.NewCrmBulkJobService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmBulkJobService: %w", err)
	}
	hdl := &CrmBulkJobHandler{
		BaseHandler: basehdl.NewBaseHandler[crmmodels.CrmBulkJob, crmdto.CrmBulkJobCreateInput, crmdto.CrmBulkJobUpdateInput](svc),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$ne", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
