package pcsvc

import (
	"context"
	"fmt"

	basesvc "meta_commerce/internal/api/base/service"
	pcmodels "meta_commerce/internal/api/pc/models"
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

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (posUpdatedAt) hoặc document chưa tồn tại.
func (s *PcPosVariationService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosVariation, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}
