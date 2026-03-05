// Package models — Model cho module ads.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsApprovalConfig cấu hình duyệt theo ad account. Tách khỏi meta_ad_accounts.
type AdsApprovalConfig struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	AdAccountId         string                 `json:"adAccountId" bson:"adAccountId" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	ApprovalConfig      map[string]interface{} `json:"approvalConfig" bson:"approvalConfig"`
	CreatedAt           int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64                  `json:"updatedAt" bson:"updatedAt"`
}
