// Package models — Hàng đợi domain tính Order Intelligence (Raw→L1→L2→L3→Flags), không tính trong consumer AI Decision.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrderIntelligencePendingJob job trong order_intelligence_pending — worker domain poll và tính toán.
type OrderIntelligencePendingJob struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OrderUid            string             `json:"orderUid" bson:"orderUid" index:"compound:order_intel_pending_uid_org"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:order_intel_pending_uid_org"`
	// MongoRecordIdHex dùng khi chưa có orderUid (lookup pc_pos_orders theo _id).
	MongoRecordIdHex    string             `json:"mongoRecordIdHex,omitempty" bson:"mongoRecordIdHex,omitempty" index:"compound:order_intel_pending_mongo_org"`
	NormalizedRecordUid string             `json:"normalizedRecordUid,omitempty" bson:"normalizedRecordUid,omitempty"`
	OrgID               string             `json:"orgId,omitempty" bson:"orgId,omitempty"`
	TraceID             string             `json:"traceId,omitempty" bson:"traceId,omitempty"`
	CorrelationID       string             `json:"correlationId,omitempty" bson:"correlationId,omitempty"`
	ParentEventID       string             `json:"parentEventId,omitempty" bson:"parentEventId,omitempty"`
	ParentEventType     string             `json:"parentEventType,omitempty" bson:"parentEventType,omitempty"`
	// Source: aidecision_order | recompute | legacy_intelligence_requested
	Source      string `json:"source" bson:"source"`
	ProcessedAt *int64 `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:order_intel_poll"`
	ProcessError string `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount  int    `json:"retryCount" bson:"retryCount"`
	CreatedAt   int64  `json:"createdAt" bson:"createdAt" index:"single:1,compound:order_intel_poll,order:1"`
}
