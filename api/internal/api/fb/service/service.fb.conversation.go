package fbsvc

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	basemodels "meta_commerce/internal/api/base/models"
	basesvc "meta_commerce/internal/api/base/service"
	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
)

// FbConversationService là cấu trúc chứa các phương thức liên quan đến Facebook conversation
type FbConversationService struct {
	*basesvc.BaseServiceMongoImpl[fbmodels.FbConversation]
	fbPageService    *FbPageService
	fbMessageService *FbMessageService
}

// NewFbConversationService tạo mới FbConversationService
func NewFbConversationService() (*FbConversationService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !exist {
		return nil, fmt.Errorf("failed to get fb_conversations collection: %v", common.ErrNotFound)
	}
	fbPageService, err := NewFbPageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb_page service: %v", err)
	}
	fbMessageService, err := NewFbMessageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb_message service: %v", err)
	}
	return &FbConversationService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[fbmodels.FbConversation](coll),
		fbPageService:        fbPageService,
		fbMessageService:     fbMessageService,
	}, nil
}

// IsConversationIdExist kiểm tra ID cuộc trò chuyện có tồn tại hay không
func (s *FbConversationService) IsConversationIdExist(ctx context.Context, conversationId string) (bool, error) {
	filter := bson.M{"conversationId": conversationId}
	var conversation fbmodels.FbConversation
	err := s.Collection().FindOne(ctx, filter).Decode(&conversation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FindAllSortByApiUpdate tìm tất cả các FbConversation với phân trang sắp xếp theo panCakeUpdatedAt
func (s *FbConversationService) FindAllSortByApiUpdate(ctx context.Context, page int64, limit int64, filter bson.M) (*basemodels.PaginateResult[fbmodels.FbConversation], error) {
	opts := options.Find().SetSort(bson.D{{Key: "panCakeUpdatedAt", Value: -1}})
	return s.BaseServiceMongoImpl.FindWithPagination(ctx, filter, page, limit, opts)
}

// SyncUpsertOne thực hiện upsert có điều kiện: chỉ ghi khi dữ liệu mới hơn (panCakeUpdatedAt) hoặc document chưa tồn tại.
// Giảm tải backend khi sync incremental: bỏ qua các document không thay đổi.
// Trả về (model, skipped, error). skipped=true khi không ghi.
func (s *FbConversationService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (fbmodels.FbConversation, bool, error) {
	var zero fbmodels.FbConversation
	updateData, err := basesvc.ToUpdateData(data)
	if err != nil {
		return zero, false, common.ErrInvalidFormat
	}
	// Lấy updated_at từ panCakeData
	var newUpdatedAt int64
	if set := updateData.Set; set != nil {
		if panCake, ok := set["panCakeData"].(map[string]interface{}); ok {
			newUpdatedAt = utility.ParseTimestampFromMap(panCake, "updated_at")
		}
	}
	// Build filter có điều kiện: chỉ update khi panCakeUpdatedAt < newUpdatedAt hoặc chưa có
	condFilter := basesvc.BuildSyncUpsertFilter(filter, "panCakeUpdatedAt", newUpdatedAt)
	now := time.Now().UnixMilli()
	if updateData.Set == nil {
		updateData.Set = make(map[string]interface{})
	}
	updateData.Set["updatedAt"] = now
	updateData.Set["createdAt"] = now
	opts := options.Update().SetUpsert(true)
	updateDoc := bson.M{}
	if updateData.Set != nil {
		updateDoc["$set"] = updateData.Set
	}
	if updateData.SetOnInsert != nil {
		updateDoc["$setOnInsert"] = updateData.SetOnInsert
	}
	if updateData.Unset != nil {
		updateDoc["$unset"] = updateData.Unset
	}
	result, err := s.Collection().UpdateOne(ctx, condFilter, updateDoc, opts)
	if err != nil {
		return zero, false, common.ConvertMongoError(err)
	}
	if result.MatchedCount == 0 && result.ModifiedCount == 0 && result.UpsertedCount == 0 {
		return zero, true, nil
	}
	var updated fbmodels.FbConversation
	if result.UpsertedID != nil {
		_ = s.Collection().FindOne(ctx, bson.M{"_id": result.UpsertedID}).Decode(&updated)
		events.EmitDataChanged(ctx, events.DataChangeEvent{
			CollectionName: s.Collection().Name(),
			Operation:       events.OpUpsert,
			Document:        updated,
		})
	} else if result.ModifiedCount > 0 {
		filterMap, _ := filter.(map[string]interface{})
		if filterMap != nil {
			_ = s.Collection().FindOne(ctx, filter).Decode(&updated)
			events.EmitDataChanged(ctx, events.DataChangeEvent{
				CollectionName: s.Collection().Name(),
				Operation:       events.OpUpdate,
				Document:        updated,
			})
		}
	}
	return updated, false, nil
}
