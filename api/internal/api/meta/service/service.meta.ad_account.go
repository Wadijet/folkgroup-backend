// Package metasvc - Service quản lý meta ad accounts.
package metasvc

import (
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	metamodels "meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"
)

// MetaAdAccountService service quản lý meta ad accounts.
// Chỉ kế thừa BaseServiceMongoImpl, dùng Upsert từ base (filter adAccountId).
type MetaAdAccountService struct {
	*basesvc.BaseServiceMongoImpl[metamodels.MetaAdAccount]
}

// NewMetaAdAccountService tạo MetaAdAccountService.
func NewMetaAdAccountService() (*MetaAdAccountService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ad_accounts")
	}
	return &MetaAdAccountService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[metamodels.MetaAdAccount](coll),
	}, nil
}
