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

// WebhookLogService là cấu trúc chứa các phương thức liên quan đến webhook logs
type WebhookLogService struct {
	*BaseServiceMongoImpl[models.WebhookLog]
}

// NewWebhookLogService tạo mới WebhookLogService
func NewWebhookLogService() (*WebhookLogService, error) {
	webhookLogCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.WebhookLogs)
	if !exist {
		return nil, fmt.Errorf("failed to get webhook_logs collection: %v", common.ErrNotFound)
	}

	return &WebhookLogService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.WebhookLog](webhookLogCollection),
	}, nil
}

// CreateWebhookLog tạo mới webhook log
// Tham số:
//   - ctx: Context
//   - log: WebhookLog cần tạo
//
// Trả về:
//   - *models.WebhookLog: Webhook log đã được tạo
//   - error: Lỗi nếu có
func (s *WebhookLogService) CreateWebhookLog(ctx context.Context, log models.WebhookLog) (*models.WebhookLog, error) {
	result, err := s.InsertOne(ctx, log)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateProcessedStatus cập nhật trạng thái đã xử lý của webhook log
// Tham số:
//   - ctx: Context
//   - logID: ID của webhook log
//   - processed: Đã xử lý thành công hay chưa
//   - errorMsg: Thông báo lỗi nếu có
//
// Trả về:
//   - error: Lỗi nếu có
func (s *WebhookLogService) UpdateProcessedStatus(ctx context.Context, logID primitive.ObjectID, processed bool, errorMsg string) error {
	filter := bson.M{"_id": logID}
	update := bson.M{
		"$set": bson.M{
			"processed":    processed,
			"processError": errorMsg,
			"processedAt": 0, // Sẽ set sau
			"updatedAt":   0,  // Sẽ set sau
		},
	}

	if processed {
		update["$set"].(bson.M)["processedAt"] = time.Now().UnixMilli()
	}

	update["$set"].(bson.M)["updatedAt"] = time.Now().UnixMilli()

	opts := options.Update()
	_, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return common.ConvertMongoError(err)
	}

	return nil
}
