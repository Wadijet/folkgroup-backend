// Package models — DecisionEvent cho hàng đợi AI Decision.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §2.3 Event Envelope.
package models

import (
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DecisionEvent document trong decision_events_queue — event chờ AI Decision xử lý.
type DecisionEvent struct {
	ID        primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	EventID   string                 `json:"eventId" bson:"eventId" index:"unique:1"` // evt_xxx
	EventType string                 `json:"eventType" bson:"eventType" index:"single:1"`
	EventSource string               `json:"eventSource" bson:"eventSource" index:"single:1"`
	// PipelineStage giai đoạn trong khung quy trình tổng (ingress → merge → intel → AID) — xem eventtypes.PipelineStage*.
	PipelineStage string `json:"pipelineStage,omitempty" bson:"pipelineStage,omitempty" index:"single:1,sparse"`
	// E2EStage / E2EStepID — tham chiếu luồng chuẩn G1–G6 (docs/flows/bang-pha-buoc-event-e2e.md); gán khi emit.
	E2EStage       string `json:"e2eStage,omitempty" bson:"e2eStage,omitempty" index:"single:1,sparse"`
	E2EStepID      string `json:"e2eStepId,omitempty" bson:"e2eStepId,omitempty"`
	E2EStepLabelVi string `json:"e2eStepLabelVi,omitempty" bson:"e2eStepLabelVi,omitempty"`
	EntityType string                `json:"entityType" bson:"entityType"`
	EntityID  string                 `json:"entityId" bson:"entityId"`
	OrgID     string                 `json:"orgId" bson:"orgId" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`

	Priority string `json:"priority" bson:"priority" index:"single:1"` // high | normal | low
	// PriorityRank số để sort lease (1=ưu tiên cao nhất); tránh sort lexicographic trên chuỗi priority.
	PriorityRank int `json:"priorityRank" bson:"priorityRank" index:"single:1"`
	Lane     string `json:"lane" bson:"lane" index:"single:1"`         // fast | normal | batch

	Status string `json:"status" bson:"status" index:"single:1"` // pending | leased | processing | completed | completed_no_handler | completed_routing_skipped | failed_retryable | failed_terminal | deferred

	ParentEventID   string `json:"parentEventId,omitempty" bson:"parentEventId,omitempty"`
	RootEventID     string `json:"rootEventId,omitempty" bson:"rootEventId,omitempty"`
	CausationEventID string `json:"causationEventId,omitempty" bson:"causationEventId,omitempty"`

	TraceID      string `json:"traceId,omitempty" bson:"traceId,omitempty"`
	// W3CTraceID trace-id W3C (32 hex) — đồng bộ với decisionlive / traceutil; bù từ traceId nếu thiếu khi consume.
	W3CTraceID    string `json:"w3cTraceId,omitempty" bson:"w3cTraceId,omitempty" index:"single:1,sparse"`
	CorrelationID string `json:"correlationId,omitempty" bson:"correlationId,omitempty"`

	Payload map[string]interface{} `json:"payload" bson:"payload"`

	ScheduledAt *int64 `json:"scheduledAt,omitempty" bson:"scheduledAt,omitempty" index:"single:1"`
	AttemptCount int   `json:"attemptCount" bson:"attemptCount"`
	MaxAttempts  int   `json:"maxAttempts" bson:"maxAttempts"`

	LeasedBy   string `json:"leasedBy,omitempty" bson:"leasedBy,omitempty"`
	LeasedUntil *int64 `json:"leasedUntil,omitempty" bson:"leasedUntil,omitempty"`

	Error     string `json:"error,omitempty" bson:"error,omitempty"`
	CreatedAt int64  `json:"createdAt" bson:"createdAt" index:"single:-1"`
}

// Event status constants
const (
	EventStatusPending         = "pending"
	EventStatusLeased          = "leased"
	EventStatusProcessing      = "processing"
	EventStatusCompleted       = "completed"
	// EventStatusCompletedNoHandler — consumer đã đóng job nhưng chưa đăng ký handler cho event_type (chưa có logic xử lý).
	EventStatusCompletedNoHandler = "completed_no_handler"
	// EventStatusCompletedRoutingSkipped — rule routing noop: không dispatch handler đã đăng ký.
	EventStatusCompletedRoutingSkipped = "completed_routing_skipped"
	EventStatusFailedRetryable = "failed_retryable"
	EventStatusFailedTerminal  = "failed_terminal"
	EventStatusDeferred        = "deferred"
)

// ConsumerCompletionKind — chi tiết khi consumer đóng job thành công (metrics / phân tích).
type ConsumerCompletionKind string

const (
	ConsumerCompletionKindProcessed      ConsumerCompletionKind = "processed"
	ConsumerCompletionKindNoHandler      ConsumerCompletionKind = "no_handler"
	ConsumerCompletionKindRoutingSkipped ConsumerCompletionKind = "routing_skipped"
)

// Event lane constants
const (
	EventLaneFast   = "fast"
	EventLaneNormal = "normal"
	EventLaneBatch  = "batch"
)

// PriorityRankFromString map priority → rank cho sort Mongo (1 trước = xử lý trước).
func PriorityRankFromString(p string) int {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "high", "urgent":
		return 1
	case "low":
		return 3
	default:
		return 2 // normal, rỗng, hoặc không nhận diện
	}
}
