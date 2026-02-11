// Package reportsvc - Service CRUD cho Report Snapshot (report_snapshots).
package reportsvc

import (
	"fmt"

	reportmodels "meta_commerce/internal/api/report/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// ReportSnapshotService service CRUD cho bảng report_snapshots.
type ReportSnapshotService struct {
	*basesvc.BaseServiceMongoImpl[reportmodels.ReportSnapshot]
}

// NewReportSnapshotService tạo mới ReportSnapshotService.
func NewReportSnapshotService() (*ReportSnapshotService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ReportSnapshots)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.ReportSnapshots, common.ErrNotFound)
	}
	return &ReportSnapshotService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[reportmodels.ReportSnapshot](coll),
	}, nil
}
