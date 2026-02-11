// Package deliverysvc chứa service data access cho domain Delivery (queue, history).
// Nằm trong folder service/ để đối xứng với dto/, handler/. Base service (BaseServiceMongoImpl) ở api/basesvc.
// File: service.delivery.queue.go - giữ tên cấu trúc cũ (service.<domain>.<entity>.go).
package deliverysvc

import (
	"context"
	"fmt"
	"time"

	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	deliverymodels "meta_commerce/internal/api/delivery/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DeliveryQueueService là service quản lý Delivery Queue (enqueue, dequeue, pending, status).
type DeliveryQueueService struct {
	*basesvc.BaseServiceMongoImpl[deliverymodels.DeliveryQueueItem]
}

// NewDeliveryQueueService tạo mới DeliveryQueueService
func NewDeliveryQueueService() (*DeliveryQueueService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DeliveryQueue)
	if !exist {
		return nil, fmt.Errorf("failed to get delivery_queue collection: %v", common.ErrNotFound)
	}

	return &DeliveryQueueService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[deliverymodels.DeliveryQueueItem](collection),
	}, nil
}

// FindPending tìm các items có status="pending" hoặc "processing" quá lâu (stale)
func (s *DeliveryQueueService) FindPending(ctx context.Context, limit int) ([]deliverymodels.DeliveryQueueItem, error) {
	now := time.Now().Unix()
	staleThreshold := now - 300 // 5 phút trước

	filter := bson.M{
		"$and": []bson.M{
			{
				"$or": []bson.M{
					{"status": "pending"},
					{
						"status":     "processing",
						"updatedAt": bson.M{"$lt": staleThreshold},
					},
				},
			},
			{
				"$or": []bson.M{
					{"nextRetryAt": nil},
					{"nextRetryAt": bson.M{"$lte": now}},
				},
			},
		},
	}

	opts := options.Find().
		SetSort(bson.D{
			{Key: "priority", Value: 1},
			{Key: "createdAt", Value: 1},
		}).
		SetLimit(int64(limit))

	cursor, err := s.Collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []deliverymodels.DeliveryQueueItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// UpdateStatus cập nhật status cho nhiều items
func (s *DeliveryQueueService) UpdateStatus(ctx context.Context, ids []interface{}, status string) error {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	update := bson.M{"$set": bson.M{"status": status, "updatedAt": time.Now().Unix()}}
	_, err := s.Collection().UpdateMany(ctx, filter, update)
	return err
}

// CleanupFailedItems xóa các items failed đã quá N ngày
func (s *DeliveryQueueService) CleanupFailedItems(ctx context.Context, daysOld int) (int64, error) {
	cutoffTime := time.Now().Unix() - int64(daysOld*24*60*60)
	filter := bson.M{
		"status":    "failed",
		"updatedAt": bson.M{"$lt": cutoffTime},
	}
	result, err := s.Collection().DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}
	return result.DeletedCount, nil
}

// FindRecentDuplicates tìm các items duplicate trong khoảng thời gian (tránh spam)
func (s *DeliveryQueueService) FindRecentDuplicates(ctx context.Context, eventType string, recipient string, channelType string, timeWindowSeconds int64) ([]deliverymodels.DeliveryQueueItem, error) {
	now := time.Now().Unix()
	timeThreshold := now - timeWindowSeconds
	filter := bson.M{
		"eventType":   eventType,
		"recipient":   recipient,
		"channelType": channelType,
		"createdAt":   bson.M{"$gte": timeThreshold},
		"status":      bson.M{"$in": []string{"pending", "processing"}},
	}
	opts := options.Find().SetSort(bson.M{"createdAt": -1}).SetLimit(10)
	cursor, err := s.Collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var items []deliverymodels.DeliveryQueueItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// FindStuckItems tìm các items bị kẹt (processing quá lâu hoặc senderID rỗng)
func (s *DeliveryQueueService) FindStuckItems(ctx context.Context, staleMinutes int, limit int) ([]deliverymodels.DeliveryQueueItem, error) {
	now := time.Now().Unix()
	staleThreshold := now - int64(staleMinutes*60)
	zeroObjectID := primitive.NilObjectID
	filter := bson.M{
		"$or": []bson.M{
			{
				"status":     "processing",
				"updatedAt": bson.M{"$lt": staleThreshold},
			},
			{"senderId": zeroObjectID},
		},
	}
	opts := options.Find().SetSort(bson.M{"updatedAt": 1}).SetLimit(int64(limit))
	cursor, err := s.Collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var items []deliverymodels.DeliveryQueueItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// InsertOne wrapper để package khác gọi được
func (s *DeliveryQueueService) InsertOne(ctx context.Context, data deliverymodels.DeliveryQueueItem) (deliverymodels.DeliveryQueueItem, error) {
	return s.BaseServiceMongoImpl.InsertOne(ctx, data)
}

// InsertMany wrapper để package khác gọi được
func (s *DeliveryQueueService) InsertMany(ctx context.Context, data []deliverymodels.DeliveryQueueItem) ([]deliverymodels.DeliveryQueueItem, error) {
	return s.BaseServiceMongoImpl.InsertMany(ctx, data)
}

// UpdateOne wrapper (processor dùng UpdateOne với UpdateData)
func (s *DeliveryQueueService) UpdateOne(ctx context.Context, filter interface{}, update interface{}, opts *options.UpdateOptions) (deliverymodels.DeliveryQueueItem, error) {
	return s.BaseServiceMongoImpl.UpdateOne(ctx, filter, update, opts)
}

// DeleteOne wrapper
func (s *DeliveryQueueService) DeleteOne(ctx context.Context, filter interface{}) error {
	return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
}
