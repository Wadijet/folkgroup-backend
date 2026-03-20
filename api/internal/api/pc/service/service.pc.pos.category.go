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

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (posUpdatedAt) hoặc document chưa tồn tại.
func (s *PcPosCategoryService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosCategory, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}

// RunSyncUpsertOneFromJSON logic đồng bộ với HandleSyncUpsertOne (parse body + extract + SyncUpsertOne).
func (s *PcPosCategoryService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosCategory, bool, error) {
	var zero pcmodels.PcPosCategory
	var category pcmodels.PcPosCategory
	if err := json.Unmarshal(body, &category); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && category.OwnerOrganizationID.IsZero() {
		category.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&category); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &category)
}
