package pcsvc

import (
	"context"
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	pcmodels "meta_commerce/internal/api/pc/models"
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

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (panCakeUpdatedAt) hoặc document chưa tồn tại.
func (s *PcPosShopService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosShop, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "panCakeData", "panCakeUpdatedAt")
}
