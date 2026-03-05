package dto

// MetaAdSetCreateInput input tạo ad set.
type MetaAdSetCreateInput struct {
	AdSetId             string                 `json:"adSetId" validate:"required"`
	CampaignId          string                 `json:"campaignId" validate:"required"`
	AdAccountId         string                 `json:"adAccountId" validate:"required"`
	Name                string                 `json:"name"`
	Status              string                 `json:"status"`
	EffectiveStatus     string                 `json:"effectiveStatus"`
	DailyBudget         string                 `json:"dailyBudget"`
	LifetimeBudget      string                 `json:"lifetimeBudget"`
	MetaData            map[string]interface{} `json:"metaData"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}

// MetaAdSetUpdateInput input cập nhật ad set.
type MetaAdSetUpdateInput struct {
	Name                string                 `json:"name"`
	Status              string                 `json:"status"`
	EffectiveStatus     string                 `json:"effectiveStatus"`
	DailyBudget         string                 `json:"dailyBudget"`
	LifetimeBudget      string                 `json:"lifetimeBudget"`
	MetaData            map[string]interface{} `json:"metaData"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}
