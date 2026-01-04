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

// NotificationQueueService là cấu trúc chứa các phương thức liên quan đến Notification Queue
type NotificationQueueService struct {
	*BaseServiceMongoImpl[models.NotificationQueueItem]
}

// NewNotificationQueueService tạo mới NotificationQueueService
func NewNotificationQueueService() (*NotificationQueueService, error) {
	collection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.NotificationQueue)
	if !exist {
		return nil, fmt.Errorf("failed to get notification_queue collection: %v", common.ErrNotFound)
	}

	return &NotificationQueueService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.NotificationQueueItem](collection),
	}, nil
}

// FindPending tìm các items có status="pending" hoặc "processing" quá lâu (stale)
// Stale processing items: items có status="processing" nhưng đã quá 5 phút (có thể processor bị crash)
// Chỉ lấy items có nextRetryAt là null hoặc đã đến thời điểm retry
func (s *NotificationQueueService) FindPending(ctx context.Context, limit int) ([]models.NotificationQueueItem, error) {
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

	opts := options.Find().
		SetSort(bson.M{"createdAt": 1}).
		SetLimit(int64(limit))

	cursor, err := s.BaseServiceMongoImpl.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.NotificationQueueItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// UpdateStatus cập nhật status cho nhiều items
func (s *NotificationQueueService) UpdateStatus(ctx context.Context, ids []interface{}, status string) error {
	filter := bson.M{"_id": bson.M{"$in": ids}}
	update := bson.M{"$set": bson.M{"status": status, "updatedAt": time.Now().Unix()}}

	_, err := s.BaseServiceMongoImpl.collection.UpdateMany(ctx, filter, update)
	return err
}

// CleanupFailedItems xóa các items failed đã quá 7 ngày (cleanup old failed items)
func (s *NotificationQueueService) CleanupFailedItems(ctx context.Context, daysOld int) (int64, error) {
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

// FindStuckItems tìm các items bị kẹt (stuck)
// - Items có status="processing" quá lâu (stale)
// - Items có senderID rỗng
func (s *NotificationQueueService) FindStuckItems(ctx context.Context, staleMinutes int, limit int) ([]models.NotificationQueueItem, error) {
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

	var items []models.NotificationQueueItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}

	return items, nil
}

