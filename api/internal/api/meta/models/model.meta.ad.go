package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MetaAd lưu thông tin Ad từ Meta Marketing API.
// Các field có extract tag để lấy từ metaData khi ghi. CreativeId lấy từ metaData.creative.id.
type MetaAd struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	AdId                string                 `json:"adId" bson:"adId" index:"unique;text;single:1,compound:meta_ad_lookup_unique;compound:meta_ad_by_adset" extract:"metaData\\.id,converter=string,required"`
	AdSetId             string                 `json:"adSetId" bson:"adSetId" index:"text;single:1,compound:meta_ad_by_adset" extract:"metaData\\.adset_id,converter=string,optional"`
	CampaignId          string                 `json:"campaignId" bson:"campaignId" index:"text" extract:"metaData\\.campaign_id,converter=string,optional"`
	AdAccountId         string                 `json:"adAccountId" bson:"adAccountId" index:"text;single:1,compound:meta_ad_lookup_unique;compound:meta_ad_by_adset" extract:"metaData\\.account_id,converter=string,optional"`
	Name                string                 `json:"name" bson:"name" index:"text" extract:"metaData\\.name,converter=string,optional"`
	Status              string                 `json:"status" bson:"status" extract:"metaData\\.status,converter=string,optional"`
	EffectiveStatus     string                 `json:"effectiveStatus" bson:"effectiveStatus" extract:"metaData\\.effective_status,converter=string,optional"`
	CreativeId          string                 `json:"creativeId" bson:"creativeId" extract:"metaData\\.creative\\.id,converter=string,optional"`
	MetaData            map[string]interface{} `json:"metaData" bson:"metaData"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:meta_ad_lookup_unique;compound:meta_ad_by_adset"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`                                                                                                                       // Thời gian tạo bản ghi trong hệ thống (lúc sync lần đầu)
	MetaCreatedAt       int64                  `json:"metaCreatedAt" bson:"metaCreatedAt" extract:"metaData\\.created_time,converter=time,format=2006-01-02T15:04:05-0700,optional"`                       // Thời gian tạo gốc từ Meta API (khi ad được tạo trên Meta)
	UpdatedAt           int64                  `json:"updatedAt" bson:"updatedAt"`
	LastSyncedAt        int64                  `json:"lastSyncedAt" bson:"lastSyncedAt"`

	// CurrentMetrics: trạng thái metrics hiện tại (raw/layer1/layer2/layer3). Cập nhật khi insight sync hoặc order mới.
	CurrentMetrics map[string]interface{} `json:"currentMetrics,omitempty" bson:"currentMetrics,omitempty"`
}
