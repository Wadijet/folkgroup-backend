package services

import (
	"context"
	"fmt"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DeliveryQueueService là cấu trúc chứa các phương thức liên quan đến Delivery Queue (thuộc Delivery System)
type DeliveryQueueService struct {
	*BaseServiceMongoImpl[models.DeliveryQueueItem]
}

// NewDeliveryQueueService tạo mới DeliveryQueueService
func NewDeliveryQueueService() (*DeliveryQueueService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.DeliveryQueue)
	if !exist {
		return nil, fmt.Errorf("failed to get delivery_queue collection: %v", common.ErrNotFound)
	}

	return &DeliveryQueueService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.DeliveryQueueItem](collection),
	}, nil
}

// FindPending tìm các items có status="pending" hoặc "processing" quá lâu (stale)
// Stale processing items: items có status="processing" nhưng đã quá 5 phút (có thể processor bị crash)
// Chỉ lấy items có nextRetryAt là null hoặc đã đến thời điểm retry
func (s *DeliveryQueueService) FindPending(ctx context.Context, limit int) ([]models.DeliveryQueueItem, error) {
	now := time.Now().Unix()
	staleThreshold := now - 300 // 5 phút trước
	
	filter := bson.M{
		"$and": []bson.M{
			{
				"$or": []bson.M{
					{"status": "pending"},
					{
						"status":     "processing",
						"updatedAt": bson.M{"$lt": staleThreshold}, // Items processing quá lâu
					},
				},
			},
			{
				"$or": []bson.M{
					{"nextRetryAt": nil},                    // Chưa có nextRetryAt (lần đầu)
					{"nextRetryAt": bson.M{"$lte": now}},    // Đã đến thời điểm retry
				},
			},
		},
	}

	// Dùng bson.D cho sort với nhiều keys (MongoDB driver yêu cầu)
	opts := options.Find().
		SetSort(bson.D{
			{Key: "priority", Value: 1},   // Sort theo Priority trước (1=critical xử lý đầu tiên)
			{Key: "createdAt", Value: 1},  // Sau đó sort theo createdAt
		}).
		SetLimit(int64(limit))

	cursor, err := s.BaseServiceMongoImpl.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.DeliveryQueueItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// UpdateStatus cập nhật status cho nhiều items
func (s *DeliveryQueueService) UpdateStatus(ctx context.Context, ids []interface{}, status string) error {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	update := bson.M{"$set": bson.M{"status": status, "updatedAt": time.Now().Unix()}}

	_, err := s.BaseServiceMongoImpl.collection.UpdateMany(ctx, filter, update)
	return err
}

// CleanupFailedItems xóa các items failed đã quá 7 ngày (cleanup old failed items)
func (s *DeliveryQueueService) CleanupFailedItems(ctx context.Context, daysOld int) (int64, error) {
	cutoffTime := time.Now().Unix() - int64(daysOld*24*60*60)
	
	filter := bson.M{
		"status": "failed",
		"updatedAt": bson.M{"$lt": cutoffTime},
	}

	result, err := s.BaseServiceMongoImpl.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return result.DeletedCount, nil
}

// FindRecentDuplicates tìm các items duplicate trong khoảng thời gian gần đây
// Dùng để tránh tạo duplicate items khi client spam hoặc có nhiều channels trùng lặp
// Kiểm tra: eventType, recipient, channelType trong khoảng thời gian (default: 1 phút)
func (s *DeliveryQueueService) FindRecentDuplicates(ctx context.Context, eventType string, recipient string, channelType string, timeWindowSeconds int64) ([]models.DeliveryQueueItem, error) {
	now := time.Now().Unix()
	timeThreshold := now - timeWindowSeconds

	filter := bson.M{
		"eventType":   eventType,
		"recipient":   recipient,
		"channelType": channelType,
		"createdAt":   bson.M{"$gte": timeThreshold}, // Items được tạo trong khoảng thời gian gần đây
		"status":      bson.M{"$in": []string{"pending", "processing"}}, // Chỉ check items chưa completed/failed
	}

	opts := options.Find().SetSort(bson.M{"createdAt": -1}).SetLimit(10)

	cursor, err := s.BaseServiceMongoImpl.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.DeliveryQueueItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// FindStuckItems tìm các items bị kẹt (stuck)
// - Items có status="processing" quá lâu (stale)
// - Items có senderID rỗng
func (s *DeliveryQueueService) FindStuckItems(ctx context.Context, staleMinutes int, limit int) ([]models.DeliveryQueueItem, error) {
	now := time.Now().Unix()
	staleThreshold := now - int64(staleMinutes*60)
	zeroObjectID := primitive.NilObjectID
	
	filter := bson.M{
		"$or": []bson.M{
			{
				"status":     "processing",
				"updatedAt": bson.M{"$lt": staleThreshold}, // Items processing quá lâu
			},
			{
				"senderId": zeroObjectID, // Items có senderID rỗng
			},
		},
	}

	opts := options.Find().
		SetSort(bson.M{"updatedAt": 1}).
		SetLimit(int64(limit))

	cursor, err := s.BaseServiceMongoImpl.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.DeliveryQueueItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	return items, nil
}
