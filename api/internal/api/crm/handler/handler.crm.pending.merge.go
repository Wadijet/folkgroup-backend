// Package crmhdl — Handler CRUD cho CrmPendingMerge (queue crm_pending_merge).
package crmhdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	crmdto "meta_commerce/internal/api/crm/dto"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
)

// CrmPendingMergeHandler queue merge L1→L2 (chủ yếu đọc).
type CrmPendingMergeHandler struct {
	*basehdl.BaseHandler[crmmodels.CrmPendingMerge, crmdto.CrmPendingMergeCreateInput, crmdto.CrmPendingMergeUpdateInput]
}

// NewCrmPendingMergeHandler tạo handler.
func NewCrmPendingMergeHandler() (*CrmPendingMergeHandler, error) {
	svc, err := crmvc.NewCrmPendingMergeService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmPendingMergeService: %w", err)
	}
	hdl := &CrmPendingMergeHandler{
		BaseHandler: basehdl.NewBaseHandler[crmmodels.CrmPendingMerge, crmdto.CrmPendingMergeCreateInput, crmdto.CrmPendingMergeUpdateInput](svc),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$ne", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
