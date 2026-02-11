package fbsvc

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	fbmodels "meta_commerce/internal/api/fb/models"
	fbdto "meta_commerce/internal/api/fb/dto"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	basesvc "meta_commerce/internal/api/base/service"
)

// FbPostService là cấu trúc chứa các phương thức liên quan đến bài viết Facebook
type FbPostService struct {
	*basesvc.BaseServiceMongoImpl[fbmodels.FbPost]
	fbPageService *FbPageService
}

// NewFbPostService tạo mới FbPostService
func NewFbPostService() (*FbPostService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.FbPosts)
	if !exist {
		return nil, fmt.Errorf("failed to get fb_posts collection: %v", common.ErrNotFound)
	}
	fbPageService, err := NewFbPageService()
	if err != nil {
		return nil, fmt.Errorf("failed to create fb_page service: %v", err)
	}
	return &FbPostService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[fbmodels.FbPost](coll),
		fbPageService:        fbPageService,
	}, nil
}

// IsPostExist kiểm tra bài viết có tồn tại hay không
func (s *FbPostService) IsPostExist(ctx context.Context, postId string) (bool, error) {
	filter := bson.M{"postId": postId}
	var post fbmodels.FbPost
	err := s.Collection().FindOne(ctx, filter).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, common.ConvertMongoError(err)
	}
	return true, nil
}

// FindOneByPostID tìm một FbPost theo PostID
func (s *FbPostService) FindOneByPostID(ctx context.Context, postID string) (fbmodels.FbPost, error) {
	filter := bson.M{"postId": postID}
	var post fbmodels.FbPost
	err := s.Collection().FindOne(ctx, filter).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return post, common.ErrNotFound
		}
		return post, common.ConvertMongoError(err)
	}
	return post, nil
}

// FindAll tìm kiếm tất cả bài viết
func (s *FbPostService) FindAll(ctx context.Context, page int64, limit int64) ([]fbmodels.FbPost, error) {
	opts := options.Find().
		SetSkip((page - 1) * limit).
		SetLimit(limit).
		SetSort(bson.D{{Key: "updatedAt", Value: 1}})
	cursor, err := s.Collection().Find(ctx, nil, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)
	var results []fbmodels.FbPost
	if err = cursor.All(ctx, &results); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return results, nil
}

// UpdateToken cập nhật token/panCakeData của một FbPost theo ID
func (s *FbPostService) UpdateToken(ctx context.Context, input *fbdto.FbPostUpdateTokenInput) (*fbmodels.FbPost, error) {
	post, err := s.FindOneByPostID(ctx, input.PostId)
	if err != nil {
		return nil, err
	}
	updateData := &basesvc.UpdateData{
		Set: map[string]interface{}{"panCakeData": input.PanCakeData},
	}
	updatedPost, err := s.BaseServiceMongoImpl.UpdateById(ctx, post.ID, updateData)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &updatedPost, nil
}
