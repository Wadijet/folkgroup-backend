// Package dto — DTO cho module CIX (Contextual Conversation Intelligence).
package dto

// AnalyzeSessionRequest request phân tích session.
type AnalyzeSessionRequest struct {
	SessionUid  string `json:"sessionUid"`  // sess_xxx hoặc conversationId (Messenger: 1 conv = 1 session)
	CustomerUid string `json:"customerUid"`  // cust_xxx — tùy chọn, dùng để lấy customer context
}

// CixAnalysisResponse response kết quả phân tích — theo schema vision.
type CixAnalysisResponse struct {
	ID                 string       `json:"id,omitempty"`
	SessionUid         string       `json:"sessionUid"`
	CustomerUid        string       `json:"customerUid,omitempty"`
	TraceID            string       `json:"traceId,omitempty"`
	Layer1             CixLayer1DTO `json:"layer1"`
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
