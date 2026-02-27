// Package models - CrmActivityHistory thuộc domain CRM (crm_activity_history).
// Lưu lịch sử hoạt động của khách hàng: order, conversation, note, ...
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ActivityChangeItem mô tả một thay đổi (field, oldValue, newValue).
type ActivityChangeItem struct {
	Field    string      `json:"field" bson:"field"`
	OldValue interface{} `json:"oldValue,omitempty" bson:"oldValue,omitempty"`
	NewValue interface{} `json:"newValue,omitempty" bson:"newValue,omitempty"`
}

// CrmActivityHistory lưu lịch sử hoạt động khách (crm_activity_history).
type CrmActivityHistory struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	UnifiedId           string                 `json:"unifiedId" bson:"unifiedId" index:"single:1,compound:crm_activity_org_unified_at,compound:crm_activity_org_unified_type,compound:crm_activity_org_unified_domain_at"`
	OwnerOrganizationID primitive.ObjectID    `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:crm_activity_org_unified_at,compound:crm_activity_org_type_at,compound:crm_activity_org_source_at,compound:crm_activity_org_unified_type,compound:crm_activity_org_unified_domain_at,compound:crm_activity_org_snapshot_at,compound:crm_activity_org_at_report,compound:crm_activity_org_created"`
	Domain              string                 `json:"domain" bson:"domain" index:"single:1,compound:crm_activity_org_unified_domain_at"` // order, conversation, note, profile, customer, ...
	ActivityType       string                 `json:"activityType" bson:"activityType" index:"single:1,compound:crm_activity_org_type_at,compound:crm_activity_org_source_at,compound:crm_activity_org_unified_type"`
	ActivityAt         int64                  `json:"activityAt" bson:"activityAt" index:"single:-1,compound:crm_activity_org_unified_at,compound:crm_activity_org_type_at,compound:crm_activity_org_source_at,compound:crm_activity_org_unified_domain_at,compound:crm_activity_org_at_report,order:-1"`
	Source             string                 `json:"source" bson:"source" index:"single:1,compound:crm_activity_org_source_at"` // pos | fb | system
	SourceRef          map[string]interface{} `json:"sourceRef,omitempty" bson:"sourceRef,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`
	// MetadataSnapshotAt chỉ dùng cho index (metadata.snapshotAt). Giá trị thực nằm trong Metadata["snapshotAt"].
	MetadataSnapshotAt int64 `json:"-" bson:"metadata.snapshotAt,omitempty" index:"compound:crm_activity_org_snapshot_at,order:-1"`

	DisplayLabel   string `json:"displayLabel,omitempty" bson:"displayLabel,omitempty"`
	DisplayIcon    string `json:"displayIcon,omitempty" bson:"displayIcon,omitempty"`
	DisplaySubtext string `json:"displaySubtext,omitempty" bson:"displaySubtext,omitempty"`
	ActorId        *primitive.ObjectID `json:"actorId,omitempty" bson:"actorId,omitempty"` // nil = không lưu (omitempty)
	ActorName      string             `json:"actorName,omitempty" bson:"actorName,omitempty"`
	Changes        []ActivityChangeItem `json:"changes,omitempty" bson:"changes,omitempty"`
	Reason         string             `json:"reason,omitempty" bson:"reason,omitempty"`
	ClientIp       string             `json:"clientIp,omitempty" bson:"clientIp,omitempty"`
	UserAgent      string             `json:"userAgent,omitempty" bson:"userAgent,omitempty"`
	Status         string             `json:"status,omitempty" bson:"status,omitempty"` // success | failed

	CreatedAt int64 `json:"createdAt" bson:"createdAt" index:"single:-1,compound:crm_activity_org_created,order:-1"`
}
