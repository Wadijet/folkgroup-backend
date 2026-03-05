package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsActivityHistory lưu lịch sử thay đổi currentMetrics (Campaign/AdSet/Ad).
// Khi currentMetrics thay đổi, ghi snapshotChanges (field, oldValue, newValue).
type AdsActivityHistory struct {
	ID                   primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	ActivityType         string                 `json:"activityType" bson:"activityType" index:"single:1"`
	AdAccountId          string                 `json:"adAccountId" bson:"adAccountId" index:"single:1,compound:ads_activity_by_entity"`
	ObjectType           string                 `json:"objectType" bson:"objectType" index:"single:1,compound:ads_activity_by_entity"`
	ObjectId             string                 `json:"objectId" bson:"objectId" index:"single:1,compound:ads_activity_by_entity"`
	OwnerOrganizationID  primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:ads_activity_by_entity"`
	ActivityAt           int64                  `json:"activityAt" bson:"activityAt" index:"single:-1,compound:ads_activity_by_entity,order:-1"`
	Metadata             map[string]interface{} `json:"metadata" bson:"metadata"`
	CreatedAt            int64                  `json:"createdAt" bson:"createdAt"`
}

// Metadata.MetricsSnapshot currentMetrics tại thời điểm.
// Metadata.SnapshotChanges []{field, oldValue, newValue}.
// Metadata.Trigger meta_ad_insights | pc_pos_orders | manual.
const (
	AdsActivityTypeMetricsChanged = "metrics_changed"
)
