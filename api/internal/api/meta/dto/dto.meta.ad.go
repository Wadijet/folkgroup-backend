package dto

// MetaAdCreateInput input tạo ad.
type MetaAdCreateInput struct {
	AdId                string                 `json:"adId" validate:"required"`
	AdSetId             string                 `json:"adSetId" validate:"required"`
	CampaignId          string                 `json:"campaignId" validate:"required"`
	AdAccountId         string                 `json:"adAccountId" validate:"required"`
	Name                string                 `json:"name"`
	Status              string                 `json:"status"`
	EffectiveStatus     string                 `json:"effectiveStatus"`
	CreativeId          string                 `json:"creativeId"`
	MetaData            map[string]interface{} `json:"metaData"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}

// MetaAdUpdateInput input cập nhật ad.
type MetaAdUpdateInput struct {
	Name                string                 `json:"name"`
	Status              string                 `json:"status"`
	EffectiveStatus     string                 `json:"effectiveStatus"`
	CreativeId          string                 `json:"creativeId"`
	MetaData            map[string]interface{} `json:"metaData"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}
