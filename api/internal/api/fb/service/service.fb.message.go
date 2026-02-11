package fbsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// FbMessageService là cấu trúc chứa các phương thức liên quan đến tin nhắn Facebook
type FbMessageService struct {
	*basesvc.BaseServiceMongoImpl[fbmodels.FbMessage]
	fbPageService        *FbPageService
	fbMessageItemService *FbMessageItemService
}

// NewFbMessageService tạo mới FbMessageService
func NewFbMessageService() (*FbMessageService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.FbMessages)
	if !exist {
		return nil, fmt.Errorf("failed to get fb_messages collection: %v", common.ErrNotFound)
	}
	fbPageService, err := NewFbPageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb_page service: %v", err)
	}
	fbMessageItemService, err := NewFbMessageItemService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb_message_item service: %v", err)
	}
	return &FbMessageService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[fbmodels.FbMessage](coll),
		fbPageService:        fbPageService,
		fbMessageItemService: fbMessageItemService,
	}, nil
}

// IsMessageExist kiểm tra tin nhắn có tồn tại hay không
func (s *FbMessageService) IsMessageExist(ctx context.Context, conversationId string, customerId string) (bool, error) {
	filter := bson.M{"conversationId": conversationId, "customerId": customerId}
	var message fbmodels.FbMessage
	err := s.Collection().FindOne(ctx, filter).Decode(&message)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, common.ConvertMongoError(err)
	}
	return true, nil
}

// FindOneByConversationID tìm một FbMessage theo ConversationID
func (s *FbMessageService) FindOneByConversationID(ctx context.Context, conversationID string) (fbmodels.FbMessage, error) {
	filter := bson.M{"conversationId": conversationID}
	var message fbmodels.FbMessage
	err := s.Collection().FindOne(ctx, filter).Decode(&message)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return message, common.ErrNotFound
		}
		return message, common.ConvertMongoError(err)
	}
	return message, nil
}

// FindAll tìm tất cả các FbMessage với phân trang
func (s *FbMessageService) FindAll(ctx context.Context, page int64, limit int64) ([]fbmodels.FbMessage, error) {
	opts := options.Find().
		SetSkip((page - 1) * limit).
		SetLimit(limit).
		SetSort(bson.D{{Key: "updatedAt", Value: 1}})
	cursor, err := s.Collection().Find(ctx, nil, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)
	var results []fbmodels.FbMessage
	if err = cursor.All(ctx, &results); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return results, nil
}

// UpsertMessages xử lý upsert messages từ panCakeData
func (s *FbMessageService) UpsertMessages(ctx context.Context, conversationId string, pageId string, pageUsername string, customerId string, panCakeData map[string]interface{}, hasMore bool) (fbmodels.FbMessage, error) {
	now := time.Now().UnixMilli()
	messages, _ := panCakeData["messages"].([]interface{})
	metadataPanCakeData := make(map[string]interface{})
	for k, v := range panCakeData {
		if k != "messages" {
			metadataPanCakeData[k] = v
		}
	}
	if _, exists := metadataPanCakeData["conversation_id"]; !exists {
		metadataPanCakeData["conversation_id"] = conversationId
	}
	filter := bson.M{"conversationId": conversationId}
	var existingDoc fbmodels.FbMessage
	err := s.Collection().FindOne(ctx, filter).Decode(&existingDoc)
	exists := err == nil
	mergedPanCakeData := make(map[string]interface{})
	if exists && existingDoc.PanCakeData != nil {
		for k, v := range existingDoc.PanCakeData {
			mergedPanCakeData[k] = v
		}
		logrus.WithFields(logrus.Fields{"conversationId": conversationId}).Debug("UpsertMessages: Đã copy panCakeData cũ")
	}
	for k, v := range metadataPanCakeData {
		if existingMap, ok := mergedPanCakeData[k].(map[string]interface{}); ok {
			if newMap, ok := v.(map[string]interface{}); ok {
				for nk, nv := range newMap {
					existingMap[nk] = nv
				}
				mergedPanCakeData[k] = existingMap
			} else {
				mergedPanCakeData[k] = v
			}
		} else {
			mergedPanCakeData[k] = v
		}
	}
	delete(mergedPanCakeData, "messages")
	update := bson.M{
		"$set": bson.M{
			"pageId": pageId, "pageUsername": pageUsername, "customerId": customerId,
			"panCakeData": mergedPanCakeData, "lastSyncedAt": now, "hasMore": hasMore, "updatedAt": now,
		},
		"$setOnInsert": bson.M{"createdAt": now},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var metadataResult fbmodels.FbMessage
	err = s.Collection().FindOneAndUpdate(ctx, filter, update, opts).Decode(&metadataResult)
	if err != nil {
		return metadataResult, common.ConvertMongoError(err)
	}
	if len(messages) > 0 {
		_, err = s.fbMessageItemService.UpsertMessages(ctx, conversationId, messages)
		if err != nil {
			return metadataResult, fmt.Errorf("failed to upsert messages: %v", err)
		}
	}
	totalMessages, err := s.fbMessageItemService.CountByConversationId(ctx, conversationId)
	if err != nil {
		return metadataResult, fmt.Errorf("failed to count messages: %v", err)
	}
	update = bson.M{"$set": bson.M{"totalMessages": totalMessages, "updatedAt": now}}
	opts = options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated fbmodels.FbMessage
	err = s.Collection().FindOneAndUpdate(ctx, filter, update, opts).Decode(&updated)
	if err != nil {
		return metadataResult, common.ConvertMongoError(err)
	}
	return updated, nil
}
