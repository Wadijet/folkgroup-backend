// Package authsvc - service quản trị (Admin): block user, set role, v.v.
package authsvc

import (
	"context"
	"fmt"

	models "meta_commerce/internal/api/auth/models"
	basesvc "meta_commerce/internal/api/base/service"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdminService là cấu trúc chứa các phương thức liên quan đến admin
type AdminService struct {
	userService           *UserService
	roleService           *RoleService
	permissionService     *PermissionService
	userRoleService       *UserRoleService
	rolePermissionService *RolePermissionService
}

// NewAdminService tạo mới AdminService
func NewAdminService() (*AdminService, error) {
	userService, err := NewUserService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user service: %w", err)
	}

	roleService, err := NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %w", err)
	}

	permissionService, err := NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %w", err)
	}

	userRoleService, err := NewUserRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user_role service: %w", err)
	}

	rolePermissionService, err := NewRolePermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role_permission service: %w", err)
	}

	return &AdminService{
		userService:           userService,
		roleService:           roleService,
		permissionService:     permissionService,
		userRoleService:       userRoleService,
		rolePermissionService: rolePermissionService,
	}, nil
}

// SetRole gán Role cho User dựa trên Email và RoleID
func (s *AdminService) SetRole(ctx context.Context, email string, roleID primitive.ObjectID) (*models.User, error) {
	_, err := s.roleService.FindOneById(ctx, roleID)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"email": email}
	user, err := s.userService.FindOne(ctx, filter, nil)
	if err != nil {
		if err == common.ErrNotFound {
			return nil, err
		}
		return nil, err
	}

	updateData := &basesvc.UpdateData{
		Set: map[string]interface{}{"token": roleID.Hex()},
	}
	updatedUser, err := s.userService.UpdateById(ctx, user.ID, updateData)
	if err != nil {
		return nil, err
	}
	return &updatedUser, nil
}

// BlockUser chặn hoặc bỏ chặn User dựa trên Email và trạng thái Block
func (s *AdminService) BlockUser(ctx context.Context, email string, block bool, note string) (*models.User, error) {
	filter := bson.M{"email": email}
	user, err := s.userService.FindOne(ctx, filter, nil)
	if err != nil {
		return nil, err
	}

	updateData := &basesvc.UpdateData{
		Set: map[string]interface{}{
			"isBlock":   block,
			"blockNote": note,
		},
	}
	updatedUser, err := s.userService.UpdateById(ctx, user.ID, updateData)
	if err != nil {
		return nil, err
	}
	return &updatedUser, nil
}

// UnBlockUser mở khóa người dùng
func (s *AdminService) UnBlockUser(ctx context.Context, email string) (*models.User, error) {
	return s.BlockUser(ctx, email, false, "")
}
