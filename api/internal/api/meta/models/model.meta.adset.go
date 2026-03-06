package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MetaAdSet lưu thông tin Ad Set từ Meta Marketing API.
// Các field có extract tag để lấy từ metaData khi ghi.
type MetaAdSet struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	AdSetId             string                 `json:"adSetId" bson:"adSetId" index:"unique;text;single:1,compound:meta_adset_lookup_unique;compound:meta_adset_by_campaign" extract:"metaData\\.id,converter=string,required"`
	CampaignId          string                 `json:"campaignId" bson:"campaignId" index:"text;single:1,compound:meta_adset_by_campaign" extract:"metaData\\.campaign_id,converter=string,optional"`
	AdAccountId         string                 `json:"adAccountId" bson:"adAccountId" index:"text;single:1,compound:meta_adset_lookup_unique;compound:meta_adset_by_campaign" extract:"metaData\\.account_id,converter=string,optional"`
	Name                string                 `json:"name" bson:"name" index:"text" extract:"metaData\\.name,converter=string,optional"`
	Status              string                 `json:"status" bson:"status" extract:"metaData\\.status,converter=string,optional"`
	EffectiveStatus     string                 `json:"effectiveStatus" bson:"effectiveStatus" extract:"metaData\\.effective_status,converter=string,optional"`
	DailyBudget         string                 `json:"dailyBudget" bson:"dailyBudget" extract:"metaData\\.daily_budget,converter=string,optional"`
	LifetimeBudget      string                 `json:"lifetimeBudget" bson:"lifetimeBudget" extract:"metaData\\.lifetime_budget,converter=string,optional"`
	MetaData            map[string]interface{} `json:"metaData" bson:"metaData"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:meta_adset_lookup_unique;compound:meta_adset_by_campaign"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`                                                                                                                       // Thời gian tạo bản ghi trong hệ thống (lúc sync lần đầu)
	MetaCreatedAt       int64                  `json:"metaCreatedAt" bson:"metaCreatedAt" extract:"metaData\\.created_time,converter=time,format=2006-01-02T15:04:05-0700,optional"`                       // Thời gian tạo gốc từ Meta API (khi adset được tạo trên Meta)
	UpdatedAt           int64                  `json:"updatedAt" bson:"updatedAt"`
	LastSyncedAt        int64                  `json:"lastSyncedAt" bson:"lastSyncedAt"`

	// CurrentMetrics: trạng thái metrics hiện tại (raw/layer1/layer2/layer3). Cập nhật khi insight sync hoặc order mới.
	CurrentMetrics map[string]interface{} `json:"currentMetrics,omitempty" bson:"currentMetrics,omitempty"`
}
