package services

import (
	"context"
	"fmt"
	"time"

	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserRoleService là cấu trúc chứa các phương thức liên quan đến vai trò của người dùng
type UserRoleService struct {
	*BaseServiceMongoImpl[models.UserRole]
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
		BaseServiceMongoImpl: NewBaseServiceMongo[models.UserRole](userRoleCollection),
		userService:          userService,
		roleService:          roleService,
	}, nil
}

// Create tạo mới một vai trò người dùng
func (s *UserRoleService) Create(ctx context.Context, input *dto.UserRoleCreateInput) (*models.UserRole, error) {
	userObjID, err := primitive.ObjectIDFromHex(input.UserID)
	if err != nil {
		return nil, common.ErrInvalidInput
	}
	roleObjID, err := primitive.ObjectIDFromHex(input.RoleID)
	if err != nil {
		return nil, common.ErrInvalidInput
	}

	// Kiểm tra User có tồn tại không
	if _, err := s.userService.FindOneById(ctx, userObjID); err != nil {
		return nil, common.ErrNotFound
	}

	// Kiểm tra Role có tồn tại không
	if _, err := s.roleService.FindOneById(ctx, roleObjID); err != nil {
		return nil, common.ErrNotFound
	}

	// Kiểm tra UserRole đã tồn tại chưa
	exists, err := s.IsExist(ctx, userObjID, roleObjID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, common.ErrInvalidInput
	}

	// Tạo userRole mới
	userRole := &models.UserRole{
		ID:        primitive.NewObjectID(),
		UserID:    userObjID,
		RoleID:    roleObjID,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// Lưu userRole
	createdUserRole, err := s.BaseServiceMongoImpl.InsertOne(ctx, *userRole)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}

	return &createdUserRole, nil
}

// UpdateUserRoles cập nhật danh sách roles cho một user
// Xóa tất cả roles cũ và thêm roles mới
// Tự động kiểm tra logic bảo vệ role Administrator
func (s *UserRoleService) UpdateUserRoles(ctx context.Context, userID primitive.ObjectID, newRoleIDs []primitive.ObjectID) ([]models.UserRole, error) {
	// Kiểm tra xem có thể xóa user khỏi role Administrator không
	if err := s.validateCanRemoveAdministratorRole(ctx, userID, newRoleIDs); err != nil {
		return nil, err
	}

	// Xóa tất cả user role cũ của user (dùng base service để tránh kiểm tra trùng lặp)
	filter := bson.M{"userId": userID}
	if _, err := s.BaseServiceMongoImpl.DeleteMany(ctx, filter); err != nil {
		return nil, err
	}

	// Tạo danh sách user role mới
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

	// Thêm các user role mới
	if len(userRoles) > 0 {
		_, err := s.InsertMany(ctx, userRoles)
		if err != nil {
			return nil, err
		}
	}

	return userRoles, nil
}

// validateCanRemoveAdministratorRole kiểm tra xem có thể xóa user khỏi role Administrator không
// Trả về lỗi nếu đây là user cuối cùng có role Administrator
func (s *UserRoleService) validateCanRemoveAdministratorRole(ctx context.Context, userID primitive.ObjectID, newRoleIDs []primitive.ObjectID) error {
	// Lấy role Administrator
	adminRole, err := s.roleService.FindOne(ctx, bson.M{"name": "Administrator"}, nil)
	if err != nil {
		// Nếu không tìm thấy role Administrator, không cần kiểm tra
		return nil
	}

	var modelAdminRole models.Role
	bsonBytes, _ := bson.Marshal(adminRole)
	if err := bson.Unmarshal(bsonBytes, &modelAdminRole); err != nil {
		return nil // Bỏ qua nếu không parse được
	}

	// Kiểm tra user hiện tại có role Administrator không
	oldUserRoles, err := s.Find(ctx, bson.M{"userId": userID, "roleId": modelAdminRole.ID}, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}

	hasAdminRoleInOldRoles := err == nil && len(oldUserRoles) > 0

	// Kiểm tra trong danh sách role mới có Administrator không
	hasAdminRoleInNewRoles := false
	for _, roleID := range newRoleIDs {
		if roleID == modelAdminRole.ID {
			hasAdminRoleInNewRoles = true
			break
		}
	}

	// Nếu user đang có role Administrator và sẽ bị xóa khỏi role Administrator
	if hasAdminRoleInOldRoles && !hasAdminRoleInNewRoles {
		// Kiểm tra xem đây có phải là user cuối cùng có role Administrator không
		allAdminUserRoles, err := s.Find(ctx, bson.M{"roleId": modelAdminRole.ID}, nil)
		if err != nil && err != common.ErrNotFound {
			return err
		}

		// Nếu chỉ còn 1 user (user đang bị update), không cho phép
		if err == nil && len(allAdminUserRoles) <= 1 {
			return common.NewError(
				common.ErrCodeBusinessOperation,
				"Không thể xóa user khỏi role Administrator. Role Administrator phải có ít nhất một user.",
				common.StatusConflict,
				nil,
			)
		}
	}

	return nil
}

