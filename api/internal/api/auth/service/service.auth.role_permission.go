// Package authsvc - service quyền của vai trò (RolePermission).
package authsvc

import (
	"context"
	"fmt"
	"time"

	authdto "meta_commerce/internal/api/auth/dto"
	models "meta_commerce/internal/api/auth/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RolePermissionService là cấu trúc chứa các phương thức liên quan đến quyền của vai trò
type RolePermissionService struct {
	*basesvc.BaseServiceMongoImpl[models.RolePermission]
	roleService       *RoleService
	permissionService *PermissionService
}

// NewRolePermissionService tạo mới RolePermissionService
func NewRolePermissionService() (*RolePermissionService, error) {
	rolePermissionCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.RolePermissions)
	if !exist {
		return nil, fmt.Errorf("failed to get role_permissions collection: %v", common.ErrNotFound)
	}

	roleService, err := NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}

	permissionService, err := NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}

	return &RolePermissionService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.RolePermission](rolePermissionCollection),
		roleService:          roleService,
		permissionService:    permissionService,
	}, nil
}

// Create tạo mới một quyền cho vai trò
func (s *RolePermissionService) Create(ctx context.Context, input *authdto.RolePermissionCreateInput) (*models.RolePermission, error) {
	roleObjID, err := primitive.ObjectIDFromHex(input.RoleID)
	if err != nil {
		return nil, common.ErrInvalidInput
	}
	permissionObjID, err := primitive.ObjectIDFromHex(input.PermissionID)
	if err != nil {
		return nil, common.ErrInvalidInput
	}

	if _, err := s.roleService.BaseServiceMongoImpl.FindOneById(ctx, roleObjID); err != nil {
		return nil, common.ErrNotFound
	}
	if _, err := s.permissionService.BaseServiceMongoImpl.FindOneById(ctx, permissionObjID); err != nil {
		return nil, common.ErrNotFound
	}

	exists, err := s.IsExist(ctx, roleObjID, permissionObjID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrInvalidInput
	}

	rolePermission := &models.RolePermission{
		ID:           primitive.NewObjectID(),
		RoleID:       roleObjID,
		PermissionID: permissionObjID,
		Scope:        input.Scope,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}

	createdRolePermission, err := s.BaseServiceMongoImpl.InsertOne(ctx, *rolePermission)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &createdRolePermission, nil
}

// IsExist kiểm tra xem một RolePermission đã tồn tại chưa
func (s *RolePermissionService) IsExist(ctx context.Context, roleID, permissionID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"roleId":       roleID,
		"permissionId": permissionID,
	}
	count, err := s.BaseServiceMongoImpl.Collection().CountDocuments(ctx, filter)
	if err != nil {
		return false, common.ConvertMongoError(err)
	}
	return count > 0, nil
}
