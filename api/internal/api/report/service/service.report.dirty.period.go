// Package reportsvc - Service CRUD cho Report Dirty Period (report_dirty_periods).
package reportsvc

import (
	"fmt"

	reportmodels "meta_commerce/internal/api/report/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// ReportDirtyPeriodService service CRUD cho bảng report_dirty_periods.
type ReportDirtyPeriodService struct {
	*basesvc.BaseServiceMongoImpl[reportmodels.ReportDirtyPeriod]
}

// NewReportDirtyPeriodService tạo mới ReportDirtyPeriodService.
func NewReportDirtyPeriodService() (*ReportDirtyPeriodService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ReportDirtyPeriods)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.ReportDirtyPeriods, common.ErrNotFound)
	}
	return &ReportDirtyPeriodService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[reportmodels.ReportDirtyPeriod](coll),
	}, nil
}
