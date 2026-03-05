package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MetaAdInsight lưu insights (hiệu suất) từ Meta Insights API.
// Mỗi document = 1 ngày cho 1 object (ad account, campaign, adset, hoặc ad).
// ObjectId, ObjectType, AdAccountId do sync logic truyền (phụ thuộc level). Các metric có extract tag.
type MetaAdInsight struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	ObjectId            string                 `json:"objectId" bson:"objectId" index:"text"`       // act_xxx, campaign_id, adset_id, ad_id (truyền từ sync)
	ObjectType          string                 `json:"objectType" bson:"objectType"`                // ad_account, campaign, adset, ad (truyền từ sync)
	AdAccountId         string                 `json:"adAccountId" bson:"adAccountId" index:"text"` // truyền từ sync
	DateStart           string                 `json:"dateStart" bson:"dateStart" extract:"metaData\\.date_start,converter=string,optional"`
	DateStop            string                 `json:"dateStop" bson:"dateStop" extract:"metaData\\.date_stop,converter=string,optional"`
	Impressions         string                 `json:"impressions" bson:"impressions" extract:"metaData\\.impressions,converter=string,optional"`
	Clicks              string                 `json:"clicks" bson:"clicks" extract:"metaData\\.clicks,converter=string,optional"`
	Spend               string                 `json:"spend" bson:"spend" extract:"metaData\\.spend,converter=string,optional"`
	Reach               string                 `json:"reach" bson:"reach" extract:"metaData\\.reach,converter=string,optional"`
	Cpc                 string                 `json:"cpc" bson:"cpc" extract:"metaData\\.cpc,converter=string,optional"`
	Cpm                 string                 `json:"cpm" bson:"cpm" extract:"metaData\\.cpm,converter=string,optional"`
	Ctr                 string                 `json:"ctr" bson:"ctr" extract:"metaData\\.ctr,converter=string,optional"`
	MetaData            map[string]interface{} `json:"metaData" bson:"metaData"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64                  `json:"updatedAt" bson:"updatedAt"`
}
