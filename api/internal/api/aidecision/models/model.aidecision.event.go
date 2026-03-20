// Package models — DecisionEvent cho hàng đợi AI Decision.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §2.3 Event Envelope.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DecisionEvent document trong decision_events_queue — event chờ AI Decision xử lý.
type DecisionEvent struct {
	ID        primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	EventID   string                 `json:"eventId" bson:"eventId" index:"unique:1"` // evt_xxx
	EventType string                 `json:"eventType" bson:"eventType" index:"single:1"`
	EventSource string               `json:"eventSource" bson:"eventSource" index:"single:1"`
	EntityType string                `json:"entityType" bson:"entityType"`
	EntityID  string                 `json:"entityId" bson:"entityId"`
	OrgID     string                 `json:"orgId" bson:"orgId" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`

	Priority string `json:"priority" bson:"priority" index:"single:1"` // high | normal | low
	Lane     string `json:"lane" bson:"lane" index:"single:1"`         // fast | normal | batch

	Status string `json:"status" bson:"status" index:"single:1"` // pending | leased | processing | completed | failed_retryable | failed_terminal | deferred

	ParentEventID   string `json:"parentEventId,omitempty" bson:"parentEventId,omitempty"`
	RootEventID     string `json:"rootEventId,omitempty" bson:"rootEventId,omitempty"`
	CausationEventID string `json:"causationEventId,omitempty" bson:"causationEventId,omitempty"`

	TraceID      string `json:"traceId,omitempty" bson:"traceId,omitempty"`
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
	EventStatusFailedRetryable = "failed_retryable"
	EventStatusFailedTerminal  = "failed_terminal"
	EventStatusDeferred        = "deferred"
)

// Event lane constants
const (
	EventLaneFast   = "fast"
	EventLaneNormal = "normal"
	EventLaneBatch  = "batch"
)
