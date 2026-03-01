package delivery

import (
	"context"
	"fmt"
	"time"

	deliverymodels "meta_commerce/internal/api/delivery/models"
	deliverysvc "meta_commerce/internal/api/delivery/service"
	"meta_commerce/internal/logger"
)

// Queue xử lý việc enqueue và dequeue
type Queue struct {
	queueService *deliverysvc.DeliveryQueueService
}

// NewQueue tạo mới Queue
func NewQueue() (*Queue, error) {
	queueService, err := deliverysvc.NewDeliveryQueueService()
	if err != nil {
		return nil, fmt.Errorf("failed to create queue service: %w", err)
	}

	return &Queue{
		queueService: queueService,
	}, nil
}

// Enqueue thêm items vào queue
func (q *Queue) Enqueue(ctx context.Context, items []*deliverymodels.DeliveryQueueItem) error {
	now := time.Now().Unix()
	log := logger.GetAppLogger()

	// Log thông tin items trước khi insert
	eventTypes := make(map[string]int)
	recipients := make(map[string]int)
	channelTypes := make(map[string]int)
	organizationIDs := make(map[string]int)

	for _, item := range items {
		item.Status = "pending"
		item.RetryCount = 0
		// MaxRetries và Priority đã được set ở NotificationTriggerHandler (từ Severity)
		// Chỉ set default nếu chưa có
		if item.MaxRetries == 0 {
			item.MaxRetries = 3 // Default
		}
		if item.Priority == 0 {
			item.Priority = 3 // Default medium
		}
		item.CreatedAt = now
		item.UpdatedAt = now

		// Track statistics
		eventTypes[item.EventType]++
		recipients[item.Recipient]++
		channelTypes[item.ChannelType]++
		organizationIDs[item.OwnerOrganizationID.Hex()]++
	}

	// Log trước khi insert
	log.WithFields(map[string]interface{}{
		"totalItems":      len(items),
		"eventTypes":      eventTypes,
		"uniqueRecipients": len(recipients),
		"channelTypes":    channelTypes,
		"organizationIds": organizationIDs,
		"timestamp":       now,
	}).Info("📦 [DELIVERY] Bắt đầu insert queue items vào database")

	// Convert []*deliverymodels.DeliveryQueueItem to []deliverymodels.DeliveryQueueItem
	itemsToInsert := make([]deliverymodels.DeliveryQueueItem, len(items))
	for i, item := range items {
		itemsToInsert[i] = *item
	}

	insertedItems, err := q.queueService.InsertMany(ctx, itemsToInsert)
	if err != nil {
		log.WithError(err).WithFields(map[string]interface{}{
			"totalItems": len(items),
		}).Error("📦 [DELIVERY] Lỗi khi insert queue items vào database")
		return err
	}

	// Log sau khi insert thành công
	log.WithFields(map[string]interface{}{
		"totalItems":       len(items),
		"insertedCount":    len(insertedItems),
		"eventTypes":       eventTypes,
		"uniqueRecipients": len(recipients),
		"channelTypes":     channelTypes,
		"organizationIds": organizationIDs,
		"timestamp":        now,
	}).Info("📦 [DELIVERY] Đã insert queue items thành công vào database")

	return nil
}

// Dequeue lấy items từ queue (status="pending", limit)
func (q *Queue) Dequeue(ctx context.Context, limit int) ([]*deliverymodels.DeliveryQueueItem, error) {
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
	result := make([]*deliverymodels.DeliveryQueueItem, len(items))
	for i := range items {
		result[i] = &items[i]
	}

	return result, nil
}
