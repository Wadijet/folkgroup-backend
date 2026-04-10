package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Loại job trong collection ads_intel_compute (domain ads — worker poll, không tính trong consumer AI Decision).
const (
	AdsIntelComputeKindRecomputeOne   = "recompute_one"
	AdsIntelComputeKindRecalculateAll = "recalculate_all"
	// AdsIntelComputeKindContextReady — đọc snapshot Intelligence từ meta_campaigns + emit ads.context_ready (chỉ worker domain ads).
	AdsIntelComputeKindContextReady = "context_ready"
)

// AdsIntelComputeJob — job tính Ads Intelligence (ApplyAdsIntelligenceRecompute / RecalculateAll).
type AdsIntelComputeJob struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	JobKind string `json:"jobKind" bson:"jobKind" index:"single:1"` // recompute_one | recalculate_all | context_ready

	ObjectType    string `json:"objectType,omitempty" bson:"objectType,omitempty"`
	ObjectID      string `json:"objectId,omitempty" bson:"objectId,omitempty"`
	AdAccountID   string `json:"adAccountId,omitempty" bson:"adAccountId,omitempty"`
	Source        string `json:"source,omitempty" bson:"source,omitempty"`
	RecomputeMode string `json:"recomputeMode,omitempty" bson:"recomputeMode,omitempty"`

	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`

	ParentDecisionEventID string `json:"parentDecisionEventId,omitempty" bson:"parentDecisionEventId,omitempty"`

	// ParentTraceID / ParentCorrelationID — nối timeline decisionlive khi worker ads chạy (recompute_one / recalculate_all).
	ParentTraceID       string `json:"parentTraceId,omitempty" bson:"parentTraceId,omitempty"`
	ParentCorrelationID string `json:"parentCorrelationId,omitempty" bson:"parentCorrelationId,omitempty"`

	// CausalOrderingAtMs — mốc nghiệp vụ (payload causalOrderingAtMs từ event hoặc gán lúc enqueue); sort lịch sử intel khi worker không FIFO.
	CausalOrderingAtMs int64 `json:"causalOrderingAtMs,omitempty" bson:"causalOrderingAtMs,omitempty"`

	// ContextReady: bản ghi emit ads.context_ready (jobKind=context_ready) — consumer không đọc meta_campaigns.
	ContextEmitOrgID         string `json:"contextEmitOrgId,omitempty" bson:"contextEmitOrgId,omitempty"`
	ContextEmitTraceID       string `json:"contextEmitTraceId,omitempty" bson:"contextEmitTraceId,omitempty"`
	ContextEmitCorrelationID string `json:"contextEmitCorrelationId,omitempty" bson:"contextEmitCorrelationId,omitempty"`

	RecalculateAllLimit int `json:"recalculateAllLimit,omitempty" bson:"recalculateAllLimit,omitempty"`

	ProcessedAt  *int64 `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:ads_intel_compute_poll"`
	ProcessError string `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount   int    `json:"retryCount" bson:"retryCount"`
	CreatedAt    int64  `json:"createdAt" bson:"createdAt" index:"single:1,compound:ads_intel_compute_poll,order:1"`
}
