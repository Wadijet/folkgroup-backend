// Package crmhdl - Handler CRUD cho CrmPendingIngest (queue crm_pending_ingest).
package crmhdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
)

// CrmPendingIngestHandler xử lý CRUD cho crm_pending_ingest (queue Merge/Ingest).
// Chỉ hỗ trợ đọc (find, find-one, find-by-id, find-with-pagination, count) — queue được ghi bởi hook.
type CrmPendingIngestHandler struct {
	*basehdl.BaseHandler[crmmodels.CrmPendingIngest, crmdto.CrmPendingIngestCreateInput, crmdto.CrmPendingIngestUpdateInput]
}

// NewCrmPendingIngestHandler tạo mới CrmPendingIngestHandler.
func NewCrmPendingIngestHandler() (*CrmPendingIngestHandler, error) {
	svc, err := crmvc.NewCrmPendingIngestService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmPendingIngestService: %w", err)
	}
	hdl := &CrmPendingIngestHandler{
		BaseHandler: basehdl.NewBaseHandler[crmmodels.CrmPendingIngest, crmdto.CrmPendingIngestCreateInput, crmdto.CrmPendingIngestUpdateInput](svc),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$ne", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
