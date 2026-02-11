package pcsvc

import (
	"fmt"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
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
