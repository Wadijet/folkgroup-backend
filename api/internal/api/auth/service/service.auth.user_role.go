// Package authsvc - service vai trò người dùng (UserRole).
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

// UserRoleService là cấu trúc chứa các phương thức liên quan đến vai trò của người dùng
type UserRoleService struct {
	*basesvc.BaseServiceMongoImpl[models.UserRole]
	userService *UserService
	roleService *RoleService
}

// NewUserRoleService tạo mới UserRoleService
func NewUserRoleService() (*UserRoleService, error) {
	userRoleCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.UserRoles)
	if !exist {
		return nil, fmt.Errorf("failed to get user_roles collection: %v", common.ErrNotFound)
	}
	userService, err := NewUserService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user service: %v", err)
	}
	roleService, err := NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}
	return &UserRoleService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[models.UserRole](userRoleCollection),
		userService:          userService,
		roleService:          roleService,
	}, nil
}

// Create tạo mới một vai trò người dùng
func (s *UserRoleService) Create(ctx context.Context, input *authdto.UserRoleCreateInput) (*models.UserRole, error) {
	userObjID, err := primitive.ObjectIDFromHex(input.UserID)
	if err != nil {
		return nil, common.ErrInvalidInput
	}
	roleObjID, err := primitive.ObjectIDFromHex(input.RoleID)
	if err != nil {
		return nil, common.ErrInvalidInput
	}
	if _, err := s.userService.BaseServiceMongoImpl.FindOneById(ctx, userObjID); err != nil {
		return nil, common.ErrNotFound
	}
	if _, err := s.roleService.BaseServiceMongoImpl.FindOneById(ctx, roleObjID); err != nil {
		return nil, common.ErrNotFound
	}
	exists, err := s.IsExist(ctx, userObjID, roleObjID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrInvalidInput
	}
	userRole := &models.UserRole{
		ID:        primitive.NewObjectID(),
		UserID:    userObjID,
		RoleID:    roleObjID,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	createdUserRole, err := s.BaseServiceMongoImpl.InsertOne(ctx, *userRole)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return &createdUserRole, nil
}

// UpdateUserRoles cập nhật danh sách roles cho một user
func (s *UserRoleService) UpdateUserRoles(ctx context.Context, userID primitive.ObjectID, newRoleIDs []primitive.ObjectID) ([]models.UserRole, error) {
	if err := s.validateCanRemoveAdministratorRole(ctx, userID, newRoleIDs); err != nil {
		return nil, err
	}
	filter := bson.M{"userId": userID}
	if _, err := s.BaseServiceMongoImpl.DeleteMany(ctx, filter); err != nil {
		return nil, err
	}
	var userRoles []models.UserRole
	now := time.Now().Unix()
	for _, roleID := range newRoleIDs {
		userRole := models.UserRole{
			ID:        primitive.NewObjectID(),
			UserID:    userID,
			RoleID:    roleID,
			CreatedAt: now,
			UpdatedAt: now,
		}
		userRoles = append(userRoles, userRole)
	}
	if len(userRoles) > 0 {
		_, err := s.BaseServiceMongoImpl.InsertMany(ctx, userRoles)
		if err != nil {
			return nil, err
		}
	}
	return userRoles, nil
}

func (s *UserRoleService) validateCanRemoveAdministratorRole(ctx context.Context, userID primitive.ObjectID, newRoleIDs []primitive.ObjectID) error {
	adminRole, err := s.roleService.BaseServiceMongoImpl.FindOne(ctx, bson.M{"name": "Administrator"}, nil)
	if err != nil {
		return nil
	}
	var modelAdminRole models.Role
	bsonBytes, _ := bson.Marshal(adminRole)
	if err := bson.Unmarshal(bsonBytes, &modelAdminRole); err != nil {
		return nil
	}
	oldUserRoles, err := s.BaseServiceMongoImpl.Find(ctx, bson.M{"userId": userID, "roleId": modelAdminRole.ID}, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}
	hasAdminRoleInOldRoles := err == nil && len(oldUserRoles) > 0
	hasAdminRoleInNewRoles := false
	for _, roleID := range newRoleIDs {
		if roleID == modelAdminRole.ID {
			hasAdminRoleInNewRoles = true
			break
		}
	}
	if hasAdminRoleInOldRoles && !hasAdminRoleInNewRoles {
		allAdminUserRoles, err := s.BaseServiceMongoImpl.Find(ctx, bson.M{"roleId": modelAdminRole.ID}, nil)
		if err != nil && err != common.ErrNotFound {
			return err
		}
		if err == nil && len(allAdminUserRoles) <= 1 {
			return common.NewError(common.ErrCodeBusinessOperation, "Không thể xóa user khỏi role Administrator. Role Administrator phải có ít nhất một user.", common.StatusConflict, nil)
		}
	}
	return nil
}

// IsExist kiểm tra xem một UserRole đã tồn tại chưa
func (s *UserRoleService) IsExist(ctx context.Context, userID, roleID primitive.ObjectID) (bool, error) {
	filter := bson.M{"userId": userID, "roleId": roleID}
	count, err := s.BaseServiceMongoImpl.Collection().CountDocuments(ctx, filter)
	if err != nil {
		return false, common.ConvertMongoError(err)
	}
	return count > 0, nil
}

func (s *UserRoleService) validateBeforeDeleteAdministratorRole(ctx context.Context, userRoleID primitive.ObjectID) error {
	userRole, err := s.BaseServiceMongoImpl.FindOneById(ctx, userRoleID)
	if err != nil {
		return err
	}
	var modelUserRole models.UserRole
	bsonBytes, _ := bson.Marshal(userRole)
	if err := bson.Unmarshal(bsonBytes, &modelUserRole); err != nil {
		return common.ErrInvalidFormat
	}
	role, err := s.roleService.BaseServiceMongoImpl.FindOneById(ctx, modelUserRole.RoleID)
	if err != nil {
		return err
	}
	var modelRole models.Role
	bsonBytes, _ = bson.Marshal(role)
	if err := bson.Unmarshal(bsonBytes, &modelRole); err != nil {
		return common.ErrInvalidFormat
	}
	if modelRole.Name == "Administrator" {
		adminUserRoles, err := s.BaseServiceMongoImpl.Find(ctx, bson.M{"roleId": modelRole.ID}, nil)
		if err != nil && err != common.ErrNotFound {
			return err
		}
		if err == nil && len(adminUserRoles) <= 1 {
			return common.NewError(common.ErrCodeBusinessOperation, "Không thể xóa user khỏi role Administrator. Role Administrator phải có ít nhất một user.", common.StatusConflict, nil)
		}
	}
	return nil
}

func (s *UserRoleService) validateBeforeDeleteAdministratorRoleByFilter(ctx context.Context, filter bson.M) error {
	userRoles, err := s.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}
	adminRole, err := s.roleService.BaseServiceMongoImpl.FindOne(ctx, bson.M{"name": "Administrator"}, nil)
	if err != nil {
		return nil
	}
	var modelAdminRole models.Role
	bsonBytes, _ := bson.Marshal(adminRole)
	if err := bson.Unmarshal(bsonBytes, &modelAdminRole); err != nil {
		return nil
	}
	allAdminUserRoles, err := s.BaseServiceMongoImpl.Find(ctx, bson.M{"roleId": modelAdminRole.ID}, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}
	adminUserRolesToDelete := 0
	for _, userRole := range userRoles {
		var modelUserRole models.UserRole
		bsonBytes, _ := bson.Marshal(userRole)
		if err := bson.Unmarshal(bsonBytes, &modelUserRole); err != nil {
			continue
		}
		if modelUserRole.RoleID == modelAdminRole.ID {
			adminUserRolesToDelete++
		}
	}
	if err == nil && len(allAdminUserRoles) <= adminUserRolesToDelete {
		return common.NewError(common.ErrCodeBusinessOperation, "Không thể xóa user khỏi role Administrator. Role Administrator phải có ít nhất một user.", common.StatusConflict, nil)
	}
	return nil
}

