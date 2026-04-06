// Package models — Model cho module CIX (Contextual Conversation Intelligence).
//
// CixAnalysisResult lưu kết quả phân tích hội thoại — Raw → L1 → L2 → L3 → Flag → Action.
// Theo docs-shared/architecture/vision/05 - cix-contextual-conversation-intelligence.md
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CixLayer1 Conversation Stage — cấu trúc hội thoại.
type CixLayer1 struct {
	Stage string `json:"stage" bson:"stage"` // new | engaged | consulting | negotiating | waiting | stalled
}

// CixLayer2 Conversation State — ý định, urgency, risk.
type CixLayer2 struct {
	IntentStage      string `json:"intentStage" bson:"intentStage"`           // low | medium | high
	UrgencyLevel     string `json:"urgencyLevel" bson:"urgencyLevel"`         // normal | high | critical
	RiskLevelRaw     string `json:"riskLevelRaw" bson:"riskLevelRaw"`         // safe | warning | danger
	RiskLevelAdj     string `json:"riskLevelAdj" bson:"riskLevelAdj"`         // adjusted theo Customer Context
	AdjustmentRule   string `json:"adjustmentRule,omitempty" bson:"adjustmentRule,omitempty"`
	AdjustmentReason string `json:"adjustmentReason,omitempty" bson:"adjustmentReason,omitempty"`
}

// CixLayer3 Micro Signals — NLP output.
type CixLayer3 struct {
	BuyingIntent   string `json:"buyingIntent" bson:"buyingIntent"`     // none | inquiring | ready_to_buy
	ObjectionLevel string `json:"objectionLevel" bson:"objectionLevel"` // none | soft_objection | hard_objection
	Sentiment      string `json:"sentiment" bson:"sentiment"`           // positive | neutral | negative | angry
}

// CixFlag cờ báo từ Rule Engine.
type CixFlag struct {
	Name          string `json:"name" bson:"name"`
	Severity      string `json:"severity" bson:"severity"` // critical | high | medium | low
	TriggeredByRule string `json:"triggeredByRule" bson:"triggeredByRule"`
}

// Trạng thái terminal bản ghi lớp A (cix_analysis_results) — khớp khung intelligence mục 3.
const (
	CixAnalysisStatusSuccess = "success"
	CixAnalysisStatusFailed  = "failed"
	CixAnalysisStatusSkipped = "skipped"
)

// CixRawFacts tóm tắt facts đầu vào pipeline (không lưu full transcript — tránh phình document).
type CixRawFacts struct {
	TurnCount  int   `json:"turnCount" bson:"turnCount"`
	FirstMsgAt int64 `json:"firstMsgAt,omitempty" bson:"firstMsgAt,omitempty"`
	LastMsgAt  int64 `json:"lastMsgAt,omitempty" bson:"lastMsgAt,omitempty"`
}

// CixAnalysisResult document lưu trong collection cix_analysis_results.
type CixAnalysisResult struct {
	ID                 primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	SessionUid         string             `json:"sessionUid" bson:"sessionUid" index:"single:1"`   // sess_xxx
	CustomerUid        string             `json:"customerUid" bson:"customerUid" index:"single:1"` // cust_xxx
	TraceID            string             `json:"traceId" bson:"traceId" index:"single:1,sparse"`
	CorrelationID      string             `json:"correlationId" bson:"correlationId" index:"single:1,sparse"`
	// PipelineRuleTraceIDs thứ tự các lần chạy rule trong pipeline CIX (L1→L2→Adj→[L3]→Flags→Actions) — tra rule_execution_logs.
	PipelineRuleTraceIDs []string `json:"pipelineRuleTraceIds,omitempty" bson:"pipelineRuleTraceIds,omitempty"`
	// Status — success | failed | skipped (mặc định rỗng coi như bản ghi cũ = success).
	Status       string `json:"status,omitempty" bson:"status,omitempty" index:"single:1,sparse"`
	ComputedAt   int64  `json:"computedAt,omitempty" bson:"computedAt,omitempty" index:"single:-1"`
	FailedAt     int64  `json:"failedAt,omitempty" bson:"failedAt,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty" bson:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty" bson:"errorMessage,omitempty"`
	// ParentJobID — _id document cix_intel_compute sinh ra bản ghi này.
	ParentJobID primitive.ObjectID `json:"parentJobId,omitempty" bson:"parentJobId,omitempty" index:"single:1,sparse"`
	// CausalOrderingAt — mốc nghiệp vụ (unix ms) để sort lịch sử khi queue không FIFO.
	CausalOrderingAt int64 `json:"causalOrderingAt,omitempty" bson:"causalOrderingAt,omitempty"`
	// CixIntelSequence — bản sao số thứ tự tại thời điểm ghi (sau $inc trên crm_customers.cixIntelSequence).
	CixIntelSequence int64 `json:"cixIntelSequence,omitempty" bson:"cixIntelSequence,omitempty"`
	// RawFacts — aggregate thô tối thiểu để audit / derive nhẹ (không thay transcript đầy đủ).
	RawFacts CixRawFacts `json:"rawFacts,omitempty" bson:"rawFacts,omitempty"`
	Layer1             CixLayer1          `json:"layer1" bson:"layer1"`
	Layer2             CixLayer2          `json:"layer2" bson:"layer2"`
	Layer3             CixLayer3          `json:"layer3" bson:"layer3"`
	Flags              []CixFlag          `json:"flags" bson:"flags"`
	ActionSuggestions   []string           `json:"actionSuggestions" bson:"actionSuggestions"` // assign_to_human_sale | prioritize_followup | ...
	CreatedAt          int64              `json:"createdAt" bson:"createdAt" index:"single:-1"`
}
