package authdto

// OrganizationCreateInput đầu vào khi tạo tổ chức.
type OrganizationCreateInput struct {
	Name     string `json:"name" validate:"required"`
	Code     string `json:"code" validate:"required"`
	Type     string `json:"type" validate:"required"`
	ParentID string `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"`
	IsActive bool   `json:"isActive"`
}

// OrganizationUpdateInput đầu vào khi cập nhật tổ chức.
type OrganizationUpdateInput struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	Type     string `json:"type"`
	ParentID string `json:"parentId,omitempty" transform:"str_objectid_ptr,optional"`
	IsActive *bool  `json:"isActive,omitempty"`
}