// DeleteOne override
func (s *UserRoleService) DeleteOne(ctx context.Context, filter interface{}) error {
	var filterMap bson.M
	switch v := filter.(type) {
	case bson.M:
		filterMap = v
	case bson.D:
		filterMap = v.Map()
	default:
		filterBytes, _ := bson.Marshal(filter)
		var temp bson.M
		if err := bson.Unmarshal(filterBytes, &temp); err == nil {
			filterMap = temp
		} else {
			return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
		}
	}
	if err := s.validateBeforeDeleteAdministratorRoleByFilter(ctx, filterMap); err != nil {
		return err
	}
	return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
}

// DeleteById override
func (s *UserRoleService) DeleteById(ctx context.Context, id primitive.ObjectID) error {
	if err := s.validateBeforeDeleteAdministratorRole(ctx, id); err != nil {
		return err
	}
	return s.BaseServiceMongoImpl.DeleteById(ctx, id)
}

// DeleteMany override
func (s *UserRoleService) DeleteMany(ctx context.Context, filter interface{}) (int64, error) {
	var filterMap bson.M
	switch v := filter.(type) {
	case bson.M:
		filterMap = v
	case bson.D:
		filterMap = v.Map()
	default:
		filterBytes, _ := bson.Marshal(filter)
		var temp bson.M
		if err := bson.Unmarshal(filterBytes, &temp); err == nil {
			filterMap = temp
		} else {
			return s.BaseServiceMongoImpl.DeleteMany(ctx, filter)
		}
	}
	if err := s.validateBeforeDeleteAdministratorRoleByFilter(ctx, filterMap); err != nil {
		return 0, err
	}
	return s.BaseServiceMongoImpl.DeleteMany(ctx, filter)
}
