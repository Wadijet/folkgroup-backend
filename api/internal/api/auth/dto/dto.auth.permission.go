package authdto

// PermissionCreateInput đầu vào tạo mới quyền.
type PermissionCreateInput struct {
	Name     string `json:"name" validate:"required"`
	Describe string `json:"describe,omitempty"`
	Category string `json:"category" validate:"required"`
	Group    string `json:"group" validate:"required"`
}

// PermissionUpdateInput đầu vào cập nhật quyền.
type PermissionUpdateInput struct {
	Name     string `json:"name"`
	Describe string `json:"describe"`
	Category string `json:"category"`
	Group    string `json:"group"`
}
