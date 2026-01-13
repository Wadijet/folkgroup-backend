package handler

import (
	"fmt"
	"meta_commerce/core/api/dto"
	models "meta_commerce/core/api/models/mongodb"
	"meta_commerce/core/api/services"
	"meta_commerce/core/common"

	"github.com/gofiber/fiber/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserRoleHandler xử lý các route liên quan đến vai trò của người dùng cho Fiber
// Kế thừa từ FiberBaseHandler để có các chức năng CRUD cơ bản
// Các phương thức của FiberBaseHandler đã có sẵn:
// - InsertOne: Thêm mới một user role
// - InsertMany: Thêm nhiều user role
// - FindOne: Tìm một user role theo điều kiện
// - FindOneById: Tìm một user role theo ID
// - FindManyByIds: Tìm nhiều user role theo danh sách ID
// - FindWithPagination: Tìm user role với phân trang
// - Find: Tìm nhiều user role theo điều kiện
// - UpdateOne: Cập nhật một user role theo điều kiện
// - UpdateMany: Cập nhật nhiều user role theo điều kiện
// - UpdateById: Cập nhật một user role theo ID
// - DeleteOne: Xóa một user role theo điều kiện
// - DeleteMany: Xóa nhiều user role theo điều kiện
// - DeleteById: Xóa một user role theo ID
// - FindOneAndUpdate: Tìm và cập nhật một user role
// - FindOneAndDelete: Tìm và xóa một user role
// - CountDocuments: Đếm số lượng user role theo điều kiện
// - Distinct: Lấy danh sách giá trị duy nhất của một trường
// - Upsert: Thêm mới hoặc cập nhật một user role
// - UpsertMany: Thêm mới hoặc cập nhật nhiều user role
// - DocumentExists: Kiểm tra user role có tồn tại không
type UserRoleHandler struct {
	BaseHandler[models.UserRole, dto.UserRoleCreateInput, dto.UserRoleCreateInput]
	UserRoleService *services.UserRoleService
}

// NewUserRoleHandler tạo một instance mới của FiberUserRoleHandler
// Returns:
//   - *FiberUserRoleHandler: Instance mới của FiberUserRoleHandler đã được khởi tạo với UserRoleService
//   - error: Lỗi nếu có trong quá trình khởi tạo
func NewUserRoleHandler() (*UserRoleHandler, error) {
	// Khởi tạo UserRoleService
	userRoleService, err := services.NewUserRoleService()
	if err != nil {
		return nil, fmt.Errorf("failed to create user role service: %v", err)
	}

	handler := &UserRoleHandler{
		UserRoleService: userRoleService,
	}
	handler.BaseService = handler.UserRoleService
	return handler, nil
}

// Các hàm đặc thù của UserRole (nếu có) sẽ được thêm vào đây

// HandleUpdateUserRoles xử lý cập nhật vai trò cho người dùng
//
// LÝ DO PHẢI TẠO ENDPOINT ĐẶC BIỆT (không thể dùng CRUD chuẩn):
// 1. Logic nghiệp vụ phức tạp trong service:
//    - Gọi UserRoleService.UpdateUserRoles (service method đặc biệt)
//    - Service method này xử lý toàn bộ logic: validation, xóa roles cũ, tạo roles mới
//    - Có thể có validation đặc biệt: check role tồn tại, check scope, etc.
// 2. Atomic operation:
//    - Service method đảm bảo atomic: xóa tất cả user roles cũ rồi tạo mới
//    - Không có trạng thái trung gian (một phần roles cũ, một phần mới)
// 3. Input format đặc biệt:
//    - Input: {userId, roleIds: [...]}
//    - Output: Array các UserRole đã được tạo
//    - Không phải format CRUD chuẩn (update một document)
// 4. Service layer abstraction:
//    - Logic nghiệp vụ được đóng gói trong service method
//    - Handler chỉ là wrapper, gọi service method
//    - Service method có thể có validation và logic phức tạp
//
// KẾT LUẬN: Cần giữ endpoint đặc biệt vì logic nghiệp vụ phức tạp được đóng gói trong service method
//           và đây là atomic "replace all" operation
//
// Parameters:
//   - c: Context của Fiber chứa thông tin request
//
// Returns:
//   - error: Lỗi nếu có
//
// Request Body:
//   - userId: ID của người dùng cần cập nhật vai trò
//   - roleIds: Danh sách ID của các vai trò
//
// Response:
//   - 200: Cập nhật vai trò thành công
//     {
//     "message": "Thành công",
//     "data": [
//     {
//     "id": "...",
//     "userId": "...",
//     "roleId": "...",
//     "scope": 0,
//     "createdAt": 123,
//     "updatedAt": 123
//     }
//     ]
//     }
//   - 400: Dữ liệu không hợp lệ
//   - 500: Lỗi server
func (h *UserRoleHandler) HandleUpdateUserRoles(c fiber.Ctx) error {
	// Parse input từ request body
	input := new(dto.UserRoleUpdateInput)
	if err := h.ParseRequestBody(c, input); err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}

	// Chuyển đổi userId từ string sang ObjectID
	userId, err := primitive.ObjectIDFromHex(input.UserID)
	if err != nil {
		h.HandleResponse(c, nil, common.NewError(common.ErrCodeValidationFormat, "ID người dùng không hợp lệ", common.StatusBadRequest, err))
		return nil
	}

	// Chuyển đổi danh sách roleIDs sang ObjectID
	var newRoleIDs []primitive.ObjectID
	for _, roleIdStr := range input.RoleIDs {
		roleIdObj, err := primitive.ObjectIDFromHex(roleIdStr)
		if err == nil {
			newRoleIDs = append(newRoleIDs, roleIdObj)
		}
	}

	// Gọi service method để update user roles
	// Service sẽ tự động xử lý toàn bộ logic bao gồm validation
	userRoles, err := h.UserRoleService.UpdateUserRoles(c.Context(), userId, newRoleIDs)
	if err != nil {
		h.HandleResponse(c, nil, err)
		return nil
	}

	h.HandleResponse(c, userRoles, nil)
	return nil
}
