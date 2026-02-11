package pcsvc

import (
	"context"
	"fmt"
	"time"

	pcdto "meta_commerce/internal/api/pc/dto"
	pcmodels "meta_commerce/internal/api/pc/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AccessTokenService là cấu trúc chứa các phương thức liên quan đến access token
type AccessTokenService struct {
	*basesvc.BaseServiceMongoImpl[pcmodels.AccessToken]
}

// NewAccessTokenService tạo mới AccessTokenService
func NewAccessTokenService() (*AccessTokenService, error) {
	accessTokenCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.AccessTokens)
	if !exist {
		return nil, fmt.Errorf("failed to get access_tokens collection: %v", common.ErrNotFound)
	}

	return &AccessTokenService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[pcmodels.AccessToken](accessTokenCollection),
	}, nil
}

// IsNameExist kiểm tra tên access token có tồn tại hay không
func (s *AccessTokenService) IsNameExist(ctx context.Context, name string) (bool, error) {
	filter := bson.M{"name": name}
	var accessToken pcmodels.AccessToken
	err := s.Collection().FindOne(ctx, filter).Decode(&accessToken)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, common.ConvertMongoError(err)
	}
	return true, nil
}

// Create tạo mới một access token
func (s *AccessTokenService) Create(ctx context.Context, input *pcdto.AccessTokenCreateInput) (*pcmodels.AccessToken, error) {
	exists, err := s.IsNameExist(ctx, input.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrInvalidInput
	}

	assignedUsers := make([]primitive.ObjectID, 0)
	for _, userID := range input.AssignedUsers {
		assignedUsers = append(assignedUsers, utility.String2ObjectID(userID))
	}

	accessToken := &pcmodels.AccessToken{
		ID:            primitive.NewObjectID(),
		Name:          input.Name,
		Describe:      input.Describe,
		System:        input.System,
		Value:         input.Value,
		AssignedUsers: assignedUsers,
		Status:        0,
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}

	createdAccessToken, err := s.BaseServiceMongoImpl.InsertOne(ctx, *accessToken)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return &createdAccessToken, nil
}

// Update cập nhật thông tin access token
func (s *AccessTokenService) Update(ctx context.Context, id primitive.ObjectID, input *pcdto.AccessTokenUpdateInput) (*pcmodels.AccessToken, error) {
	accessToken, err := s.BaseServiceMongoImpl.FindOneById(ctx, id)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	set := make(map[string]interface{})

	if input.Name != "" && input.Name != accessToken.Name {
		exists, err := s.IsNameExist(ctx, input.Name)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, common.ErrInvalidInput
		}
		set["name"] = input.Name
	}
	if input.Describe != "" {
		set["describe"] = input.Describe
	}
	if input.System != "" {
		set["system"] = input.System
	}
	if input.Value != "" {
		set["value"] = input.Value
	}
	if len(input.AssignedUsers) > 0 {
		assignedUsers := make([]primitive.ObjectID, 0)
		for _, userID := range input.AssignedUsers {
			assignedUsers = append(assignedUsers, utility.String2ObjectID(userID))
		}
		set["assignedUsers"] = assignedUsers
	}

	if len(set) == 0 {
		return &accessToken, nil
	}

	updateData := &basesvc.UpdateData{Set: set}
	updatedAccessToken, err := s.BaseServiceMongoImpl.UpdateById(ctx, id, updateData)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &updatedAccessToken, nil
}
