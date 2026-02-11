package fbsvc

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	basemodels "meta_commerce/internal/api/base/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
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
