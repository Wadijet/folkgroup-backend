package pcsvc

import (
	"fmt"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// PcPosProductService là cấu trúc chứa các phương thức liên quan đến Pancake POS Product
type PcPosProductService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosProduct]
}

// NewPcPosProductService tạo mới PcPosProductService
func NewPcPosProductService() (*PcPosProductService, error) {
	productCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosProducts)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_products collection: %v", common.ErrNotFound)
	}

	return &PcPosProductService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosProduct](productCollection),
	}, nil
}