// IsExist kiểm tra xem một UserRole đã tồn tại chưa
func (s *UserRoleService) IsExist(ctx context.Context, userID, roleID primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"userId": userID,
		"roleId": roleID,
	}
	count, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, common.ConvertMongoError(err)
	}
	return count > 0, nil
}

// validateBeforeDeleteAdministratorRole kiểm tra xem có thể xóa user khỏi role Administrator không
// Không cho phép xóa nếu đây là user cuối cùng có role Administrator
func (s *UserRoleService) validateBeforeDeleteAdministratorRole(ctx context.Context, userRoleID primitive.ObjectID) error {
	// Lấy thông tin userRole cần xóa
	userRole, err := s.FindOneById(ctx, userRoleID)
	if err != nil {
		return err
	}

	var modelUserRole models.UserRole
	bsonBytes, _ := bson.Marshal(userRole)
	err = bson.Unmarshal(bsonBytes, &modelUserRole)
	if err != nil {
		return common.ErrInvalidFormat
	}

	// Lấy thông tin role
	role, err := s.roleService.FindOneById(ctx, modelUserRole.RoleID)
	if err != nil {
		return err
	}

	var modelRole models.Role
	bsonBytes, _ = bson.Marshal(role)
	err = bson.Unmarshal(bsonBytes, &modelRole)
	if err != nil {
		return common.ErrInvalidFormat
	}

	// Kiểm tra nếu role là Administrator
	if modelRole.Name == "Administrator" {
		// Đếm số lượng user có role Administrator
		adminUserRoles, err := s.Find(ctx, bson.M{"roleId": modelRole.ID}, nil)
		if err != nil && err != common.ErrNotFound {
			return err
		}

		// Nếu chỉ còn 1 user (user đang bị xóa), không cho phép xóa
		if err == nil && len(adminUserRoles) <= 1 {
			return common.NewError(
				common.ErrCodeBusinessOperation,
				"Không thể xóa user khỏi role Administrator. Role Administrator phải có ít nhất một user.",
				common.StatusConflict,
				nil,
			)
		}
	}

	return nil
}

// validateBeforeDeleteAdministratorRoleByFilter kiểm tra khi xóa theo filter
func (s *UserRoleService) validateBeforeDeleteAdministratorRoleByFilter(ctx context.Context, filter bson.M) error {
	// Lấy danh sách userRole sẽ bị xóa
	userRoles, err := s.Find(ctx, filter, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}

	// Lấy role Administrator
	adminRole, err := s.roleService.FindOne(ctx, bson.M{"name": "Administrator"}, nil)
	if err != nil {
		// Nếu không tìm thấy role Administrator, không cần kiểm tra
		return nil
	}

	var modelAdminRole models.Role
	bsonBytes, _ := bson.Marshal(adminRole)
	err = bson.Unmarshal(bsonBytes, &modelAdminRole)
	if err != nil {
		return nil // Bỏ qua nếu không parse được
	}

	// Đếm số lượng user có role Administrator hiện tại
	allAdminUserRoles, err := s.Find(ctx, bson.M{"roleId": modelAdminRole.ID}, nil)
	if err != nil && err != common.ErrNotFound {
		return err
	}

	// Đếm số lượng userRole Administrator sẽ bị xóa
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

	// Nếu sau khi xóa, không còn user nào có role Administrator
	if err == nil && len(allAdminUserRoles) <= adminUserRolesToDelete {
		return common.NewError(
			common.ErrCodeBusinessOperation,
			"Không thể xóa user khỏi role Administrator. Role Administrator phải có ít nhất một user.",
			common.StatusConflict,
			nil,
		)
	}

	return nil
}

