package delivery

import (
	"context"
	"fmt"
	"time"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// Queue xử lý việc enqueue và dequeue
type Queue struct {
	queueService *services.NotificationQueueService
}

// NewQueue tạo mới Queue
func NewQueue() (*Queue, error) {
	queueService, err := services.NewNotificationQueueService()
	if err != nil {
		return nil, fmt.Errorf("failed to create queue service: %w", err)
	}

	return &Queue{
		queueService: queueService,
	}, nil
}

// Enqueue thêm items vào queue
func (q *Queue) Enqueue(ctx context.Context, items []*models.NotificationQueueItem) error {
	now := time.Now().Unix()
	for _, item := range items {
		item.Status = "pending"
		item.RetryCount = 0
		item.MaxRetries = 3
		item.CreatedAt = now
		item.UpdatedAt = now
	}

	// Convert []*models.NotificationQueueItem to []models.NotificationQueueItem
	itemsToInsert := make([]models.NotificationQueueItem, len(items))
	for i, item := range items {
		itemsToInsert[i] = *item
	}

	_, err := q.queueService.InsertMany(ctx, itemsToInsert)
	return err
}

// Dequeue lấy items từ queue (status="pending", limit)
func (q *Queue) Dequeue(ctx context.Context, limit int) ([]*models.NotificationQueueItem, error) {
	items, err := q.queueService.FindPending(ctx, limit)
	if err != nil {
		return nil, err
	}

	// Update status to "processing"
	ids := make([]interface{}, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}

	err = q.queueService.UpdateStatus(ctx, ids, "processing")
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	result := make([]*models.NotificationQueueItem, len(items))
	for i := range items {
		result[i] = &items[i]
	}

	return result, nil
}
