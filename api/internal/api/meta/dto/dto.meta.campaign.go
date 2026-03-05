package dto

// MetaCampaignCreateInput input tạo campaign.
type MetaCampaignCreateInput struct {
	CampaignId          string                 `json:"campaignId" validate:"required"` // ID campaign trên Meta
	AdAccountId         string                 `json:"adAccountId" validate:"required"`
	Name                string                 `json:"name"`
	Objective           string                 `json:"objective"`
	Status              string                 `json:"status"`
	EffectiveStatus     string                 `json:"effectiveStatus"`
	MetaData            map[string]interface{} `json:"metaData"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}

// MetaCampaignUpdateInput input cập nhật campaign.
type MetaCampaignUpdateInput struct {
	Name                string                 `json:"name"`
	Objective           string                 `json:"objective"`
	Status              string                 `json:"status"`
	EffectiveStatus     string                 `json:"effectiveStatus"`
	MetaData            map[string]interface{} `json:"metaData"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}
