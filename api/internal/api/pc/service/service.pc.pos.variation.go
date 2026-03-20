package pcsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"

	basesvc "meta_commerce/internal/api/base/service"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
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

// RunSyncUpsertOneFromJSON logic đồng bộ với HandleSyncUpsertOne (parse body + extract + SyncUpsertOne).
func (s *PcPosVariationService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosVariation, bool, error) {
	var zero pcmodels.PcPosVariation
	var variation pcmodels.PcPosVariation
	if err := json.Unmarshal(body, &variation); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && variation.OwnerOrganizationID.IsZero() {
		variation.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&variation); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &variation)
}
