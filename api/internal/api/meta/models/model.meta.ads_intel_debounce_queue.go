package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// RecomputeDebounceQueue lưu trạng thái giảm chấn tính lại theo entity.
// Dùng làm hàng đợi đệm trước queue domain, hỗ trợ dùng chung nhiều domain.
type RecomputeDebounceQueue struct {
	ID               primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	DebounceKey      string             `json:"debounceKey" bson:"debounceKey" index:"unique:1"`
	OwnerOrgID       primitive.ObjectID `json:"ownerOrgId" bson:"ownerOrgId" index:"single:1,compound:ads_intel_debounce_entity"`
	AdAccountID      string             `json:"adAccountId" bson:"adAccountId" index:"single:1,compound:ads_intel_debounce_entity"`
	RecalcObjectType string             `json:"recalcObjectType" bson:"recalcObjectType" index:"single:1,compound:ads_intel_debounce_entity"`
	RecalcObjectID   string             `json:"recalcObjectId" bson:"recalcObjectId" index:"single:1,compound:ads_intel_debounce_entity"`
	Status           string             `json:"status" bson:"status" index:"single:1"` // scheduled|emitted|emit_failed
	UrgentRequested  bool               `json:"urgentRequested" bson:"urgentRequested"`
	MinIntervalMs    int64              `json:"minIntervalMs" bson:"minIntervalMs"`
	TrailingMs       int64              `json:"trailingMs" bson:"trailingMs"`
	FirstSeenAt      int64              `json:"firstSeenAt" bson:"firstSeenAt"`
	LastSeenAt       int64              `json:"lastSeenAt" bson:"lastSeenAt" index:"single:-1"`
	NextEmitAt       int64              `json:"nextEmitAt" bson:"nextEmitAt" index:"single:1"`
	LastEmitAt       int64              `json:"lastEmitAt" bson:"lastEmitAt" index:"single:-1"`
	LastEmitEventID  string             `json:"lastEmitEventId,omitempty" bson:"lastEmitEventId,omitempty"`
	LastEmitStatus   string             `json:"lastEmitStatus,omitempty" bson:"lastEmitStatus,omitempty"`
	LastEmitError    string             `json:"lastEmitError,omitempty" bson:"lastEmitError,omitempty"`
	EmitCount        int                `json:"emitCount" bson:"emitCount"`
	SourceKinds      []string           `json:"sourceKinds,omitempty" bson:"sourceKinds,omitempty"`
	UpdatedAt        int64              `json:"updatedAt" bson:"updatedAt" index:"single:-1"`
	CreatedAt        int64              `json:"createdAt" bson:"createdAt"`
}

