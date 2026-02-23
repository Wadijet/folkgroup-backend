package pcsvc

import (
	"context"
	"fmt"
	"time"

	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		CollectionName: global.MongoDB_ColNames.PcPosOrders,
		Operation:      events.OpUpdate,
		Document:       order,
	})
	return order, nil
}
