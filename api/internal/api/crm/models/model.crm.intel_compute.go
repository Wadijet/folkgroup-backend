package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmIntelComputeJob — job tính CRM Intelligence (RefreshMetrics, Recalculate*, classification_refresh).
// Consumer AI Decision chỉ enqueue; worker domain CRM poll và thực thi.
type CrmIntelComputeJob struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	Payload map[string]interface{} `json:"payload" bson:"payload"`

	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`

	ParentDecisionEventID string `json:"parentDecisionEventId,omitempty" bson:"parentDecisionEventId,omitempty"`

	ProcessedAt  *int64 `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:customer_intel_compute_poll"`
	ProcessError string `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount   int    `json:"retryCount" bson:"retryCount"`
	CreatedAt    int64  `json:"createdAt" bson:"createdAt" index:"single:1,compound:customer_intel_compute_poll,order:1"`
}
