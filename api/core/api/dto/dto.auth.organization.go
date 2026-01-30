package dto

// OrganizationCreateInput dữ liệu đầu vào khi tạo tổ chức (tầng transport)
type OrganizationCreateInput struct {
	Name     string `json:"name" validate:"required"`                                 // Tên tổ chức (bắt buộc)
	Code     string `json:"code" validate:"required"`                                 // Mã tổ chức (bắt buộc, unique)
	Type     string `json:"type" validate:"required"`                                 // Loại tổ chức: system, group, company, department, division, team (bắt buộc)
	ParentID string `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"` // ID tổ chức cha (tùy chọn, dạng string ObjectID) → convert sang *primitive.ObjectID
	IsActive bool   `json:"isActive"`                                                 // Trạng thái hoạt động (mặc định: true)
}

// OrganizationUpdateInput dữ liệu đầu vào khi cập nhật tổ chức (tầng transport)
type OrganizationUpdateInput struct {
	Name     string `json:"name"`                                                     // Tên tổ chức
	Code     string `json:"code"`                                                     // Mã tổ chức (unique)
	Type     string `json:"type"`                                                     // Loại tổ chức: system, group, company, department, division, team
	ParentID string `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"` // ID tổ chức cha (dạng string ObjectID) → convert sang *primitive.ObjectID
	IsActive *bool  `json:"isActive,omitempty"`                                       // Trạng thái hoạt động (dùng pointer để phân biệt false và không cập nhật)
}
