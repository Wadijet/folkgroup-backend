package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
)

// PermissionHandler xử lý các route liên quan đến permission cho Fiber
// Kế thừa từ FiberBaseHandler để có các chức năng CRUD cơ bản
// Các phương thức của FiberBaseHandler đã có sẵn:
// - InsertOne: Thêm mới một permission
// - InsertMany: Thêm nhiều permission
// - FindOne: Tìm một permission theo điều kiện
// - FindOneById: Tìm một permission theo ID
// - FindManyByIds: Tìm nhiều permission theo danh sách ID
// - FindWithPagination: Tìm permission với phân trang
// - Find: Tìm nhiều permission theo điều kiện
// - UpdateOne: Cập nhật một permission theo điều kiện
// - UpdateMany: Cập nhật nhiều permission theo điều kiện
// - UpdateById: Cập nhật một permission theo ID
// - DeleteOne: Xóa một permission theo điều kiện
// - DeleteMany: Xóa nhiều permission theo điều kiện
// - DeleteById: Xóa một permission theo ID
// - FindOneAndUpdate: Tìm và cập nhật một permission
// - FindOneAndDelete: Tìm và xóa một permission
// - CountDocuments: Đếm số lượng permission theo điều kiện
// - Distinct: Lấy danh sách giá trị duy nhất của một trường
// - Upsert: Thêm mới hoặc cập nhật một permission
// - UpsertMany: Thêm mới hoặc cập nhật nhiều permission
// - DocumentExists: Kiểm tra permission có tồn tại không
type PermissionHandler struct {
	BaseHandler[models.Permission, dto.PermissionCreateInput, dto.PermissionUpdateInput]
}

// NewPermissionHandler tạo một instance mới của FiberPermissionHandler
// Returns:
//   - *FiberPermissionHandler: Instance mới của FiberPermissionHandler đã được khởi tạo với PermissionService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewPermissionHandler() (*PermissionHandler, error) {
	handler := &PermissionHandler{}

	// Khởi tạo PermissionService
	permissionService, err := services.NewPermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create permission service: %v", err)
	}

	handler.BaseService = permissionService
	return handler, nil
}

// Tất cả các CRUD operations đã được cung cấp bởi BaseHandler:
// - InsertOne: Tạo mới permission
// - UpdateById: Cập nhật permission theo ID
// - FindOneById: Lấy permission theo ID
// - FindWithPagination: Lấy danh sách permission với phân trang
// - DeleteById: Xóa permission theo ID
// - Find: Lấy danh sách permission với filter (có thể dùng cho category/group)
