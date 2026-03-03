package pcsvc

import (
	"context"
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// PcPosWarehouseService là cấu trúc chứa các phương thức liên quan đến Pancake POS Warehouse
type PcPosWarehouseService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosWarehouse]
}

// NewPcPosWarehouseService tạo mới PcPosWarehouseService
func NewPcPosWarehouseService() (*PcPosWarehouseService, error) {
	warehouseCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosWarehouses)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_warehouses collection: %v", common.ErrNotFound)
	}

	return &PcPosWarehouseService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosWarehouse](warehouseCollection),
	}, nil
}

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (panCakeUpdatedAt) hoặc document chưa tồn tại.
func (s *PcPosWarehouseService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosWarehouse, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "panCakeData", "panCakeUpdatedAt")
}
