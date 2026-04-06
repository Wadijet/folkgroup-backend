// Package models — snapshot metrics/profile khách CRM lưu riêng (crm_customer_metric_snapshots).
// Tách khỏi crm_activity_history theo KHUNG_KHUON_MODULE_INTELLIGENCE: activity giữ metricSnapshotId, payload lớn ở collection này.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerMetricSnapshot một bản ghi snapshot tại activityAt, gắn với một dòng activity (activityHistoryId).
type CrmCustomerMetricSnapshot struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId"`
	UnifiedId           string                 `json:"unifiedId" bson:"unifiedId"`
	ActivityHistoryID   primitive.ObjectID     `json:"activityHistoryId,omitempty" bson:"activityHistoryId,omitempty"`
	ActivityAt          int64                  `json:"activityAt" bson:"activityAt"`
	Profile             map[string]interface{} `json:"profile,omitempty" bson:"profile,omitempty"`
	Metrics             map[string]interface{} `json:"metrics,omitempty" bson:"metrics,omitempty"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`
}
