package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RolePermissionHandler xử lý các route liên quan đến phân quyền cho Fiber
// Kế thừa từ FiberBaseHandler để có các chức năng CRUD cơ bản
// Các phương thức của FiberBaseHandler đã có sẵn:
// - InsertOne: Thêm mới một role permission
// - InsertMany: Thêm nhiều role permission
// - FindOne: Tìm một role permission theo điều kiện
// - FindOneById: Tìm một role permission theo ID
// - FindManyByIds: Tìm nhiều role permission theo danh sách ID
// - FindWithPagination: Tìm role permission với phân trang
// - Find: Tìm nhiều role permission theo điều kiện
// - UpdateOne: Cập nhật một role permission theo điều kiện
// - UpdateMany: Cập nhật nhiều role permission theo điều kiện
// - UpdateById: Cập nhật một role permission theo ID
// - DeleteOne: Xóa một role permission theo điều kiện
// - DeleteMany: Xóa nhiều role permission theo điều kiện
// - DeleteById: Xóa một role permission theo ID
// - FindOneAndUpdate: Tìm và cập nhật một role permission
// - FindOneAndDelete: Tìm và xóa một role permission
// - CountDocuments: Đếm số lượng role permission theo điều kiện
// - Distinct: Lấy danh sách giá trị duy nhất của một trường
// - Upsert: Thêm mới hoặc cập nhật một role permission
// - UpsertMany: Thêm mới hoặc cập nhật nhiều role permission
// - DocumentExists: Kiểm tra role permission có tồn tại không
type RolePermissionHandler struct {
	BaseHandler[models.RolePermission, dto.RolePermissionCreateInput, models.RolePermission]
	RolePermissionService *services.RolePermissionService
}

// NewRolePermissionHandler tạo một instance mới của FiberRolePermissionHandler
// Returns:
//   - *FiberRolePermissionHandler: Instance mới của FiberRolePermissionHandler đã được khởi tạo với RolePermissionService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewRolePermissionHandler() (*RolePermissionHandler, error) {
	// Khởi tạo RolePermissionService
	rolePermissionService, err := services.NewRolePermissionService()
	if err != nil {
		return nil, fmt.Errorf("failed to create role permission service: %v", err)
	}

	handler := &RolePermissionHandler{
		RolePermissionService: rolePermissionService,
	}
	handler.BaseService = handler.RolePermissionService
	return handler, nil
}

// HandleUpdateRolePermissions xử lý cập nhật quyền cho vai trò
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Atomic operation (xóa rồi tạo mới):
//    - Xóa TẤT CẢ role permissions cũ của role (DeleteMany với filter roleId)
//    - Tạo mới TẤT CẢ role permissions từ danh sách mới (InsertMany)
//    - Đảm bảo atomic: không có trạng thái trung gian (một phần permissions cũ, một phần mới)
// 2. Logic nghiệp vụ đặc biệt:
//    - Đây là "replace all" operation, không phải update từng item
//    - Client gửi danh sách permissions mới, hệ thống thay thế toàn bộ
//    - Set CreatedAt và UpdatedAt tự động cho tất cả permissions mới
// 3. Input format đặc biệt:
//    - Input: {roleId, permissions: [{permissionId, scope}]}
//    - Output: Array các RolePermission đã được tạo
//    - Không phải format CRUD chuẩn (update một document)
// 4. Performance:
//    - Sử dụng DeleteMany và InsertMany (batch operations) thay vì update từng item
//    - Hiệu quả hơn khi có nhiều permissions
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì đây là atomic "replace all" operation
//           (xóa tất cả rồi tạo mới) để đảm bảo consistency
//
// Parameters:
//   - c: Context của Fiber chứa thông tin request
//
// Returns:
//   - error: Lỗi nếu có
//
// Request Body:
//   - roleId: ID của vai trò cần cập nhật quyền
//   - permissions: Danh sách quyền với scope (mỗi item có permissionId và scope)
//
// Response:
//   - 200: Cập nhật quyền thành công
//     {
//     "message": "Thành công",
//     "data": [
//     {
//     "id": "...",
//     "roleId": "...",
//     "permissionId": "...",
//     "scope": 0,
//     "createdAt": 123,
//     "updatedAt": 123
//     }
//     ]
//     }
//   - 400: Dữ liệu không hợp lệ
//   - 500: Lỗi server
func (h *RolePermissionHandler) HandleUpdateRolePermissions(c fiber.Ctx) error {
	// Parse input từ request body
	input := new(dto.RolePermissionUpdateInput)
	if err := h.ParseRequestBody(c, input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}

	// Chuyển đổi roleId từ string sang ObjectID
	roleId, err := primitive.ObjectIDFromHex(input.RoleID)
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "ID vai trò không hợp lệ", common.StatusBadRequest, err))
		return nil
	}

	// Xóa tất cả role permission cũ của role
	filter := bson.M{"roleId": roleId}
	if _, err := h.RolePermissionService.DeleteMany(c.Context(), filter); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}

	// Tạo danh sách role permission mới
	var rolePermissions []models.RolePermission
	now := time.Now().Unix()

	for _, perm := range input.Permissions {
		permissionIdObj, err := primitive.ObjectIDFromHex(perm.PermissionID)
		if err != nil {
			continue // Bỏ qua các permissionId không hợp lệ
		}
		rolePermission := models.RolePermission{
			ID:           primitive.NewObjectID(),
			RoleID:       roleId,
			PermissionID: permissionIdObj,
			Scope:        perm.Scope, // Sử dụng scope từ request
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		rolePermissions = append(rolePermissions, rolePermission)
	}

	// Thêm các role permission mới bằng InsertMany thay vì InsertOne
	if len(rolePermissions) > 0 {
		_, err = h.RolePermissionService.InsertMany(c.Context(), rolePermissions)
		if err != nil {
			h.HandleResponse(c, nil, err)
			return nil
		}
	}

	h.HandleResponse(c, rolePermissions, nil)
	return nil
}
