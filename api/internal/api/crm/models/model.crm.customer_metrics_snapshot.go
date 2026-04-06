// Package models — bản ghi snapshot metrics/profile CRM tách khỏi crm_activity_history.
// Collection: crm_customer_metrics_snapshots (một document / activity có snapshot; khóa logic activityHistoryId).
package models

import (
	"meta_commerce/internal/common/activity"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerMetricsSnapshot lưu profileSnapshot + metricsSnapshot tại activityAt (raw/layer1/layer2/layer3).
// Tham chiếu activity qua ActivityHistoryID; không nhúng vào crm_activity_history để giảm kích thước document activity.
type CrmCustomerMetricsSnapshot struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId"`
	UnifiedId           string                 `json:"unifiedId" bson:"unifiedId"`
	ActivityHistoryID   primitive.ObjectID     `json:"activityHistoryId" bson:"activityHistoryId"`
	ActivityAt          int64                  `json:"activityAt" bson:"activityAt"`
	ActivityType        string                 `json:"activityType,omitempty" bson:"activityType,omitempty"`
	Source              string                 `json:"source,omitempty" bson:"source,omitempty"`
	SourceRef           map[string]interface{} `json:"sourceRef,omitempty" bson:"sourceRef,omitempty"`
	ProfileSnapshot     map[string]interface{} `json:"profileSnapshot,omitempty" bson:"profileSnapshot,omitempty"`
	MetricsSnapshot     map[string]interface{} `json:"metricsSnapshot,omitempty" bson:"metricsSnapshot,omitempty"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`
}

// ToActivitySnapshot chuyển sang activity.Snapshot (đọc API / so sánh).
func (m *CrmCustomerMetricsSnapshot) ToActivitySnapshot() activity.Snapshot {
	if m == nil {
		return activity.Snapshot{}
	}
	return activity.Snapshot{Profile: m.ProfileSnapshot, Metrics: m.MetricsSnapshot}
}
