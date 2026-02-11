package pcsvc

import (
	"fmt"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// PcPosShopService là cấu trúc chứa các phương thức liên quan đến Pancake POS Shop
type PcPosShopService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosShop]
}

// NewPcPosShopService tạo mới PcPosShopService
func NewPcPosShopService() (*PcPosShopService, error) {
	shopCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosShops)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_shops collection: %v", common.ErrNotFound)
	}

	return &PcPosShopService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosShop](shopCollection),
	}, nil
}
