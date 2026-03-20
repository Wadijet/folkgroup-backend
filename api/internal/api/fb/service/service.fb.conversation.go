package fbsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	basemodels "meta_commerce/internal/api/base/models"
	basesvc "meta_commerce/internal/api/base/service"
	fbmodels "meta_commerce/internal/api/fb/models"
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
// Dùng chung logic với Upsert; khác biệt duy nhất là so sánh updated_at.
func (s *FbConversationService) SyncUpsertOne(ctx context.Context, filter interface{}, data interface{}) (fbmodels.FbConversation, bool, error) {
	return basesvc.DoSyncUpsert(ctx, s.BaseServiceMongoImpl, filter, data, "panCakeData", "panCakeUpdatedAt")
}

// RunSyncUpsertOneFromJSON logic đồng bộ với HandleSyncUpsertOne (parse body + extract + SyncUpsertOne).
func (s *FbConversationService) RunSyncUpsertOneFromJSON(ctx context.Context, filter map[string]interface{}, body []byte, activeOrgID *primitive.ObjectID) (fbmodels.FbConversation, bool, error) {
	var zero fbmodels.FbConversation
	var conv fbmodels.FbConversation
	if err := json.Unmarshal(body, &conv); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Body không đúng định dạng JSON", common.StatusBadRequest, err)
	}
	if activeOrgID != nil && !activeOrgID.IsZero() && conv.OwnerOrganizationID.IsZero() {
		conv.OwnerOrganizationID = *activeOrgID
	}
	if err := utility.ExtractDataIfExists(&conv); err != nil {
		return zero, false, common.NewError(common.ErrCodeValidationFormat, "Dữ liệu panCakeData không hợp lệ: "+err.Error(), common.StatusBadRequest, err)
	}
	return s.SyncUpsertOne(ctx, filter, &conv)
}
