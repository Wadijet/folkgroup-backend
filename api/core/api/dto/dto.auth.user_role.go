package dto

// UserRoleCreateInput đại diện cho dữ liệu đầu vào khi tạo vai trò người dùng
type UserRoleCreateInput struct {
	UserID string `json:"userId" validate:"required" transform:"str_objectid"` // ID của người dùng (bắt buộc)
	RoleID string `json:"roleId" validate:"required" transform:"str_objectid"` // ID của vai trò (bắt buộc)
}

// UserRoleUpdateInput đại diện cho dữ liệu đầu vào khi cập nhật vai trò người dùng
type UserRoleUpdateInput struct {
	UserID  string   `json:"userId" validate:"required" transform:"str_objectid"` // ID người dùng - transform sang Model primitive.ObjectID
	RoleIDs []string `json:"roleIds" validate:"required,min=1"`
}

