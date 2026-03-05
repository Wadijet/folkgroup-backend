// Package metasvc - Service quản lý meta ads.
package metasvc

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	metamodels "meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"
)

// MetaAdService service quản lý meta ads.
// Kế thừa BaseServiceMongoImpl, dùng Upsert từ base (filter adId).
type MetaAdService struct {
	*basesvc.BaseServiceMongoImpl[metamodels.MetaAd]
}

// NewMetaAdService tạo MetaAdService.
func NewMetaAdService() (*MetaAdService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ads")
	}
	return &MetaAdService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[metamodels.MetaAd](coll),
	}, nil
}
