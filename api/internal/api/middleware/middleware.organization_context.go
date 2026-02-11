package middleware

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	authsvc "meta_commerce/internal/api/auth/service"
	"meta_commerce/internal/common"
)

// OrganizationContextMiddleware middleware để quản lý organization context
// QUAN TRỌNG: Context làm việc là ROLE, không phải organization
// - Đọc X-Active-Role-ID (ROLE ID) từ header
// - Validate user có role này không
// - Từ role, tự động suy ra organization ID tương ứng
// - Lưu active_role_id (PRIMARY) và active_organization_id (DERIVED) vào context
func OrganizationContextMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Lấy user ID từ context (đã được set bởi AuthMiddleware)
		userIDStr, ok := c.Locals("user_id").(string)
		if !ok || userIDStr == "" {
			// Không có user ID, có thể là route không cần auth
			// Cho phép tiếp tục nhưng không set organization context
			return c.Next()
		}

		userID, err := primitive.ObjectIDFromHex(userIDStr)
		if err != nil {
			// User ID không hợp lệ
			return c.Next()
		}

		// Lấy active role ID từ header
		activeRoleIDStr := c.Get("X-Active-Role-ID")
		var activeRoleID primitive.ObjectID

		if activeRoleIDStr != "" {
			// Có header, validate role ID
			activeRoleID, err = primitive.ObjectIDFromHex(activeRoleIDStr)
			if err != nil {
				// Role ID không hợp lệ, fallback về role đầu tiên
				activeRoleID, err = getFirstUserRoleID(context.Background(), userID)
				if err != nil {
					return c.Next() // Không có role, cho phép tiếp tục
				}
			} else {
				// Validate user có role này không
				hasRole, err := validateUserHasRole(context.Background(), userID, activeRoleID)
				if err != nil || !hasRole {
					// User không có role này, fallback về role đầu tiên
					activeRoleID, err = getFirstUserRoleID(context.Background(), userID)
					if err != nil {
						return c.Next() // Không có role, cho phép tiếp tục
					}
				}
			}
		} else {
			// Không có header, lấy role đầu tiên của user
			activeRoleID, err = getFirstUserRoleID(context.Background(), userID)
			if err != nil {
				return c.Next() // Không có role, cho phép tiếp tục
			}
		}

		// Lấy role để suy ra organization ID
		// Context làm việc là ROLE, organization được tự động suy ra từ role
		roleService, err := authsvc.NewRoleService()
		if err != nil {
			return c.Next()
		}

		role, err := roleService.BaseServiceMongoImpl.FindOneById(context.Background(), activeRoleID)
		if err != nil {
			return c.Next()
		}

		// Lưu vào context
		// active_role_id: PRIMARY - đây là context làm việc
		// active_organization_id: DERIVED - được suy ra từ role
		c.Locals("active_role_id", activeRoleID.Hex())
		// Dùng OwnerOrganizationID trực tiếp (đã bỏ OrganizationID)
		orgID := role.OwnerOrganizationID
		c.Locals("active_organization_id", orgID.Hex())

		return c.Next()
	}
}

// validateUserHasRole kiểm tra user có role này không
func validateUserHasRole(ctx context.Context, userID, roleID primitive.ObjectID) (bool, error) {
	userRoleService, err := authsvc.NewUserRoleService()
	if err != nil {
		return false, err
	}

	userRoles, err := userRoleService.BaseServiceMongoImpl.Find(ctx, bson.M{
		"userId": userID,
		"roleId": roleID,
	}, nil)

	if err != nil {
		return false, err
	}

	return len(userRoles) > 0, nil
}

// getFirstUserRoleID lấy role ID đầu tiên của user
func getFirstUserRoleID(ctx context.Context, userID primitive.ObjectID) (primitive.ObjectID, error) {
	userRoleService, err := authsvc.NewUserRoleService()
	if err != nil {
		return primitive.NilObjectID, err
	}

	userRoles, err := userRoleService.BaseServiceMongoImpl.Find(ctx, bson.M{"userId": userID}, nil)
	if err != nil {
		return primitive.NilObjectID, err
	}

	if len(userRoles) == 0 {
		return primitive.NilObjectID, common.ErrNotFound
	}

	return userRoles[0].RoleID, nil
}
