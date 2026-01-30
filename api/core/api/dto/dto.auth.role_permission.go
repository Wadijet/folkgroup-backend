package dto

// RolePermissionCreateInput đại diện cho dữ liệu đầu vào khi tạo quyền vai trò
type RolePermissionCreateInput struct {
	RoleID       string `json:"roleId" validate:"required" transform:"str_objectid"`       // ID của vai trò
	PermissionID string `json:"permissionId" validate:"required" transform:"str_objectid"` // ID của quyền
	Scope        byte   `json:"scope"`                                                     // Phạm vi của quyền (0: Chỉ tổ chức role thuộc về - default, 1: Tổ chức đó và tất cả các tổ chức con)
}

// RolePermissionUpdateItem đại diện cho một permission trong danh sách cập nhật
type RolePermissionUpdateItem struct {
	PermissionID string `json:"permissionId" validate:"required" transform:"str_objectid"` // ID của quyền - transform sang Model primitive.ObjectID
	Scope        byte   `json:"scope"`                                                     // Phạm vi (0: chỉ org role, 1: org và con)
}

// RolePermissionUpdateInput dữ liệu đầu vào khi cập nhật quyền của vai trò
type RolePermissionUpdateInput struct {
	RoleID       string                     `json:"roleId" validate:"required" transform:"str_objectid"` // ID vai trò - transform sang Model primitive.ObjectID
	Permissions  []RolePermissionUpdateItem `json:"permissions" validate:"required"`                       // Danh sách quyền với scope
}
