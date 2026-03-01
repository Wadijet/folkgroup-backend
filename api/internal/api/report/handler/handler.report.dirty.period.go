// Package reporthdl - Handler CRUD cho Report Dirty Period.
package reporthdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	reportdto "meta_commerce/internal/api/report/dto"
	reportmodels "meta_commerce/internal/api/report/models"
	reportsvc "meta_commerce/internal/api/report/service"
)

// ReportDirtyPeriodHandler xử lý CRUD cho report dirty period (report_dirty_periods).
type ReportDirtyPeriodHandler struct {
	*basehdl.BaseHandler[reportmodels.ReportDirtyPeriod, reportdto.ReportDirtyPeriodCreateInput, reportdto.ReportDirtyPeriodUpdateInput]
}

// NewReportDirtyPeriodHandler tạo mới ReportDirtyPeriodHandler.
func NewReportDirtyPeriodHandler() (*ReportDirtyPeriodHandler, error) {
	svc, err := reportsvc.NewReportDirtyPeriodService()
	if err != nil {
		return nil, fmt.Errorf("tạo ReportDirtyPeriodService: %w", err)
	}
	hdl := &ReportDirtyPeriodHandler{
		BaseHandler: basehdl.NewBaseHandler[reportmodels.ReportDirtyPeriod, reportdto.ReportDirtyPeriodCreateInput, reportdto.ReportDirtyPeriodUpdateInput](svc),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$ne", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists"},
		MaxFields:        10,
	})
	return hdl, nil
}
