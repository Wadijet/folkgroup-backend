package authdto

// OrganizationShareCreateInput dùng cho tạo organization share.
type OrganizationShareCreateInput struct {
	OwnerOrganizationID string   `json:"ownerOrganizationId" validate:"required" transform:"str_objectid"`
	ToOrgIDs            []string `json:"toOrgIds,omitempty" transform:"str_objectid_array,optional"`
	PermissionNames     []string `json:"permissionNames,omitempty"`
	Description         string   `json:"description,omitempty"`
}

// OrganizationShareUpdateInput dùng cho cập nhật organization share.
type OrganizationShareUpdateInput struct {
	PermissionNames []string `json:"permissionNames,omitempty"`
	Description     string   `json:"description,omitempty"`
}
