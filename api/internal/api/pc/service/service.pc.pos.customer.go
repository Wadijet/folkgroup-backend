package pcsvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/api/events"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
)

// PcPosCustomerService là cấu trúc chứa các phương thức liên quan đến Pancake POS Customer
type PcPosCustomerService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosCustomer]
}

// NewPcPosCustomerService tạo mới PcPosCustomerService
func NewPcPosCustomerService() (*PcPosCustomerService, error) {
	pcPosCustomerCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_customers collection: %v", common.ErrNotFound)
	}

	return &PcPosCustomerService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosCustomer](pcPosCustomerCollection),
	}, nil
}

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (posUpdatedAt) hoặc document chưa tồn tại.
func (s *PcPosCustomerService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosCustomer, bool, error) {
	var zero pcmodels.PcPosCustomer
	updateData, err := basesvc.ToUpdateData(data)
	if err != nil {
		return zero, false, common.ErrInvalidFormat
	}
	var newUpdatedAt int64
	if set := updateData.Set; set != nil {
		if posData, ok := set["posData"].(map[string]interface{}); ok {
			newUpdatedAt = utility.ParseTimestampFromMap(posData, "updated_at")
		}
	}
	condFilter := basesvc.BuildSyncUpsertFilter(filter, "posUpdatedAt", newUpdatedAt)
	now := time.Now().UnixMilli()
	if updateData.Set == nil {
		updateData.Set = make(map[string]interface{})
	}
	updateData.Set["updatedAt"] = now
	updateData.Set["createdAt"] = now
	updateDoc := bson.M{"$set": updateData.Set}
	if updateData.SetOnInsert != nil {
		updateDoc["$setOnInsert"] = updateData.SetOnInsert
	}
	if updateData.Unset != nil {
		updateDoc["$unset"] = updateData.Unset
	}
	result, err := s.Collection().UpdateOne(ctx, condFilter, updateDoc, options.Update().SetUpsert(true))
	if err != nil {
		return zero, false, common.ConvertMongoError(err)
	}
	if result.MatchedCount == 0 && result.ModifiedCount == 0 && result.UpsertedCount == 0 {
		return zero, true, nil
	}
	var updated pcmodels.PcPosCustomer
	if result.UpsertedID != nil {
		_ = s.Collection().FindOne(ctx, bson.M{"_id": result.UpsertedID}).Decode(&updated)
		events.EmitDataChanged(ctx, events.DataChangeEvent{
			CollectionName: s.Collection().Name(),
			Operation:       events.OpUpsert,
			Document:        updated,
		})
	} else if result.ModifiedCount > 0 {
		_ = s.Collection().FindOne(ctx, filter).Decode(&updated)
		events.EmitDataChanged(ctx, events.DataChangeEvent{
			CollectionName: s.Collection().Name(),
			Operation:       events.OpUpdate,
			Document:        updated,
		})
	}
	return updated, false, nil
}
