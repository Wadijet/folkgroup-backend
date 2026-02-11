// Package reportsvc - Service CRUD cho Report Definition (report_definitions).
package reportsvc

import (
	"fmt"

	reportmodels "meta_commerce/internal/api/report/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// ReportDefinitionService service CRUD cho bảng report_definitions.
type ReportDefinitionService struct {
	*basesvc.BaseServiceMongoImpl[reportmodels.ReportDefinition]
}

// NewReportDefinitionService tạo mới ReportDefinitionService.
func NewReportDefinitionService() (*ReportDefinitionService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ReportDefinitions)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.ReportDefinitions, common.ErrNotFound)
	}
	return &ReportDefinitionService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[reportmodels.ReportDefinition](coll),
	}, nil
}
