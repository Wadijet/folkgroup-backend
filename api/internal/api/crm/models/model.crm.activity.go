// Package models - CrmActivityHistory thuộc domain CRM (crm_activity_history).
// Lưu lịch sử hoạt động của khách hàng: order, conversation, note, ...
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmActivityHistory lưu lịch sử hoạt động khách (crm_activity_history).
type CrmActivityHistory struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	UnifiedId           string                 `json:"unifiedId" bson:"unifiedId" index:"single:1,compound:crm_activity_org_unified_at"`
	OwnerOrganizationID primitive.ObjectID    `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:crm_activity_org_unified_at"`
	ActivityType       string                 `json:"activityType" bson:"activityType"` // order_created, order_completed, conversation_started, note_added, ...
	ActivityAt         int64                  `json:"activityAt" bson:"activityAt" index:"single:-1,compound:crm_activity_org_unified_at"`
	Source             string                 `json:"source" bson:"source"`             // pos | fb | system
	SourceRef          map[string]interface{} `json:"sourceRef,omitempty" bson:"sourceRef,omitempty"` // orderId, conversationId, ...
	Metadata            map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"`   // amount, channel, ...

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
}
