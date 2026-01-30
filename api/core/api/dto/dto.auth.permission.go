package dto

// PermissionCreateInput là dữ liệu đầu vào để tạo mới quyền (tầng transport)
type PermissionCreateInput struct {
	Name     string `json:"name" validate:"required"`     // Tên của quyền
	Describe string `json:"describe,omitempty"` // Mô tả quyền (tùy chọn, để trống được)
	Category string `json:"category" validate:"required"` // Danh mục của quyền
	Group    string `json:"group" validate:"required"`    // Nhóm của quyền
}

// PermissionUpdateInput là dữ liệu đầu vào để cập nhật quyền (tầng transport)
type PermissionUpdateInput struct {
	Name     string `json:"name"`     // Tên của quyền
	Describe string `json:"describe"` // Mô tả quyền
	Category string `json:"category"` // Danh mục của quyền
	Group    string `json:"group"`    // Nhóm của quyền
}

