package fbsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
	"meta_commerce/internal/utility/identity"

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

// messagePayloadSourceMS lấy mốc thời gian từ payload Pancake (ưu tiên updated_at, sau đó inserted_at) — Unix ms.
// Khớp layout ISO trong mẫu fb_message_items-sample.json (messageData.inserted_at).
func messagePayloadSourceMS(m map[string]interface{}) int64 {
	if m == nil {
		return 0
	}
	u := utility.ParseTimestampFromMap(m, "updated_at")
	if u > 0 {
		return u
	}
	return utility.ParseTimestampFromMap(m, "inserted_at")
}

// messageItemSyncUnchanged true → không cần ghi DB (đã có bản ≥ incoming theo thời gian hoặc JSON trùng).
func messageItemSyncUnchanged(existingData, incoming map[string]interface{}) bool {
	exTS := messagePayloadSourceMS(existingData)
	inTS := messagePayloadSourceMS(incoming)
	if exTS > 0 && inTS > 0 {
		return exTS >= inTS
	}
	// incoming có mốc thời gian nhưng bản lưu chưa parse được → vẫn ghi để đồng bộ insertedAt/ messageData.
	if inTS > 0 && exTS == 0 {
		return false
	}
	// Một phía thiếu timestamp (hoặc cả hai 0) → so nội dung (cùng package với jsonCanonicalEqual trong service.fb.message.go).
	return jsonCanonicalEqual(existingData, incoming)
}

// UpsertMessages upsert nhiều messages vào collection.
// Bỏ qua ghi khi document đã có cùng messageId và messageData đã ≥ incoming (updated_at/inserted_at trong payload) hoặc JSON trùng khi thiếu mốc thời gian.
func (s *FbMessageItemService) UpsertMessages(ctx context.Context, conversationId string, messages []interface{}) (int, error) {
	if len(messages) == 0 {
		return 0, nil
	}

	type pair struct {
		id  string
		m   map[string]interface{}
	}
	var pairs []pair
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}
		messageId, ok := msgMap["id"].(string)
		if !ok || messageId == "" {
			continue
		}
		pairs = append(pairs, pair{id: messageId, m: msgMap})
	}
	if len(pairs) == 0 {
		return 0, nil
	}

	ids := make([]string, 0, len(pairs))
	for _, p := range pairs {
		ids = append(ids, p.id)
	}
	existingByID := make(map[string]fbmodels.FbMessageItem)
	cursor, errFind := s.Collection().Find(ctx, bson.M{"messageId": bson.M{"$in": ids}})
	if errFind != nil {
		logrus.WithError(errFind).Warn("FbMessageItem UpsertMessages: không đọc được bản hiện có, sẽ bulk ghi đủ batch")
	} else {
		defer cursor.Close(ctx)
		var existings []fbmodels.FbMessageItem
		if allErr := cursor.All(ctx, &existings); allErr != nil {
			logrus.WithError(allErr).Warn("FbMessageItem UpsertMessages: decode bản hiện có lỗi")
		} else {
			for _, e := range existings {
				existingByID[e.MessageId] = e
			}
		}
	}

	var operations []mongo.WriteModel
	now := time.Now().UnixMilli()
	for _, p := range pairs {
		messageId, msgMap := p.id, p.m
		ex, hadExisting := existingByID[messageId]
		if hadExisting && messageItemSyncUnchanged(ex.MessageData, msgMap) {
			logrus.WithFields(logrus.Fields{"messageId": messageId, "conversationId": conversationId}).Debug("FbMessageItem: bỏ qua — không đổi theo inserted_at/updated_at hoặc payload trùng")
			continue
		}
		insertedAt := messagePayloadSourceMS(msgMap)
		docMap := map[string]interface{}{
			"conversationId": conversationId,
			"messageId":      messageId,
			"messageData":    msgMap,
			"insertedAt":     insertedAt,
			"updatedAt":      now,
		}
		var merge map[string]interface{}
		if hadExisting {
			m, errMap := utility.ToMap(ex)
			if errMap != nil {
				return 0, fmt.Errorf("ToMap FbMessageItem: %w", errMap)
			}
			merge = m
		} else {
			merge = map[string]interface{}{
				"_id": primitive.NewObjectID(),
			}
		}
		for k, v := range docMap {
			merge[k] = v
		}
		identity.ScrubEmptyIdentityFieldsFromSet(merge)
		if err := identity.EnrichIdentity4Layers(ctx, global.MongoDB_ColNames.FbMessageItems, merge, nil); err != nil {
			return 0, fmt.Errorf("enrich identity fb_message_items: %w", err)
		}
		setDoc := bson.M{}
		for k, v := range merge {
			if k == "_id" {
				continue
			}
			setDoc[k] = v
		}
		filter := bson.M{"messageId": messageId}
		update := bson.M{
			"$set": setDoc,
			"$setOnInsert": bson.M{
				"createdAt": now,
				"_id":       merge["_id"],
			},
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
