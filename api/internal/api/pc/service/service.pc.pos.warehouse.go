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

// RunSyncUpsertOneFromJSON logic đồng bộ với HandleSyncUpsertOne (parse body + extract + SyncUpsertOne).
func (s *PcPosWarehouseService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (pcmodels.PcPosWarehouse, bool, error) {
	var zero pcmodels.PcPosWarehouse
	var wh pcmodels.PcPosWarehouse
	if err := json.Unmarshal(body, &wh); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && wh.OwnerOrganizationID.IsZero() {
		wh.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&wh); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &wh)
}
