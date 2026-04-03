package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/utility/identity"
)

// MetaCampaign lưu thông tin Campaign từ Meta Marketing API.
// Các field CampaignId, AdAccountId, Name, Objective, Status, EffectiveStatus có extract tag để lấy từ metaData khi ghi.
type MetaCampaign struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	// ===== IDENTITY 4 LỚP =====
	Uid           string                       `json:"uid" bson:"uid" index:"single:1"`
	SourceIds     map[string]string            `json:"sourceIds,omitempty" bson:"sourceIds,omitempty"`
	SourceIdsMeta string                       `json:"-" bson:"sourceIds.meta,omitempty" index:"single:1,sparse"`
	Links         map[string]identity.LinkItem `json:"links,omitempty" bson:"links,omitempty"`
	CampaignId          string                 `json:"campaignId" bson:"campaignId" index:"unique;text;single:1,compound:meta_campaign_lookup_unique;compound:meta_campaign_by_account" extract:"metaData\\.id,converter=string,required"`
	AdAccountId         string                 `json:"adAccountId" bson:"adAccountId" index:"text;single:1,compound:meta_campaign_lookup_unique;compound:meta_campaign_by_account" extract:"metaData\\.account_id,converter=string,optional"`
	Name                string                 `json:"name" bson:"name" index:"text" extract:"metaData\\.name,converter=string,optional"`
	Objective           string                 `json:"objective" bson:"objective" extract:"metaData\\.objective,converter=string,optional"`
	Status              string                 `json:"status" bson:"status" extract:"metaData\\.status,converter=string,optional"`
	EffectiveStatus     string                 `json:"effectiveStatus" bson:"effectiveStatus" extract:"metaData\\.effective_status,converter=string,optional"`
	MetaData            map[string]interface{} `json:"metaData" bson:"metaData"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:meta_campaign_lookup_unique;compound:meta_campaign_by_account"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`                                                                                                                       // Thời gian tạo bản ghi trong hệ thống (lúc sync lần đầu)
	MetaCreatedAt       int64                  `json:"metaCreatedAt" bson:"metaCreatedAt" extract:"metaData\\.created_time,converter=time,format=2006-01-02T15:04:05-0700,optional"`                       // Thời gian tạo gốc từ Meta API (khi campaign được tạo trên Meta)
	UpdatedAt           int64                  `json:"updatedAt" bson:"updatedAt"`
	LastSyncedAt        int64                  `json:"lastSyncedAt" bson:"lastSyncedAt"`

	// CurrentMetrics: trạng thái metrics hiện tại (raw/layer1/layer2/layer3). Cập nhật khi insight sync hoặc order mới.
	CurrentMetrics map[string]interface{} `json:"currentMetrics,omitempty" bson:"currentMetrics,omitempty"`
}
