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

// FbPageService là cấu trúc chứa các phương thức liên quan đến Facebook page
type FbPageService struct {
	*basesvc.BaseServiceMongoImpl[fbmodels.FbPage]
}

// NewFbPageService tạo mới FbPageService
func NewFbPageService() (*FbPageService, error) {
	coll, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.FbPages)
	if !exist {
		return nil, fmt.Errorf("failed to get fb_pages collection: %v", common.ErrNotFound)
	}
	return &FbPageService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[fbmodels.FbPage](coll),
	}, nil
}

// IsPageExist kiểm tra trang Facebook có tồn tại hay không
func (s *FbPageService) IsPageExist(ctx context.Context, pageId string) (bool, error) {
	filter := bson.M{"pageId": pageId}
	var page fbmodels.FbPage
	err := s.Collection().FindOne(ctx, filter).Decode(&page)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, common.ConvertMongoError(err)
	}
	return true, nil
}

// FindOneByPageID tìm một FbPage theo PageID
func (s *FbPageService) FindOneByPageID(ctx context.Context, pageID string) (fbmodels.FbPage, error) {
	filter := bson.M{"pageId": pageID}
	var page fbmodels.FbPage
	err := s.Collection().FindOne(ctx, filter).Decode(&page)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return page, common.ErrNotFound
		}
		return page, common.ConvertMongoError(err)
	}
	return page, nil
}

// FindAll tìm tất cả các FbPage với phân trang
func (s *FbPageService) FindAll(ctx context.Context, page int64, limit int64) ([]fbmodels.FbPage, error) {
	opts := options.Find().
		SetSkip((page - 1) * limit).
		SetLimit(limit).
		SetSort(bson.D{{Key: "updatedAt", Value: 1}})
	cursor, err := s.Collection().Find(ctx, nil, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)
	var results []fbmodels.FbPage
	if err = cursor.All(ctx, &results); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return results, nil
}

// UpdateToken cập nhật access token của một FbPage theo ID
func (s *FbPageService) UpdateToken(ctx context.Context, input *fbdto.FbPageUpdateTokenInput) (*fbmodels.FbPage, error) {
	page, err := s.FindOneByPageID(ctx, input.PageId)
	if err != nil {
		return nil, err
	}
	updateData := &basesvc.UpdateData{
		Set: map[string]interface{}{"pageAccessToken": input.PageAccessToken},
	}
	updatedPage, err := s.BaseServiceMongoImpl.UpdateById(ctx, page.ID, updateData)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &updatedPage, nil
}
