package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MetaCampaign lưu thông tin Campaign từ Meta Marketing API.
// Các field CampaignId, AdAccountId, Name, Objective, Status, EffectiveStatus có extract tag để lấy từ metaData khi ghi.
type MetaCampaign struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	CampaignId          string                 `json:"campaignId" bson:"campaignId" index:"unique;text" extract:"metaData\\.id,converter=string,optional"`
	AdAccountId         string                 `json:"adAccountId" bson:"adAccountId" index:"text" extract:"metaData\\.account_id,converter=string,optional"`
	Name                string                 `json:"name" bson:"name" index:"text" extract:"metaData\\.name,converter=string,optional"`
	Objective           string                 `json:"objective" bson:"objective" extract:"metaData\\.objective,converter=string,optional"`
	Status              string                 `json:"status" bson:"status" extract:"metaData\\.status,converter=string,optional"`
	EffectiveStatus     string                 `json:"effectiveStatus" bson:"effectiveStatus" extract:"metaData\\.effective_status,converter=string,optional"`
	MetaData            map[string]interface{} `json:"metaData" bson:"metaData"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64                  `json:"updatedAt" bson:"updatedAt"`
	LastSyncedAt        int64                  `json:"lastSyncedAt" bson:"lastSyncedAt"`

	// CurrentMetrics: trạng thái metrics hiện tại (raw/layer1/layer2/layer3). Cập nhật khi insight sync hoặc order mới.
	CurrentMetrics map[string]interface{} `json:"currentMetrics,omitempty" bson:"currentMetrics,omitempty"`
}
