// Package models — DebounceState cho gom message trước khi emit message.batch_ready.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §2.6.1 Debounce Key Design.
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// DebounceState document trong decision_debounce_state.
type DebounceState struct {
	DebounceKey    string             `json:"debounceKey" bson:"debounceKey" index:"unique:1"`
	OrgID          string             `json:"orgId" bson:"orgId" index:"single:1"`
	OwnerOrgID     primitive.ObjectID `json:"ownerOrgId" bson:"ownerOrgId"`
	ConversationID string             `json:"conversationId" bson:"conversationId"`
	CustomerID     string             `json:"customerId" bson:"customerId"`
	Channel        string             `json:"channel" bson:"channel"`
	LastEventID    string             `json:"lastEventId" bson:"lastEventId"`
	// TraceID / CorrelationID — lấy từ event đầu tiên trong cửa sổ debounce ($setOnInsert), nối lại khi emit message.batch_ready.
	TraceID       string `json:"traceId,omitempty" bson:"traceId,omitempty"`
	CorrelationID string `json:"correlationId,omitempty" bson:"correlationId,omitempty"`
	LastMessageAt int64  `json:"lastMessageAt" bson:"lastMessageAt" index:"single:1"`
	CreatedAt     int64  `json:"createdAt" bson:"createdAt"`
}
