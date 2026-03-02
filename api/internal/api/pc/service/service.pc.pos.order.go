package pcsvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/api/events"
	pcmodels "meta_commerce/internal/api/pc/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
)

// PcPosOrderService là cấu trúc chứa các phương thức liên quan đến Pancake POS Order.
// Report MarkDirty được xử lý qua event OnDataChanged (package report), không cần override CRUD.
type PcPosOrderService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.PcPosOrder]
}

// NewPcPosOrderService tạo mới PcPosOrderService.
func NewPcPosOrderService() (*PcPosOrderService, error) {
	orderCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !exist {
		return nil, fmt.Errorf("failed to get pc_pos_orders collection: %v", common.ErrNotFound)
	}
	return &PcPosOrderService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.PcPosOrder](orderCollection),
	}, nil
}

// SyncFlattenedFromPosData đọc document theo id, chạy extract từ posData vào các field flatten (billFullName, status, posCreatedAt, ...) rồi ghi lại document.
// Dùng để sửa document cũ thiếu field flatten (ví dụ do webhook cũ hoặc extract lỗi trước khi có unwrap Extended JSON).
// ReplaceOne không đi qua BaseServiceMongoImpl nên cần phát event thủ công.
func (s *PcPosOrderService) SyncFlattenedFromPosData(ctx context.Context, id primitive.ObjectID) (pcmodels.PcPosOrder, error) {
	var zero pcmodels.PcPosOrder
	order, err := s.BaseServiceMongoImpl.FindOneById(ctx, id)
	if err != nil {
		return zero, err
	}
	if len(order.PosData) == 0 {
		return zero, fmt.Errorf("document không có posData để sync")
	}
	prevOrder := order // Lưu bản cũ trước khi mutate để truyền PreviousDocument
	if err := utility.ExtractDataIfExists(&order); err != nil {
		return zero, fmt.Errorf("extract từ posData thất bại: %w", err)
	}
	order.UpdatedAt = time.Now().UnixMilli()
	dataMap, err := utility.ToMap(&order)
	if err != nil {
		return zero, fmt.Errorf("ToMap thất bại: %w", err)
	}
	_, err = s.Collection().ReplaceOne(ctx, bson.M{"_id": id}, dataMap)
	if err != nil {
		return zero, common.ConvertMongoError(err)
	}
	events.EmitDataChanged(ctx, events.DataChangeEvent{
		CollectionName:   global.MongoDB_ColNames.PcPosOrders,
		Operation:        events.OpUpdate,
		Document:         order,
		PreviousDocument: prevOrder,
	})
	return order, nil
}

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (posUpdatedAt) hoặc document chưa tồn tại.
func (s *PcPosOrderService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (pcmodels.PcPosOrder, bool, error) {
	var zero pcmodels.PcPosOrder
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
	var updated pcmodels.PcPosOrder
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
