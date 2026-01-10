package services

import (
	"context"
	"fmt"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetUserAllowedOrganizationIDs lấy danh sách organization IDs mà user được phép truy cập
// Dựa trên permissions và scope của user
func GetUserAllowedOrganizationIDs(ctx context.Context, userID primitive.ObjectID, permissionName string) ([]primitive.ObjectID, error) {
	// Lấy các services cần thiết
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

	// 1. Lấy UserRoles của user
	userRoles, err := userRoleService.Find(ctx, bson.M{"userId": userID}, nil)
	if err != nil {
		return nil, err
	}

	allowedOrgIDsMap := make(map[primitive.ObjectID]bool)

	// 2. Duyệt qua từng role
	for _, userRole := range userRoles {
		// Lấy role
		role, err := roleService.FindOneById(ctx, userRole.RoleID)
		if err != nil {
			continue
		}

		orgID := role.OwnerOrganizationID

		// 3. Lấy RolePermissions của role
		rolePermissions, err := rolePermissionService.Find(ctx, bson.M{"roleId": role.ID}, nil)
		if err != nil {
			continue
		}

		// 4. Kiểm tra permission cụ thể
		for _, rp := range rolePermissions {
			permission, err := permissionService.FindOneById(ctx, rp.PermissionID)
			if err != nil {
				continue
			}

			// Chỉ xử lý nếu permission name khớp (hoặc permissionName rỗng = tất cả permissions)
			if permissionName != "" && permission.Name != permissionName {
				continue
			}

			// 5. Tính toán allowed organization IDs dựa trên scope
			if rp.Scope == 0 {
				// Scope 0: Chỉ organization của role
				allowedOrgIDsMap[orgID] = true
			} else if rp.Scope == 1 {
				// Scope 1: Organization + children
				allowedOrgIDsMap[orgID] = true

				// Lấy children IDs
				childrenIDs, err := organizationService.GetChildrenIDs(ctx, orgID)
				if err == nil {
					for _, childID := range childrenIDs {
						allowedOrgIDsMap[childID] = true
					}
				}
			}
		}
	}

	// 6. Convert map thành slice (KHÔNG tự động thêm parents)
	result := make([]primitive.ObjectID, 0, len(allowedOrgIDsMap))
	for orgID := range allowedOrgIDsMap {
		result = append(result, orgID)
	}

	return result, nil
}

// GetAllowedOrganizationIDsFromRole lấy danh sách organization IDs mà role được phép truy cập
// Dựa trên permission và scope của role
// Đơn giản hơn GetUserAllowedOrganizationIDs vì chỉ xử lý một role cụ thể
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

	// 1. Lấy role
	role, err := roleService.FindOneById(ctx, roleID)
	if err != nil {
		return nil, err
	}

	orgID := role.OwnerOrganizationID
	allowedOrgIDsMap := make(map[primitive.ObjectID]bool)

	// 2. Lấy RolePermissions của role
	rolePermissions, err := rolePermissionService.Find(ctx, bson.M{"roleId": roleID}, nil)
	if err != nil {
		return nil, err
	}

	// 3. Kiểm tra permission cụ thể
	for _, rp := range rolePermissions {
		permission, err := permissionService.FindOneById(ctx, rp.PermissionID)
		if err != nil {
			continue
		}

		// Chỉ xử lý nếu permission name khớp (hoặc permissionName rỗng = tất cả permissions)
		if permissionName != "" && permission.Name != permissionName {
			continue
		}

		// 4. Tính toán allowed organization IDs dựa trên scope
		if rp.Scope == 0 {
			// Scope 0: Chỉ organization của role
			allowedOrgIDsMap[orgID] = true
		} else if rp.Scope == 1 {
			// Scope 1: Organization + children
			allowedOrgIDsMap[orgID] = true

			// Lấy children IDs
			childrenIDs, err := organizationService.GetChildrenIDs(ctx, orgID)
			if err == nil {
				for _, childID := range childrenIDs {
					allowedOrgIDsMap[childID] = true
				}
			}
		}
	}

	// 5. Convert map thành slice
	result := make([]primitive.ObjectID, 0, len(allowedOrgIDsMap))
	for orgID := range allowedOrgIDsMap {
		result = append(result, orgID)
	}

	return result, nil
}

// IsUserAdministrator kiểm tra xem user có phải Administrator không
// Returns:
//   - bool: true nếu user là Administrator
//   - error: Lỗi nếu có
func IsUserAdministrator(ctx context.Context, userID primitive.ObjectID) (bool, error) {
	userRoleService, err := NewUserRoleService()
	if err != nil {
		return false, fmt.Errorf("failed to create user role service: %v", err)
	}

	roleService, err := NewRoleService()
	if err != nil {
		return false, fmt.Errorf("failed to create role service: %v", err)
	}

	// Lấy role Administrator
	adminRole, err := roleService.FindOne(ctx, bson.M{"name": "Administrator"}, nil)
	if err != nil {
		if err == common.ErrNotFound {
			return false, nil // Chưa có role Administrator
		}
		return false, err
	}

	var modelRole models.Role
	bsonBytes, _ := bson.Marshal(adminRole)
	if err := bson.Unmarshal(bsonBytes, &modelRole); err != nil {
		return false, err
	}

	// Kiểm tra user có role Administrator không
	_, err = userRoleService.FindOne(ctx, bson.M{
		"userId": userID,
		"roleId": modelRole.ID,
	}, nil)

	if err != nil {
		if err == common.ErrNotFound {
			return false, nil // User không có role Administrator
		}
		return false, err
	}

	return true, nil // User có role Administrator
}

// Context key type để tránh conflict
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
		return false, nil // Không có userID trong context, không phải admin
	}
	return IsUserAdministrator(ctx, userID)
}
