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

	// EventType / EventSource / PipelineStage — bản sao envelope decision_events_queue (tùy chọn, audit E2E).
	EventType     string `json:"eventType,omitempty" bson:"eventType,omitempty"`
	EventSource   string `json:"eventSource,omitempty" bson:"eventSource,omitempty"`
	PipelineStage string `json:"pipelineStage,omitempty" bson:"pipelineStage,omitempty"`
	// OwnerDomain / ProcessorDomain / EnqueueSourceDomain — chuẩn gộp event một nguồn (payload vs worker vs nơi enqueue).
	OwnerDomain           string `json:"ownerDomain,omitempty" bson:"ownerDomain,omitempty"`
	ProcessorDomain       string `json:"processorDomain,omitempty" bson:"processorDomain,omitempty"`
	EnqueueSourceDomain   string `json:"enqueueSourceDomain,omitempty" bson:"enqueueSourceDomain,omitempty"`
	E2EStage              string `json:"e2eStage,omitempty" bson:"e2eStage,omitempty"`
	E2EStepID             string `json:"e2eStepId,omitempty" bson:"e2eStepId,omitempty"`

	ProcessedAt  *int64 `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:customer_intel_compute_poll"`
	ProcessError string `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount   int    `json:"retryCount" bson:"retryCount"`
	CreatedAt    int64  `json:"createdAt" bson:"createdAt" index:"single:1,compound:customer_intel_compute_poll,order:1"`
}
