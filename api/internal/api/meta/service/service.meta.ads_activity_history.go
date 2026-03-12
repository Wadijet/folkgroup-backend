// Package metasvc - Service quản lý lịch sử hoạt động Meta Ads (ads_activity_history).
package metasvc

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	metamodels "meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"
)

// MetaAdsActivityHistoryService service quản lý lịch sử thay đổi metrics (Campaign/AdSet/Ad).
// Dữ liệu được ghi tự động bởi hệ thống khi currentMetrics thay đổi; API chỉ hỗ trợ đọc.
type MetaAdsActivityHistoryService struct {
	*basesvc.BaseServiceMongoImpl[metamodels.AdsActivityHistory]
}

// NewMetaAdsActivityHistoryService tạo MetaAdsActivityHistoryService.
func NewMetaAdsActivityHistoryService() (*MetaAdsActivityHistoryService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsActivityHistory)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s", global.MongoDB_ColNames.AdsActivityHistory)
	}
	return &MetaAdsActivityHistoryService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[metamodels.AdsActivityHistory](coll),
	}, nil
}
