package fbsvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/api/events"
	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
)

// FbCustomerService là cấu trúc chứa các phương thức liên quan đến Facebook Customer
type FbCustomerService struct {
	*basesvc.BaseServiceMongoImpl[fbmodels.FbCustomer]
}

// NewFbCustomerService tạo mới FbCustomerService
func NewFbCustomerService() (*FbCustomerService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.FbCustomers)
	if !exist {
		return nil, fmt.Errorf("failed to get fb_customers collection: %v", common.ErrNotFound)
	}
	return &FbCustomerService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[fbmodels.FbCustomer](coll),
	}, nil
}

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (panCakeUpdatedAt) hoặc document chưa tồn tại.
func (s *FbCustomerService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (fbmodels.FbCustomer, bool, error) {
	var zero fbmodels.FbCustomer
	updateData, err := basesvc.ToUpdateData(data)
	if err != nil {
		return zero, false, common.ErrInvalidFormat
	}
	var newUpdatedAt int64
	if set := updateData.Set; set != nil {
		if panCake, ok := set["panCakeData"].(map[string]interface{}); ok {
			newUpdatedAt = utility.ParseTimestampFromMap(panCake, "updated_at")
		}
	}
	condFilter := basesvc.BuildSyncUpsertFilter(filter, "panCakeUpdatedAt", newUpdatedAt)
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
	var updated fbmodels.FbCustomer
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
