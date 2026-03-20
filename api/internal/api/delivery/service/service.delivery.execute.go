// Package deliverysvc — Service thực thi ExecutionActionInput (Execution Engine).
//
// ExecuteActions nhận actions từ AI Decision Engine, route SEND_MESSAGE → delivery queue.
// Handler và AI Decision Engine đều gọi service này.
//
// Dùng DeliveryQueueService trực tiếp (không qua delivery.Queue) để tránh import cycle.
package deliverysvc

import (
	"context"
	"fmt"
	"time"

	deliverydto "meta_commerce/internal/api/delivery/dto"
	deliverymodels "meta_commerce/internal/api/delivery/models"
	notifmodels "meta_commerce/internal/api/notification/models"
	notifsvc "meta_commerce/internal/api/notification/service"
	"meta_commerce/internal/notification"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DeliveryExecuteService service thực thi actions.
type DeliveryExecuteService struct {
	queueService *DeliveryQueueService
}

// NewDeliveryExecuteService tạo service mới.
func NewDeliveryExecuteService() (*DeliveryExecuteService, error) {
	queueSvc, err := NewDeliveryQueueService()
	if err != nil {
		return nil, err
	}
	return &DeliveryExecuteService{queueService: queueSvc}, nil
}

// ExecuteActions thực thi danh sách actions — SEND_MESSAGE → queue, các action khác → unsupported.
// Gọi từ handler hoặc AI Decision Engine.
func (s *DeliveryExecuteService) ExecuteActions(ctx context.Context, actions []deliverydto.ExecutionActionInput, ownerOrgID primitive.ObjectID) (queued int, err error) {
	var queueItems []*deliverymodels.DeliveryQueueItem
	for _, act := range actions {
		if act.ActionType != deliverydto.ActionTypeSendMessage {
			continue
		}
		recipient := ""
		if act.Payload != nil {
			if r, ok := act.Payload["recipient"].(string); ok {
				recipient = r
			}
		}
		if recipient == "" {
			recipient = act.Target.CustomerID
		}
		content := ""
		if act.Payload != nil {
			if ct, ok := act.Payload["content"].(string); ok {
				content = ct
			}
		}
		channelType := act.Target.Channel
		if channelType == "" {
			channelType = "messenger"
		}
		senderID := primitive.NilObjectID
		if senderSvc, err := notifsvc.NewNotificationSenderService(); err == nil {
			_, sid, _ := findSenderForChannelType(ctx, senderSvc, channelType, ownerOrgID)
			senderID = sid
		}
		severity := "info"
		priority := notification.GetPriorityFromSeverity(severity)
		maxRetries := notification.GetMaxRetriesFromSeverity(severity)
		item := &deliverymodels.DeliveryQueueItem{
			ID:                  primitive.NewObjectID(),
			EventType:           "execution_send_message",
			OwnerOrganizationID: ownerOrgID,
			SenderID:            senderID,
			ChannelType:         channelType,
			Recipient:           recipient,
			Content:             content,
			Payload:             act.Payload,
			Status:              "pending",
			RetryCount:          0,
			MaxRetries:          maxRetries,
			Priority:            priority,
		}
		queueItems = append(queueItems, item)
	}
	if len(queueItems) == 0 {
		return 0, nil
	}
	now := time.Now().Unix()
	for _, item := range queueItems {
		item.Status = "pending"
		item.RetryCount = 0
		if item.MaxRetries == 0 {
			item.MaxRetries = 3
		}
		if item.Priority == 0 {
			item.Priority = 3
		}
		item.CreatedAt = now
		item.UpdatedAt = now
	}
	itemsToInsert := make([]deliverymodels.DeliveryQueueItem, len(queueItems))
	for i, item := range queueItems {
		itemsToInsert[i] = *item
	}
	if _, err := s.queueService.InsertMany(ctx, itemsToInsert); err != nil {
		return 0, err
	}
	return len(queueItems), nil
}

func findSenderForChannelType(ctx context.Context, senderService *notifsvc.NotificationSenderService, channelType string, organizationID primitive.ObjectID) (*notifmodels.NotificationChannelSender, primitive.ObjectID, error) {
	filter := bson.M{
		"channelType": channelType,
		"isActive":    true,
		"$or": []bson.M{
			{"ownerOrganizationId": organizationID},
			{"ownerOrganizationId": nil},
		},
	}
	senders, err := senderService.Find(ctx, filter, nil)
	if err == nil && len(senders) > 0 {
		for _, s := range senders {
			if s.OwnerOrganizationID != nil && s.OwnerOrganizationID.Hex() == organizationID.Hex() {
				return &s, s.ID, nil
			}
		}
		for _, s := range senders {
			if s.OwnerOrganizationID == nil {
				return &s, s.ID, nil
			}
		}
		return &senders[0], senders[0].ID, nil
	}
	return nil, primitive.NilObjectID, fmt.Errorf("no active sender for channel %s", channelType)
}