// DeleteOne override method DeleteOne để kiểm tra trước khi xóa
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.DeleteOne trực tiếp):
// 1. Business logic validation:
//    - Validate trước khi xóa: Không cho phép xóa Administrator role của user
//    - Đảm bảo luôn có ít nhất 1 Administrator role trong hệ thống
//    - Bảo vệ data integrity và security
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate trước khi xóa bằng validateBeforeDeleteAdministratorRoleByFilter()
// ✅ Gọi BaseServiceMongoImpl.DeleteOne để đảm bảo:
//   - Xóa document từ MongoDB
//   - Xử lý errors đúng cách
func (s *UserRoleService) DeleteOne(ctx context.Context, filter interface{}) error {
	// Chuyển filter sang bson.M để kiểm tra
	var filterMap bson.M
	switch v := filter.(type) {
	case bson.M:
		filterMap = v
	case bson.D:
		filterMap = v.Map()
	default:
		// Nếu không phải bson.M hoặc bson.D, thử convert
		filterBytes, _ := bson.Marshal(filter)
		var temp bson.M
		if err := bson.Unmarshal(filterBytes, &temp); err == nil {
			filterMap = temp
		} else {
			// Nếu không convert được, bỏ qua validation
			return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
		}
	}

	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDeleteAdministratorRoleByFilter(ctx, filterMap); err != nil {
		return err
	}

	// Thực hiện xóa
	return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
}

// DeleteById override method DeleteById để kiểm tra trước khi xóa
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.DeleteById trực tiếp):
// 1. Business logic validation:
//    - Validate trước khi xóa: Không cho phép xóa Administrator role của user
//    - Đảm bảo luôn có ít nhất 1 Administrator role trong hệ thống
//    - Bảo vệ data integrity và security
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate trước khi xóa bằng validateBeforeDeleteAdministratorRole()
// ✅ Gọi BaseServiceMongoImpl.DeleteById để đảm bảo:
//   - Xóa document từ MongoDB
//   - Xử lý errors đúng cách
func (s *UserRoleService) DeleteById(ctx context.Context, id primitive.ObjectID) error {
	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDeleteAdministratorRole(ctx, id); err != nil {
		return err
	}

	// Thực hiện xóa
	return s.BaseServiceMongoImpl.DeleteById(ctx, id)
}

// DeleteMany override method DeleteMany để kiểm tra trước khi xóa
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.DeleteMany trực tiếp):
// 1. Business logic validation:
//    - Validate từng user role trước khi xóa: Không cho phép xóa Administrator role của user
//    - Đảm bảo luôn có ít nhất 1 Administrator role trong hệ thống
//    - Bảo vệ data integrity và security
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate từng user role trước khi xóa bằng validateBeforeDeleteAdministratorRoleByFilter()
// ✅ Gọi BaseServiceMongoImpl.DeleteMany để đảm bảo:
//   - Xóa documents từ MongoDB
//   - Xử lý errors đúng cách
func (s *UserRoleService) DeleteMany(ctx context.Context, filter interface{}) (int64, error) {
	// Chuyển filter sang bson.M để kiểm tra
	var filterMap bson.M
	switch v := filter.(type) {
	case bson.M:
		filterMap = v
	case bson.D:
		filterMap = v.Map()
	default:
		// Nếu không phải bson.M hoặc bson.D, thử convert
		filterBytes, _ := bson.Marshal(filter)
		var temp bson.M
		if err := bson.Unmarshal(filterBytes, &temp); err == nil {
			filterMap = temp
		} else {
			// Nếu không convert được, bỏ qua validation
			return s.BaseServiceMongoImpl.DeleteMany(ctx, filter)
		}
	}

	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDeleteAdministratorRoleByFilter(ctx, filterMap); err != nil {
		return 0, err
	}

	// Thực hiện xóa
	return s.BaseServiceMongoImpl.DeleteMany(ctx, filter)
}
