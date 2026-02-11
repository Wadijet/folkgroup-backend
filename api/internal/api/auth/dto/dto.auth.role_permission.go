package authdto

// RolePermissionCreateInput đầu vào tạo quyền vai trò.
type RolePermissionCreateInput struct {
	RoleID       string `json:"roleId" validate:"required" transform:"str_objectid"`
	PermissionID string `json:"permissionId" validate:"required" transform:"str_objectid"`
	Scope        byte   `json:"scope"`
}

// RolePermissionUpdateItem một permission trong danh sách cập nhật.
type RolePermissionUpdateItem struct {
	PermissionID string `json:"permissionId" validate:"required" transform:"str_objectid"`
	Scope        byte   `json:"scope"`
}

// RolePermissionUpdateInput đầu vào cập nhật quyền của vai trò.
type RolePermissionUpdateInput struct {
	RoleID      string                     `json:"roleId" validate:"required" transform:"str_objectid"`
	Permissions []RolePermissionUpdateItem `json:"permissions" validate:"required"`
}
