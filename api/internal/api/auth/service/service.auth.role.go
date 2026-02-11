// Package authsvc - service vai trò (Role).
package authsvc

import (
	"context"
	"fmt"
	models "meta_commerce/internal/api/auth/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// RoleService là cấu trúc chứa các phương thức liên quan đến vai trò
type RoleService struct {
	*basesvc.BaseServiceMongoImpl[models.Role]
}

// NewRoleService tạo mới RoleService
func NewRoleService() (*RoleService, error) {
	roleCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Roles)
	if !exist {
		return nil, fmt.Errorf("failed to get roles collection: %v", common.ErrNotFound)
	}

	return &RoleService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.Role](roleCollection),
	}, nil
}

// validateBeforeDelete kiểm tra các điều kiện trước khi xóa role
func (s *RoleService) validateBeforeDelete(ctx context.Context, roleID primitive.ObjectID) error {
	role, err := s.BaseServiceMongoImpl.FindOneById(ctx, roleID)
	if err != nil {
		return err
	}

	var modelRole models.Role
	bsonBytes, _ := bson.Marshal(role)
	err = bson.Unmarshal(bsonBytes, &modelRole)
	if err != nil {
		return common.ErrInvalidFormat
	}

	if modelRole.Name == "Administrator" {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			"Không thể xóa chức danh Administrator. Đây là chức danh hệ thống và không thể xóa.",
			common.StatusForbidden,
			nil,
		)
	}

	return nil
}

// DeleteOne override để kiểm tra trước khi xóa
func (s *RoleService) DeleteOne(ctx context.Context, filter interface{}) error {
	role, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return err
	}

	var modelRole models.Role
	bsonBytes, _ := bson.Marshal(role)
	err = bson.Unmarshal(bsonBytes, &modelRole)
	if err != nil {
		return common.ErrInvalidFormat
	}

	if err := s.validateBeforeDelete(ctx, modelRole.ID); err != nil {
		return err
	}
	return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
}

// DeleteById override để kiểm tra trước khi xóa
func (s *RoleService) DeleteById(ctx context.Context, id primitive.ObjectID) error {
	if err := s.validateBeforeDelete(ctx, id); err != nil {
		return err
	}
	return s.BaseServiceMongoImpl.DeleteById(ctx, id)
}

// DeleteMany override để kiểm tra trước khi xóa
func (s *RoleService) DeleteMany(ctx context.Context, filter interface{}) (int64, error) {
	roles, err := s.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil && err != common.ErrNotFound {
		return 0, err
	}

	for _, role := range roles {
		var modelRole models.Role
		bsonBytes, _ := bson.Marshal(role)
		if err := bson.Unmarshal(bsonBytes, &modelRole); err != nil {
			continue
		}
		if err := s.validateBeforeDelete(ctx, modelRole.ID); err != nil {
			return 0, err
		}
	}
	return s.BaseServiceMongoImpl.DeleteMany(ctx, filter)
}

// FindOneAndDelete override để kiểm tra trước khi xóa
func (s *RoleService) FindOneAndDelete(ctx context.Context, filter interface{}, opts *mongoopts.FindOneAndDeleteOptions) (models.Role, error) {
	var zero models.Role

	role, err := s.BaseServiceMongoImpl.FindOne(ctx, filter, nil)
	if err != nil {
		return zero, err
	}

	var modelRole models.Role
	bsonBytes, _ := bson.Marshal(role)
	err = bson.Unmarshal(bsonBytes, &modelRole)
	if err != nil {
		return zero, common.ErrInvalidFormat
	}

	if err := s.validateBeforeDelete(ctx, modelRole.ID); err != nil {
		return zero, err
	}
	return s.BaseServiceMongoImpl.FindOneAndDelete(ctx, filter, opts)
}
