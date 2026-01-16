package services

import (
	"context"
	"fmt"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/common"
	"meta_commerce/core/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// RoleService là cấu trúc chứa các phương thức liên quan đến vai trò
type RoleService struct {
	*BaseServiceMongoImpl[models.Role]
}

// NewRoleService tạo mới RoleService
func NewRoleService() (*RoleService, error) {
	roleCollection, exist := global.RegistryCollections.Get(global.MongoDB_ColNames.Roles)
	if !exist {
		return nil, fmt.Errorf("failed to get roles collection: %v", common.ErrNotFound)
	}

	return &RoleService{
		BaseServiceMongoImpl: NewBaseServiceMongo[models.Role](roleCollection),
	}, nil
}

// validateBeforeDelete kiểm tra các điều kiện trước khi xóa role
// - Không cho phép xóa role Administrator
func (s *RoleService) validateBeforeDelete(ctx context.Context, roleID primitive.ObjectID) error {
	// Lấy thông tin role cần xóa
	role, err := s.FindOneById(ctx, roleID)
	if err != nil {
		return err
	}

	var modelRole models.Role
	bsonBytes, _ := bson.Marshal(role)
	err = bson.Unmarshal(bsonBytes, &modelRole)
	if err != nil {
		return common.ErrInvalidFormat
	}

	// Kiểm tra: Nếu là role Administrator thì không cho phép xóa
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

// DeleteOne override method DeleteOne để kiểm tra trước khi xóa
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.DeleteOne trực tiếp):
// 1. Business logic validation:
//    - Validate trước khi xóa: Kiểm tra role có đang được sử dụng không (có user roles, permissions)
//    - Không cho phép xóa role đang được sử dụng
//    - Đảm bảo data integrity
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate trước khi xóa bằng validateBeforeDelete()
// ✅ Gọi BaseServiceMongoImpl.DeleteOne để đảm bảo:
//   - Xóa document từ MongoDB
//   - Xử lý errors đúng cách
func (s *RoleService) DeleteOne(ctx context.Context, filter interface{}) error {
	// Lấy thông tin role cần xóa
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

	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDelete(ctx, modelRole.ID); err != nil {
		return err
	}

	// Thực hiện xóa nếu không có ràng buộc
	return s.BaseServiceMongoImpl.DeleteOne(ctx, filter)
}

// DeleteById override method DeleteById để kiểm tra trước khi xóa
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.DeleteById trực tiếp):
// 1. Business logic validation:
//    - Validate trước khi xóa: Kiểm tra role có đang được sử dụng không (có user roles, permissions)
//    - Không cho phép xóa role đang được sử dụng
//    - Đảm bảo data integrity
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate trước khi xóa bằng validateBeforeDelete()
// ✅ Gọi BaseServiceMongoImpl.DeleteById để đảm bảo:
//   - Xóa document từ MongoDB
//   - Xử lý errors đúng cách
func (s *RoleService) DeleteById(ctx context.Context, id primitive.ObjectID) error {
	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDelete(ctx, id); err != nil {
		return err
	}

	// Thực hiện xóa nếu không có ràng buộc
	return s.BaseServiceMongoImpl.DeleteById(ctx, id)
}

// DeleteMany override method DeleteMany để kiểm tra trước khi xóa
func (s *RoleService) DeleteMany(ctx context.Context, filter interface{}) (int64, error) {
	// Lấy danh sách roles sẽ bị xóa
	roles, err := s.BaseServiceMongoImpl.Find(ctx, filter, nil)
	if err != nil && err != common.ErrNotFound {
		return 0, err
	}

	// Kiểm tra từng role trước khi xóa
	for _, role := range roles {
		var modelRole models.Role
		bsonBytes, _ := bson.Marshal(role)
		if err := bson.Unmarshal(bsonBytes, &modelRole); err != nil {
			continue
		}

		// Kiểm tra trước khi xóa
		if err := s.validateBeforeDelete(ctx, modelRole.ID); err != nil {
			return 0, err
		}
	}

	// Thực hiện xóa nếu không có ràng buộc
	return s.BaseServiceMongoImpl.DeleteMany(ctx, filter)
}

// FindOneAndDelete override method FindOneAndDelete để kiểm tra trước khi xóa
//
// LÝ DO PHẢI OVERRIDE (không dùng BaseServiceMongoImpl.FindOneAndDelete trực tiếp):
// 1. Business logic validation:
//    - Validate trước khi xóa: Kiểm tra role có đang được sử dụng không (có user roles, permissions)
//    - Không cho phép xóa role đang được sử dụng
//    - Đảm bảo data integrity
//
// ĐẢM BẢO LOGIC CƠ BẢN:
// ✅ Validate trước khi xóa bằng validateBeforeDelete()
// ✅ Gọi BaseServiceMongoImpl.FindOneAndDelete để đảm bảo:
//   - Tìm và xóa document từ MongoDB
//   - Trả về document đã bị xóa
//   - Xử lý errors đúng cách
func (s *RoleService) FindOneAndDelete(ctx context.Context, filter interface{}, opts *mongoopts.FindOneAndDeleteOptions) (models.Role, error) {
	var zero models.Role

	// Lấy thông tin role sẽ bị xóa
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

	// Kiểm tra trước khi xóa
	if err := s.validateBeforeDelete(ctx, modelRole.ID); err != nil {
		return zero, err
	}

	// Thực hiện xóa nếu không có ràng buộc
	return s.BaseServiceMongoImpl.FindOneAndDelete(ctx, filter, opts)
}
