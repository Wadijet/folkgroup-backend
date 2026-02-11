// Package authsvc - helper organization (allowed orgs, admin check, context).
package authsvc

import (
	"context"
	"fmt"
	models "meta_commerce/internal/api/auth/models"
	"meta_commerce/internal/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetUserAllowedOrganizationIDs lấy danh sách organization IDs mà user được phép truy cập
func GetUserAllowedOrganizationIDs(ctx context.Context, userID primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
	userRoleService, err := NewUserRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user role service: %v", err)
	}
	roleService, err := NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}
	rolePermissionService, err := NewRolePermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role permission service: %v", err)
	}
	permissionService, err := NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}
	organizationService, err := NewOrganizationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization service: %v", err)
	}

	userRoles, err := userRoleService.BaseServiceMongoImpl.Find(ctx, bson.M{"userId": userID}, nil)
	if err != nil {
		return nil, err
	}
	allowedOrgIDsMap := make(map[primitive.ObjectID]bool)

	for _, userRole := range userRoles {
		role, err := roleService.BaseServiceMongoImpl.FindOneById(ctx, userRole.RoleID)
		if err != nil {
			continue
		}
		orgID := role.OwnerOrganizationID

		rolePermissions, err := rolePermissionService.BaseServiceMongoImpl.Find(ctx, bson.M{"roleId": role.ID}, nil)
		if err != nil {
			continue
		}
		for _, rp := range rolePermissions {
			permission, err := permissionService.BaseServiceMongoImpl.FindOneById(ctx, rp.PermissionID)
			if err != nil {
				continue
			}
			if permissionName != "" && permission.Name != permissionName {
				continue
			}
			if rp.Scope == 0 {
				allowedOrgIDsMap[orgID] = true
			} else if rp.Scope == 1 {
				allowedOrgIDsMap[orgID] = true
				childrenIDs, err := organizationService.GetChildrenIDs(ctx, orgID)
				if err == nil {
					for _, childID := range childrenIDs {
						allowedOrgIDsMap[childID] = true
					}
				}
			}
		}
	}

	result := make([]primitive.ObjectID, 0, len(allowedOrgIDsMap))
	for orgID := range allowedOrgIDsMap {
		result = append(result, orgID)
	}
	return result, nil
}

// GetAllowedOrganizationIDsFromRole lấy danh sách organization IDs mà role được phép truy cập
func GetAllowedOrganizationIDsFromRole(ctx context.Context, roleID primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
	roleService, err := NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}
	rolePermissionService, err := NewRolePermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role permission service: %v", err)
	}
	permissionService, err := NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}
	organizationService, err := NewOrganizationService()
	if err != nil {
		return nil, fmt.Errorf("failed to create organization service: %v", err)
	}

	role, err := roleService.BaseServiceMongoImpl.FindOneById(ctx, roleID)
	if err != nil {
		return nil, err
	}
	orgID := role.OwnerOrganizationID
	allowedOrgIDsMap := make(map[primitive.ObjectID]bool)

	rolePermissions, err := rolePermissionService.BaseServiceMongoImpl.Find(ctx, bson.M{"roleId": roleID}, nil)
	if err != nil {
		return nil, err
	}
	for _, rp := range rolePermissions {
		permission, err := permissionService.BaseServiceMongoImpl.FindOneById(ctx, rp.PermissionID)
		if err != nil {
			continue
		}
		if permissionName != "" && permission.Name != permissionName {
			continue
		}
		if rp.Scope == 0 {
			allowedOrgIDsMap[orgID] = true
		} else if rp.Scope == 1 {
			allowedOrgIDsMap[orgID] = true
			childrenIDs, err := organizationService.GetChildrenIDs(ctx, orgID)
			if err == nil {
				for _, childID := range childrenIDs {
					allowedOrgIDsMap[childID] = true
				}
			}
		}
	}
	result := make([]primitive.ObjectID, 0, len(allowedOrgIDsMap))
	for oid := range allowedOrgIDsMap {
		result = append(result, oid)
	}
	return result, nil
}

// IsUserAdministrator kiểm tra xem user có phải Administrator không
func IsUserAdministrator(ctx context.Context, userID primitive.ObjectID) (bool, error) {
	userRoleService, err := NewUserRoleService()
	if err != nil {
		return false, fmt.Errorf("failed to create user role service: %v", err)
	}
	roleService, err := NewRoleService()
	if err != nil {
		return false, fmt.Errorf("failed to create role service: %v", err)
	}
	adminRole, err := roleService.BaseServiceMongoImpl.FindOne(ctx, bson.M{"name": "Administrator"}, nil)
	if err != nil {
		if err == common.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	var modelRole models.Role
	bsonBytes, _ := bson.Marshal(adminRole)
	if err := bson.Unmarshal(bsonBytes, &modelRole); err != nil {
		return false, err
	}
	_, err = userRoleService.BaseServiceMongoImpl.FindOne(ctx, bson.M{"userId": userID, "roleId": modelRole.ID}, nil)
	if err != nil {
		if err == common.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type contextKey string

const userIDContextKey contextKey = "user_id"

// SetUserIDToContext lưu userID vào context
func SetUserIDToContext(ctx context.Context, userID primitive.ObjectID) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// GetUserIDFromContext lấy userID từ context
func GetUserIDFromContext(ctx context.Context) (primitive.ObjectID, bool) {
	userID, ok := ctx.Value(userIDContextKey).(primitive.ObjectID)
	return userID, ok
}

// IsUserAdministratorFromContext kiểm tra user trong context có phải Administrator không
func IsUserAdministratorFromContext(ctx context.Context) (bool, error) {
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return false, nil
	}
	return IsUserAdministrator(ctx, userID)
}
