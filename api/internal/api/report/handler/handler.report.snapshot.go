// Package reporthdl - Handler CRUD cho Report Snapshot.
package reporthdl

import (
	"fmt"

	basehdl "meta_commerce/internal/api/base/handler"
	reportdto "meta_commerce/internal/api/report/dto"
	reportmodels "meta_commerce/internal/api/report/models"
	reportsvc "meta_commerce/internal/api/report/service"
)

// ReportSnapshotHandler xử lý CRUD cho report snapshot (report_snapshots).
type ReportSnapshotHandler struct {
	*basehdl.BaseHandler[reportmodels.ReportSnapshot, reportdto.ReportSnapshotCreateInput, reportdto.ReportSnapshotUpdateInput]
}

// NewReportSnapshotHandler tạo mới ReportSnapshotHandler.
func NewReportSnapshotHandler() (*ReportSnapshotHandler, error) {
	svc, err := reportsvc.NewReportSnapshotService()
	if err != nil {
		return nil, fmt.Errorf("tạo ReportSnapshotService: %w", err)
	}
	hdl := &ReportSnapshotHandler{
		BaseHandler: basehdl.NewBaseHandler[reportmodels.ReportSnapshot, reportdto.ReportSnapshotCreateInput, reportdto.ReportSnapshotUpdateInput](svc),
	}
	hdl.SetFilterOptions(basehdl.FilterOptions{
		DeniedFields:     []string{},
		AllowedOperators: []string{"$eq", "$gt", "$gte", "$lt", "$lte", "$in", "$nin", "$exists", "$regex"},
		MaxFields:        10,
	})
	return hdl, nil
}
