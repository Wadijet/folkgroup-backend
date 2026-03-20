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

// RunSyncUpsertOneFromJSON logic đồng bộ với HandleSyncUpsertOne (parse body + extract + SyncUpsertOne).
func (s *PcPosShopService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosShop, bool, error) {
	var zero pcmodels.PcPosShop
	var shop pcmodels.PcPosShop
	if err := json.Unmarshal(body, &shop); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && shop.OwnerOrganizationID.IsZero() {
		shop.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&shop); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &shop)
}
