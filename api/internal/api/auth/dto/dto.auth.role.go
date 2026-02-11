package authdto

// RoleCreateInput dùng cho tạo vai trò.
type RoleCreateInput struct {
	Name                string `json:"name" validate:"required"`
	Describe            string `json:"describe,omitempty"`
	OwnerOrganizationID string `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}

// RoleUpdateInput dùng cho cập nhật vai trò.
type RoleUpdateInput struct {
	Name                string `json:"name"`
	Describe            string `json:"describe"`
	OwnerOrganizationID string `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}
