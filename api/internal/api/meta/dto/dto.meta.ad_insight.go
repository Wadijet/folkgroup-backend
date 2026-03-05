package dto

// MetaAdInsightCreateInput input tạo ad insight.
type MetaAdInsightCreateInput struct {
	ObjectId            string                 `json:"objectId" validate:"required"`   // act_xxx, campaign_id, adset_id, ad_id
	ObjectType          string                 `json:"objectType" validate:"required"` // ad_account, campaign, adset, ad
	AdAccountId         string                 `json:"adAccountId" validate:"required"`
	DateStart           string                 `json:"dateStart" validate:"required"` // YYYY-MM-DD
	DateStop            string                 `json:"dateStop"`
	Impressions         string                 `json:"impressions"`
	Clicks              string                 `json:"clicks"`
	Spend               string                 `json:"spend"`
	Reach               string                 `json:"reach"`
	Cpc                 string                 `json:"cpc"`
	Cpm                 string                 `json:"cpm"`
	Ctr                 string                 `json:"ctr"`
	MetaData            map[string]interface{} `json:"metaData"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}

// MetaAdInsightUpdateInput input cập nhật ad insight.
type MetaAdInsightUpdateInput struct {
	Impressions         string                 `json:"impressions"`
	Clicks              string                 `json:"clicks"`
	Spend               string                 `json:"spend"`
	Reach               string                 `json:"reach"`
	Cpc                 string                 `json:"cpc"`
	Cpm                 string                 `json:"cpm"`
	Ctr                 string                 `json:"ctr"`
	MetaData            map[string]interface{} `json:"metaData"`
	OwnerOrganizationID string                 `json:"ownerOrganizationId,omitempty" transform:"str_objectid,optional"`
}
