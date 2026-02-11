package fbsvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// FbMessageItemService là cấu trúc chứa các phương thức liên quan đến message items Facebook
type FbMessageItemService struct {
	*basesvc.BaseServiceMongoImpl[fbmodels.FbMessageItem]
}

// NewFbMessageItemService tạo mới FbMessageItemService
func NewFbMessageItemService() (*FbMessageItemService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.FbMessageItems)
	if !exist {
		return nil, fmt.Errorf("failed to get fb_message_items collection: %v", common.ErrNotFound)
	}
	return &FbMessageItemService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[fbmodels.FbMessageItem](coll),
	}, nil
}

// UpsertMessages upsert nhiều messages vào collection
func (s *FbMessageItemService) UpsertMessages(ctx context.Context, conversationId string, messages []interface{}) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}
	var operations []mongo.WriteModel
	now := time.Now().UnixMilli()
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		messageId, ok := msgMap["id"].(string)
		if !ok || messageId == "" {
			continue
		}
		var insertedAt int64 = 0
		if insertedAtStr, ok := msgMap["inserted_at"].(string); ok {
			if t, err := time.Parse("2006-01-02T15:04:05.000000", insertedAtStr); err == nil {
				insertedAt = t.Unix()
			}
		}
		docMap := bson.M{
			"conversationId": conversationId,
			"messageId":      messageId,
			"messageData":    msgMap,
			"insertedAt":     insertedAt,
			"updatedAt":      now,
		}
		filter := bson.M{"messageId": messageId}
		update := bson.M{
			"$set": docMap,
			"$setOnInsert": bson.M{"createdAt": now},
		}
		operations = append(operations, mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true))
	}
	if len(operations) == 0 {
		return 0, nil
	}
	opts := options.BulkWrite().SetOrdered(false)
	result, err := s.Collection().BulkWrite(ctx, operations, opts)
	if err != nil {
		return 0, common.ConvertMongoError(err)
	}
	return int(result.UpsertedCount + result.ModifiedCount), nil
}

// FindByConversationId tìm tất cả messages của một conversation với phân trang
func (s *FbMessageItemService) FindByConversationId(ctx context.Context, conversationId string, page int64, limit int64) ([]fbmodels.FbMessageItem, int64, error) {
	filter := bson.M{"conversationId": conversationId}
	total, err := s.Collection().CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, common.ConvertMongoError(err)
	}
	opts := options.Find().
		SetSkip((page - 1) * limit).
		SetLimit(limit).
		SetSort(bson.D{{Key: "insertedAt", Value: -1}})
	cursor, err := s.Collection().Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)
	var results []fbmodels.FbMessageItem
	if err = cursor.All(ctx, &results); err != nil {
		return nil, 0, common.ConvertMongoError(err)
	}
	return results, total, nil
}

// CountByConversationId đếm số lượng messages của một conversation
func (s *FbMessageItemService) CountByConversationId(ctx context.Context, conversationId string) (int64, error) {
	filter := bson.M{"conversationId": conversationId}
	return s.Collection().CountDocuments(ctx, filter)
}
