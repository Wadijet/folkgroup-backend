package pcsvc

import (
	"fmt"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
)

// PcPosVariationService là cấu trúc chứa các phương thức liên quan đến Pancake POS Variation
type PcPosVariationService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosVariation]
}

// NewPcPosVariationService tạo mới PcPosVariationService
func NewPcPosVariationService() (*PcPosVariationService, error) {
	variationCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosVariations)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_variations collection: %v", common.ErrNotFound)
	}

	return &PcPosVariationService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosVariation](variationCollection),
	}, nil
}
