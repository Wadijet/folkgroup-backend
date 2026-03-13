package models

import (
	"meta_commerce/internal/common/activity"
)

// AdsActivityHistory lưu lịch sử thay đổi currentMetrics (Campaign/AdSet/Ad).
// Embed ActivityBase. Snapshot.metrics = currentMetrics; Changes = snapshotChanges.
type AdsActivityHistory struct {
	activity.ActivityBase `bson:",inline"`
	AdAccountId           string `json:"adAccountId" bson:"adAccountId"`
	ObjectType            string `json:"objectType" bson:"objectType"`
	ObjectId              string `json:"objectId" bson:"objectId"`
}

// AdsActivityTypeMetricsChanged activity type khi metrics thay đổi.
const AdsActivityTypeMetricsChanged = "metrics_changed"
