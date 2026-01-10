package middleware

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"meta_commerce/core/logger"
	"meta_commerce/core/utility"
)

// AuthManager quản lý xác thực và phân quyền người dùng
type AuthManager struct {
	UserCRUD           *services.UserService
	RoleCRUD           *services.RoleService
	PermissionCRUD     *services.PermissionService
	RolePermissionCRUD *services.RolePermissionService
	UserRoleCRUD       *services.UserRoleService
	Cache              *utility.Cache
}

var (
	authManagerInstance *AuthManager
	authManagerOnce     sync.Once
)

// GetAuthManager trả về instance duy nhất của AuthManager (singleton pattern)
func GetAuthManager() *AuthManager {
	authManagerOnce.Do(func() {
		var err error
		authManagerInstance, err = newAuthManager()
		if err != nil {
			panic(err)
		}
	})
	return authManagerInstance
}

// newAuthManager khởi tạo một instance mới của AuthManager (private constructor)
func newAuthManager() (*AuthManager, error) {
	newManager := new(AuthManager)

	// Khởi tạo các service với BaseService để thực hiện các thao tác CRUD
	userService, err := services.NewUserService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user service: %v", err)
	}
	newManager.UserCRUD = userService

	roleService, err := services.NewRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role service: %v", err)
	}
	newManager.RoleCRUD = roleService

	permissionService, err := services.NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}
	newManager.PermissionCRUD = permissionService

	rolePermissionService, err := services.NewRolePermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role permission service: %v", err)
	}
	newManager.RolePermissionCRUD = rolePermissionService

	userRoleService, err := services.NewUserRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user role service: %v", err)
	}
	newManager.UserRoleCRUD = userRoleService

	// Khởi tạo cache với thời gian sống 5 phút và thời gian dọn dẹp 10 phút
	newManager.Cache = utility.NewCache(5*time.Minute, 10*time.Minute)

	return newManager, nil
}

// getUserPermissions lấy danh sách permissions của user từ cache hoặc database
// Nếu activeRoleID được cung cấp, chỉ lấy permissions từ role đó (role context)
// Nếu activeRoleID là nil, lấy permissions từ tất cả roles của user (backward compatibility)
func (am *AuthManager) getUserPermissions(userID string, activeRoleID *primitive.ObjectID) (map[string]byte, error) {
	// Tạo cache key dựa trên userID và activeRoleID (nếu có)
	var cacheKey string
	if activeRoleID != nil {
		cacheKey = fmt.Sprintf("user_permissions:%s:role:%s", userID, activeRoleID.Hex())
	} else {
		cacheKey = "user_permissions:" + userID
	}

	// Kiểm tra cache trước để tối ưu hiệu suất
	if cached, found := am.Cache.Get(cacheKey); found {
		return cached.(map[string]byte), nil
	}

	// Nếu không có trong cache, lấy từ database
	permissions := make(map[string]byte)

	// Nếu có activeRoleID, chỉ lấy permissions từ role đó
	if activeRoleID != nil {
		// Validate user có role này không
		_, err := am.UserRoleCRUD.FindOne(context.TODO(), bson.M{
			"userId": utility.String2ObjectID(userID),
			"roleId": *activeRoleID,
		}, nil)
		if err != nil {
			// User không có role này, trả về map rỗng
			am.Cache.Set(cacheKey, permissions)
			return permissions, nil
		}

		// Lấy danh sách permissions của role
		findRolePermissions, err := am.RolePermissionCRUD.Find(context.TODO(), bson.M{"roleId": *activeRoleID}, nil)
		if err != nil {
			am.Cache.Set(cacheKey, permissions)
			return permissions, nil
		}

		// Lấy thông tin chi tiết của từng permission
		for _, rolePermission := range findRolePermissions {
			permission, err := am.PermissionCRUD.FindOneById(context.TODO(), rolePermission.PermissionID)
			if err != nil {
				continue
			}
			permissions[permission.Name] = rolePermission.Scope
		}
	} else {
		// Lấy permissions từ tất cả roles của user (backward compatibility)
		findRoles, err := am.UserRoleCRUD.Find(context.TODO(), bson.M{"userId": utility.String2ObjectID(userID)}, nil)
		if err != nil {
			return nil, common.ConvertMongoError(err)
		}

		// Duyệt qua từng vai trò để lấy permissions
		for _, userRole := range findRoles {
			// Lấy danh sách permissions của vai trò
			findRolePermissions, err := am.RolePermissionCRUD.Find(context.TODO(), bson.M{"roleId": userRole.RoleID}, nil)
			if err != nil {
				continue
			}

			// Lấy thông tin chi tiết của từng permission
			for _, rolePermission := range findRolePermissions {
				permission, err := am.PermissionCRUD.FindOneById(context.TODO(), rolePermission.PermissionID)
				if err != nil {
					continue
				}
				permissions[permission.Name] = rolePermission.Scope
			}
		}
	}

	// Lưu vào cache để sử dụng cho các lần sau
	am.Cache.Set(cacheKey, permissions)
	return permissions, nil
}

