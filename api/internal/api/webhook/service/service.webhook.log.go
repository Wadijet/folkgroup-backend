// Package webhooksvc chứa service cho domain Webhook (log).
// File: service.webhook.log.go
package webhooksvc

import (
	"context"
	"fmt"
	"time"

	basesvc "meta_commerce/internal/api/base/service"
	webhookmodels "meta_commerce/internal/api/webhook/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// WebhookLogService là cấu trúc chứa các phương thức liên quan đến webhook logs
type WebhookLogService struct {
	*basesvc.BaseServiceMongoImpl[webhookmodels.WebhookLog]
}

// NewWebhookLogService tạo mới WebhookLogService
func NewWebhookLogService() (*WebhookLogService, error) {
	webhookLogCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.WebhookLogs)
	if !exist {
		return nil, fmt.Errorf("failed to get webhook_logs collection: %v", common.ErrNotFound)
	}

	return &WebhookLogService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[webhookmodels.WebhookLog](webhookLogCollection),
	}, nil
}

// CreateWebhookLog tạo mới webhook log
func (s *WebhookLogService) CreateWebhookLog(ctx context.Context, log webhookmodels.WebhookLog) (*webhookmodels.WebhookLog, error) {
	result, err := s.InsertOne(ctx, log)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateProcessedStatus cập nhật trạng thái đã xử lý của webhook log
func (s *WebhookLogService) UpdateProcessedStatus(ctx context.Context, logID primitive.ObjectID, processed bool, errorMsg string) error {
	filter := bson.M{"_id": logID}
	update := bson.M{
		"$set": bson.M{
			"processed":    processed,
			"processError": errorMsg,
			"processedAt":  0,
			"updatedAt":    0,
		},
	}

	if processed {
		update["$set"].(bson.M)["processedAt"] = time.Now().UnixMilli()
	}
	update["$set"].(bson.M)["updatedAt"] = time.Now().UnixMilli()

	opts := options.Update()
	_, err := s.Collection().UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	return nil
}
