// Package metasvc - Service quản lý meta ad sets.
package metasvc

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	metamodels "meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"
)

// MetaAdSetService service quản lý meta ad sets.
// Kế thừa BaseServiceMongoImpl, dùng Upsert từ base (filter adSetId).
type MetaAdSetService struct {
	*basesvc.BaseServiceMongoImpl[metamodels.MetaAdSet]
}

// NewMetaAdSetService tạo MetaAdSetService.
func NewMetaAdSetService() (*MetaAdSetService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdSets)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_adsets")
	}
	return &MetaAdSetService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[metamodels.MetaAdSet](coll),
	}, nil
}
