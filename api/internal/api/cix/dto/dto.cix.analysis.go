// Package dto — DTO cho module CIX (Contextual Conversation Intelligence).
package dto

// AnalyzeSessionRequest request phân tích session.
type AnalyzeSessionRequest struct {
	SessionUid  string `json:"sessionUid"`  // sess_xxx hoặc conversationId (Messenger: 1 conv = 1 session)
	CustomerUid string `json:"customerUid"` // cust_xxx — tùy chọn, dùng để lấy customer context
	Channel     string `json:"channel,omitempty"` // mặc định messenger — đồng bộ payload cix.analysis_requested
}

// CixRawFactsDTO tóm tắt facts đầu vào đã lưu lớp A.
type CixRawFactsDTO struct {
	TurnCount  int   `json:"turnCount"`
	FirstMsgAt int64 `json:"firstMsgAt,omitempty"`
	LastMsgAt  int64 `json:"lastMsgAt,omitempty"`
}

// CixAnalysisResponse response kết quả phân tích — theo schema vision.
type CixAnalysisResponse struct {
	ID                 string       `json:"id,omitempty"`
	SessionUid         string       `json:"sessionUid"`
	CustomerUid        string       `json:"customerUid,omitempty"`
	TraceID              string   `json:"traceId,omitempty"`
	CorrelationID        string   `json:"correlationId,omitempty"`
	Status               string   `json:"status,omitempty"`
	ComputedAt           int64    `json:"computedAt,omitempty"`
	FailedAt             int64    `json:"failedAt,omitempty"`
	ErrorCode            string   `json:"errorCode,omitempty"`
	ErrorMessage         string   `json:"errorMessage,omitempty"`
	ParentJobID          string   `json:"parentJobId,omitempty"`
	CausalOrderingAt     int64    `json:"causalOrderingAt,omitempty"`
	CixIntelSequence     int64    `json:"cixIntelSequence,omitempty"`
	RawFacts             CixRawFactsDTO `json:"rawFacts,omitempty"`
	PipelineRuleTraceIDs []string `json:"pipelineRuleTraceIds,omitempty"`
	Layer1               CixLayer1DTO `json:"layer1"`
	Layer2             CixLayer2DTO `json:"layer2"`
	Layer3             CixLayer3DTO `json:"layer3"`
	Flags              []CixFlagDTO `json:"flags"`
	ActionSuggestions  []string     `json:"actionSuggestions"`
	CreatedAt          int64        `json:"createdAt"`
}

// CixLayer1DTO Layer 1 — Conversation Stage.
type CixLayer1DTO struct {
	Stage string `json:"stage"` // new | engaged | consulting | negotiating | waiting | stalled
}

// CixLayer2DTO Layer 2 — Conversation State.
type CixLayer2DTO struct {
	IntentStage    string `json:"intentStage"`
	UrgencyLevel   string `json:"urgencyLevel"`
	RiskLevelRaw   string `json:"riskLevelRaw"`
	RiskLevelAdj   string `json:"riskLevelAdj"`
	AdjustmentRule string `json:"adjustmentRule,omitempty"`
	AdjustmentReason string `json:"adjustmentReason,omitempty"`
}

// CixLayer3DTO Layer 3 — Micro Signals.
type CixLayer3DTO struct {
	BuyingIntent   string `json:"buyingIntent"`
	ObjectionLevel string `json:"objectionLevel"`
	Sentiment      string `json:"sentiment"`
}

// CixFlagDTO cờ báo.
type CixFlagDTO struct {
	Name           string `json:"name"`
	Severity       string `json:"severity"`
	TriggeredByRule string `json:"triggeredByRule"`
}
