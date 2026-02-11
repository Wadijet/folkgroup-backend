// Package reporthdl - Handler CRUD cho Report Definition.
package reporthdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	reportdto "meta_commerce/internal/api/report/dto"
	reportmodels "meta_commerce/internal/api/report/models"
	reportsvc "meta_commerce/internal/api/report/service"
)

// ReportDefinitionHandler xử lý CRUD cho report definition (report_definitions).
type ReportDefinitionHandler struct {
	*basehdl.BaseHandler[reportmodels.ReportDefinition, reportdto.ReportDefinitionCreateInput, reportdto.ReportDefinitionUpdateInput]
}

// NewReportDefinitionHandler tạo mới ReportDefinitionHandler.
func NewReportDefinitionHandler() (*ReportDefinitionHandler, error) {
	svc, err := reportsvc.NewReportDefinitionService()
	if err != nil {
		return nil, fmt.Errorf("tạo ReportDefinitionService: %w", err)
	}
	hdl := &ReportDefinitionHandler{
		BaseHandler: basehdl.NewBaseHandler[reportmodels.ReportDefinition, reportdto.ReportDefinitionCreateInput, reportdto.ReportDefinitionUpdateInput](svc),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists", "$regex"},
		MaxFields:        10,
	})
	return hdl, nil
}
