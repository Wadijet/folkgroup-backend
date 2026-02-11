// Package deliverysvc - DeliveryHistoryService (xem service.delivery.queue.go cho package doc).
// File: service.delivery.history.go - giữ tên cấu trúc cũ (service.<domain>.<entity>.go).
package deliverysvc

import (
	"context"
	"fmt"

	deliverymodels "meta_commerce/internal/api/delivery/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"

	"go.mongodb.org/mongo-driver/mongo/options"
)

// DeliveryHistoryService là service quản lý Delivery History (lịch sử gửi notification).
type DeliveryHistoryService struct {
	*basesvc.BaseServiceMongoImpl[deliverymodels.DeliveryHistory]
}

// NewDeliveryHistoryService tạo mới DeliveryHistoryService
func NewDeliveryHistoryService() (*DeliveryHistoryService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DeliveryHistory)
	if !exist {
		return nil, fmt.Errorf("failed to get delivery_history collection: %v", common.ErrNotFound)
	}

	return &DeliveryHistoryService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[deliverymodels.DeliveryHistory](collection),
	}, nil
}

// FindOne wrapper để package khác gọi được
func (s *DeliveryHistoryService) FindOne(ctx context.Context, filter interface{}, opts *options.FindOneOptions) (deliverymodels.DeliveryHistory, error) {
	return s.BaseServiceMongoImpl.FindOne(ctx, filter, opts)
}

// InsertOne wrapper
func (s *DeliveryHistoryService) InsertOne(ctx context.Context, data deliverymodels.DeliveryHistory) (deliverymodels.DeliveryHistory, error) {
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// UpdateOne wrapper
func (s *DeliveryHistoryService) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (deliverymodels.DeliveryHistory, error) {
	return s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, opts)
}
