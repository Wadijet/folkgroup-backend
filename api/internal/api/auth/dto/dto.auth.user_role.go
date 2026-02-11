package authdto

// UserRoleCreateInput đầu vào tạo vai trò người dùng.
type UserRoleCreateInput struct {
	UserID string `json:"userId" validate:"required" transform:"str_objectid"`
	RoleID string `json:"roleId" validate:"required" transform:"str_objectid"`
}

// UserRoleUpdateInput đầu vào cập nhật vai trò người dùng.
type UserRoleUpdateInput struct {
	UserID  string   `json:"userId" validate:"required" transform:"str_objectid"`
	RoleIDs []string `json:"roleIds" validate:"required,min=1"`
}
