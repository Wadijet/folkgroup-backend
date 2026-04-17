// Package models — Job trong collection cix_intel_compute (Raw→L1→L2→L3→Flag→Action, worker domain CIX).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CixIntelComputeJob document trong cix_intel_compute — cùng quy ước {domain}_intel_compute với CRM/Ads/Order.
type CixIntelComputeJob struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	ConversationID      string             `json:"conversationId" bson:"conversationId" index:"single:1"`
	CustomerID          string             `json:"customerId" bson:"customerId"`
	Channel             string             `json:"channel" bson:"channel"`
	CioEventUid         string             `json:"cioEventUid" bson:"cioEventUid"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:cix_intel_compute_poll"`
	// TraceID / CorrelationID — copy từ envelope AI Decision (vào AnalyzeSession / lớp A).
	TraceID       string `json:"traceId,omitempty" bson:"traceId,omitempty"`
	CorrelationID string `json:"correlationId,omitempty" bson:"correlationId,omitempty"`
	// CausalOrderingAtMs — mốc nghiệp vụ (unix ms), đồng tên payload CRM causalOrderingAtMs.
	CausalOrderingAtMs int64 `json:"causalOrderingAtMs,omitempty" bson:"causalOrderingAtMs,omitempty"`
	// DecisionEventID — eventId decision_events_queue đã enqueue job (audit).
	DecisionEventID string `json:"decisionEventId,omitempty" bson:"decisionEventId,omitempty"`
	// EventType / EventSource / PipelineStage — bản sao envelope bus AID (tùy chọn).
	EventType     string `json:"eventType,omitempty" bson:"eventType,omitempty"`
	EventSource   string `json:"eventSource,omitempty" bson:"eventSource,omitempty"`
	PipelineStage string `json:"pipelineStage,omitempty" bson:"pipelineStage,omitempty"`
	OwnerDomain           string `json:"ownerDomain,omitempty" bson:"ownerDomain,omitempty"`
	ProcessorDomain       string `json:"processorDomain,omitempty" bson:"processorDomain,omitempty"`
	EnqueueSourceDomain   string `json:"enqueueSourceDomain,omitempty" bson:"enqueueSourceDomain,omitempty"`
	E2EStage              string `json:"e2eStage,omitempty" bson:"e2eStage,omitempty"`
	E2EStepID             string `json:"e2eStepId,omitempty" bson:"e2eStepId,omitempty"`
	ProcessedAt   *int64 `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:cix_intel_compute_poll"`
	ProcessError  string `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount    int    `json:"retryCount" bson:"retryCount"`
	CreatedAt     int64  `json:"createdAt" bson:"createdAt" index:"single:-1,compound:cix_intel_compute_poll,order:1"`
}
