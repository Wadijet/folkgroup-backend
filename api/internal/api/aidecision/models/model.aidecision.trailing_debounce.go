// Package models — Trailing debounce datachanged + CRM intel sau ingest (một document / khóa, xóa khi flush).
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// TrailingDebounceSlot document trong decision_trailing_debounce.
// Bucket tách loại: datachanged_defer | crm_intel_after_ingest — trường còn lại tùy bucket (bson omitempty).
type TrailingDebounceSlot struct {
	DebounceKey string `json:"debounceKey" bson:"debounceKey" index:"unique:1"`
	Bucket      string `json:"bucket" bson:"bucket" index:"single:1"`
	DueAtMs     int64  `json:"dueAtMs" bson:"dueAtMs" index:"single:1"`
	OwnerOrgID  primitive.ObjectID `json:"ownerOrgId" bson:"ownerOrgId" index:"single:1"`

	// Bucket datachanged_defer
	DeferKind  string `json:"deferKind,omitempty" bson:"deferKind,omitempty"`
	OrgHex     string `json:"orgHex,omitempty" bson:"orgHex,omitempty"`
	SourceColl string `json:"sourceColl,omitempty" bson:"sourceColl,omitempty"`
	IDHex      string `json:"idHex,omitempty" bson:"idHex,omitempty"`
	TraceID    string `json:"traceId,omitempty" bson:"traceId,omitempty"`
	CorrelationID string `json:"correlationId,omitempty" bson:"correlationId,omitempty"`

	// Bucket crm_intel_after_ingest
	UnifiedID     string `json:"unifiedId,omitempty" bson:"unifiedId,omitempty"`
	ParentEventID string `json:"parentEventId,omitempty" bson:"parentEventId,omitempty"`
	CausalMs      int64  `json:"causalMs,omitempty" bson:"causalMs,omitempty"`

	CreatedAtMs int64 `json:"createdAtMs,omitempty" bson:"createdAtMs,omitempty"`
	UpdatedAtMs int64 `json:"updatedAtMs,omitempty" bson:"updatedAtMs,omitempty"`
}
