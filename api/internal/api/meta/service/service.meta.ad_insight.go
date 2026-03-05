// Package metasvc - Service quản lý meta ad insights.
package metasvc

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	metamodels "meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"
)

// MetaAdInsightService service quản lý meta ad insights.
// Kế thừa BaseServiceMongoImpl, dùng Upsert từ base (filter objectId + dateStart + objectType).
type MetaAdInsightService struct {
	*basesvc.BaseServiceMongoImpl[metamodels.MetaAdInsight]
}

// NewMetaAdInsightService tạo MetaAdInsightService.
func NewMetaAdInsightService() (*MetaAdInsightService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ad_insights")
	}
	return &MetaAdInsightService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[metamodels.MetaAdInsight](coll),
	}, nil
}
