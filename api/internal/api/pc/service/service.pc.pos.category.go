package pcsvc

import (
	"fmt"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// PcPosCategoryService là cấu trúc chứa các phương thức liên quan đến Pancake POS Category
type PcPosCategoryService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosCategory]
}

// NewPcPosCategoryService tạo mới PcPosCategoryService
func NewPcPosCategoryService() (*PcPosCategoryService, error) {
	categoryCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCategories)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_categories collection: %v", common.ErrNotFound)
	}

	return &PcPosCategoryService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosCategory](categoryCollection),
	}, nil
}
