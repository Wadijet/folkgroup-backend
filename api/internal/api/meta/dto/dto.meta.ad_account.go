package dto

// MetaAdAccountCreateInput input tạo ad account.
type MetaAdAccountCreateInput struct {
	AdAccountId         string `json:"adAccountId" validate:"required"`           // act_123456789
	Name                string `json:"name"`                                     // Tên (optional)
	OwnerOrganizationID string `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"` // Tổ chức sở hữu (tự transform string → ObjectID)
}

// MetaAdAccountUpdateInput input cập nhật ad account.
type MetaAdAccountUpdateInput struct {
	Name                string `json:"name"`                                     // Tên
	OwnerOrganizationID string `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"` // Tổ chức sở hữu
	AccountMode         string `json:"accountMode,omitempty"`                     // BLITZ | NORMAL | EFFICIENCY | PROTECT (Layer 4 — Account Mode System)
}