// AuthMiddleware middleware xác thực cho Fiber
func AuthMiddleware(requirePermission string) fiber.Handler {
	// Sử dụng singleton instance của AuthManager
	authManager := GetAuthManager()

	return func(c fiber.Ctx) error {
		// Lấy token từ header
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			// Chỉ log khi thiếu token (lỗi quan trọng)
			logger.GetAppLogger().WithFields(logrus.Fields{
				"path":   c.Path(),
				"method": c.Method(),
			}).Warn("❌ [AUTH] Missing Authorization header")
			HandleErrorResponse(c, common.ErrTokenMissing)
			return nil
		}

		// Kiểm tra định dạng token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			HandleErrorResponse(c, common.ErrTokenInvalid)
			return nil
		}

		token := parts[1]

		// Tìm user có token
		// Ưu tiên query field "token" (token mới nhất) trước vì nó được cập nhật mỗi lần login
		// Nếu không tìm thấy, query trong array "tokens" (tokens theo hwid)
		var user models.User
		var err error
		var query bson.M

		// Cách 1: Query field "token" (token mới nhất) - ĐÂY LÀ CÁCH CHÍNH
		query = bson.M{"token": token}
		user, err = authManager.UserCRUD.FindOne(context.Background(), query, nil)

		if err != nil {

			// Cách 2: Query trong array "tokens" với dot notation
			query = bson.M{"tokens.jwtToken": token}
			user, err = authManager.UserCRUD.FindOne(context.Background(), query, nil)

			if err != nil {
				// Cách 3: Query với $elemMatch
				query = bson.M{
					"tokens": bson.M{
						"$elemMatch": bson.M{
							"jwtToken": token,
						},
					},
				}
				user, err = authManager.UserCRUD.FindOne(context.Background(), query, nil)
			}
		}

		if err != nil {
			// Chỉ log khi không tìm thấy token (lỗi quan trọng)
			logger.GetAppLogger().WithFields(logrus.Fields{
				"path":  c.Path(),
				"error": err.Error(),
			}).Warn("❌ [AUTH] Token not found in database")
			HandleErrorResponse(c, common.ErrTokenInvalid)
			return nil
		}

		// Kiểm tra user có bị block không
		if user.IsBlock {
			HandleErrorResponse(c, common.NewError(
				common.ErrCodeAuthCredentials,
				"Tài khoản đã bị khóa: "+user.BlockNote,
				common.StatusForbidden,
				nil,
			))
			return nil
		}

		// Lưu thông tin user vào context
		c.Locals("user_id", user.ID.Hex())
		c.Locals("user", user)

		// Nếu không yêu cầu permission cụ thể, cho phép truy cập NGAY
		// Đây là endpoint đặc biệt như /auth/roles - chỉ cần xác thực, không cần permission
		if requirePermission == "" {
			return c.Next()
		}

		// Lấy active role ID từ header (role context)
		// Logic: Nếu route có require permission, PHẢI có header X-Active-Role-ID để chỉ định role context
		activeRoleIDStr := c.Get("X-Active-Role-ID")

		// Header X-Active-Role-ID là BẮT BUỘC khi route yêu cầu permission
		if activeRoleIDStr == "" {
			logger.GetAppLogger().WithFields(logrus.Fields{
				"user_id":    user.ID.Hex(),
				"user_email": user.Email,
				"path":       c.Path(),
				"permission": requirePermission,
			}).Warn("❌ [AUTH] Missing X-Active-Role-ID header")
			HandleErrorResponse(c, common.NewError(
				common.ErrCodeAuthRole,
				"Thiếu header X-Active-Role-ID. Vui lòng chọn role để làm việc.",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Parse và validate role ID
		roleID, err := primitive.ObjectIDFromHex(activeRoleIDStr)
		if err != nil {
			logger.GetAppLogger().WithFields(logrus.Fields{
				"user_id":        user.ID.Hex(),
				"active_role_id": activeRoleIDStr,
				"path":           c.Path(),
				"error":          err.Error(),
			}).Warn("❌ [AUTH] Invalid X-Active-Role-ID format")
			HandleErrorResponse(c, common.NewError(
				common.ErrCodeValidationFormat,
				"X-Active-Role-ID không đúng định dạng",
				common.StatusBadRequest,
				nil,
			))
			return nil
		}

		// Lấy danh sách roles của user để kiểm tra
		userRoles, err := authManager.UserRoleCRUD.Find(context.Background(), bson.M{"userId": utility.String2ObjectID(user.ID.Hex())}, nil)
		if err != nil {
			logger.GetAppLogger().WithFields(logrus.Fields{
				"user_id": user.ID.Hex(),
				"error":   err.Error(),
				"path":    c.Path(),
			}).Error("❌ [AUTH] Failed to get user roles")
			HandleErrorResponse(c, common.NewError(
				common.ErrCodeAuthRole,
				"Không thể kiểm tra quyền truy cập",
				common.StatusForbidden,
				nil,
			))
			return nil
		}

		// Nếu user không có role nào, từ chối truy cập ngay
		if len(userRoles) == 0 {
			logger.GetAppLogger().WithFields(logrus.Fields{
				"user_id":    user.ID.Hex(),
				"user_email": user.Email,
				"path":       c.Path(),
				"permission": requirePermission,
			}).Warn("❌ [AUTH] User has no roles, denying access")
			HandleErrorResponse(c, common.NewError(
				common.ErrCodeAuthRole,
				"Người dùng chưa được gán vai trò. Vui lòng liên hệ quản trị viên để được cấp quyền truy cập.",
				common.StatusForbidden,
				nil,
			))
			return nil
		}

		// Validate user có role này không
		hasRole := false
		for _, userRole := range userRoles {
			// So sánh ObjectID - dùng .Hex() để đảm bảo so sánh đúng
			if userRole.RoleID.Hex() == roleID.Hex() {
				hasRole = true
				break
			}
		}

		// Nếu user không có role này, reject request và trả về role IDs hợp lệ (an toàn hơn fallback)
		if !hasRole {
			// Lấy danh sách role IDs hợp lệ để trả về trong error response
			validRoleIDs := make([]string, 0, len(userRoles))
			for _, userRole := range userRoles {
				validRoleIDs = append(validRoleIDs, userRole.RoleID.Hex())
			}
			
			logger.GetAppLogger().WithFields(logrus.Fields{
				"user_id":        user.ID.Hex(),
				"active_role_id": roleID.Hex(),
				"valid_role_ids": validRoleIDs,
				"path":           c.Path(),
			}).Warn("⚠️ [AUTH] User does not have this role, rejecting request")
			
			// Reject với error code đặc biệt và trả về role IDs hợp lệ
			// Frontend có thể catch error này và tự động refresh role list
			HandleErrorResponse(c, common.NewError(
				common.ErrCodeAuthRole,
				"Người dùng không có quyền sử dụng role này. Vui lòng chọn role khác hoặc liên hệ quản trị viên.",
				common.StatusForbidden,
				map[string]interface{}{
					"invalidRoleId": roleID.Hex(),
					"validRoleIds":  validRoleIDs,
					"errorCode":     "ROLE_CONTEXT_INVALID",
				},
			))
			return nil
		}

		activeRoleID := &roleID

		// Kiểm tra permission của user trong role context (active role)
		permissions, err := authManager.getUserPermissions(user.ID.Hex(), activeRoleID)
		if err != nil {
			HandleErrorResponse(c, common.NewError(
				common.ErrCodeAuthRole,
				"Không thể lấy thông tin quyền",
				common.StatusForbidden,
				nil,
			))
			return nil
		}

		// Kiểm tra user có permission cần thiết trong role context không
		scope, hasPermission := permissions[requirePermission]
		if !hasPermission {
			logger.GetAppLogger().WithFields(logrus.Fields{
				"user_id":             user.ID.Hex(),
				"user_email":          user.Email,
				"active_role_id":      activeRoleID.Hex(),
				"required_permission": requirePermission,
				"path":                c.Path(),
			}).Warn("❌ [AUTH] User does not have required permission")
			HandleErrorResponse(c, common.NewError(
				common.ErrCodeAuthRole,
				"Không có quyền truy cập. Vui lòng kiểm tra lại role context hoặc liên hệ quản trị viên.",
				common.StatusForbidden,
				nil,
			))
			return nil
		}

		// Lưu scope tối thiểu và permission name vào context để sử dụng trong handler
		c.Locals("minScope", scope)
		c.Locals("permission_name", requirePermission) // Lưu permission name để handler sử dụng
		return c.Next()
	}
}
