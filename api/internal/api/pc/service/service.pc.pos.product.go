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

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (posUpdatedAt) hoặc document chưa tồn tại.
func (s *PcPosProductService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosProduct, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "posData", "posUpdatedAt")
}

// RunSyncUpsertOneFromJSON logic đồng bộ với HandleSyncUpsertOne (parse body + extract + SyncUpsertOne).
func (s *PcPosProductService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosProduct, bool, error) {
	var zero pcmodels.PcPosProduct
	var product pcmodels.PcPosProduct
	if err := json.Unmarshal(body, &product); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && product.OwnerOrganizationID.IsZero() {
		product.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&product); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu posData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &product)
}
