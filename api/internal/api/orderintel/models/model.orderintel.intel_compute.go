// Package models — Hàng đợi domain tính Order Intelligence (Raw→L1→L2→L3→Flags), không tính trong consumer AI Decision.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrderIntelComputeJob — job trong order_intel_compute — worker domain poll và tính toán.
type OrderIntelComputeJob struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OrderUid            string             `json:"orderUid" bson:"orderUid" index:"compound:order_intel_compute_uid_org"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:order_intel_compute_uid_org"`
	MongoRecordIdHex    string             `json:"mongoRecordIdHex,omitempty" bson:"mongoRecordIdHex,omitempty" index:"compound:order_intel_compute_mongo_org"`
	NormalizedRecordUid string             `json:"normalizedRecordUid,omitempty" bson:"normalizedRecordUid,omitempty"`
	OrgID               string             `json:"orgId,omitempty" bson:"orgId,omitempty"`
	TraceID             string             `json:"traceId,omitempty" bson:"traceId,omitempty"`
	CorrelationID       string             `json:"correlationId,omitempty" bson:"correlationId,omitempty"`
	ParentEventID       string             `json:"parentEventId,omitempty" bson:"parentEventId,omitempty"`
	ParentEventType     string             `json:"parentEventType,omitempty" bson:"parentEventType,omitempty"`
	// EventType / EventSource / PipelineStage — bản sao envelope decision_events_queue (tùy chọn).
	EventType     string `json:"eventType,omitempty" bson:"eventType,omitempty"`
	EventSource   string `json:"eventSource,omitempty" bson:"eventSource,omitempty"`
	PipelineStage string `json:"pipelineStage,omitempty" bson:"pipelineStage,omitempty"`
	OwnerDomain           string `json:"ownerDomain,omitempty" bson:"ownerDomain,omitempty"`
	ProcessorDomain       string `json:"processorDomain,omitempty" bson:"processorDomain,omitempty"`
	EnqueueSourceDomain   string `json:"enqueueSourceDomain,omitempty" bson:"enqueueSourceDomain,omitempty"`
	E2EStage              string `json:"e2eStage,omitempty" bson:"e2eStage,omitempty"`
	E2EStepID             string `json:"e2eStepId,omitempty" bson:"e2eStepId,omitempty"`
	Source        string `json:"source" bson:"source"`
	ProcessedAt   *int64 `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:order_intel_compute_poll"`
	ProcessError  string `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount    int    `json:"retryCount" bson:"retryCount"`
	// CausalOrderingAtMs — copy từ payload event (causalOrderingAtMs) hoặc gán lúc enqueue; sort lịch sử intel đơn.
	CausalOrderingAtMs int64 `json:"causalOrderingAtMs,omitempty" bson:"causalOrderingAtMs,omitempty"`
	CreatedAt          int64 `json:"createdAt" bson:"createdAt" index:"single:1,compound:order_intel_compute_poll,order:1"`
}
