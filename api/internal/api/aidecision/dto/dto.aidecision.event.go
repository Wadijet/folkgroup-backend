// Package dto — DTO cho API AI Decision events.
package dto

// IngestEventRequest request cho POST /ai-decision/events.
type IngestEventRequest struct {
	EventType     string                 `json:"eventType" validate:"required"`
	EventSource   string                 `json:"eventSource" validate:"required"`
	PipelineStage string                 `json:"pipelineStage,omitempty"`
	EntityType    string                 `json:"entityType" validate:"required"`
	EntityID      string                 `json:"entityId" validate:"required"`
	OrgID         string                 `json:"orgId" validate:"required"`
	Priority      string                 `json:"priority"` // high | normal | low, default normal
	Lane          string                 `json:"lane"`     // fast | normal | batch, default từ eventType
	TraceID       string                 `json:"traceId,omitempty"`
	CorrelationID string                 `json:"correlationId,omitempty"`
	Payload       map[string]interface{} `json:"payload"`
}

// IngestEventResponse response cho POST /ai-decision/events.
type IngestEventResponse struct {
	EventID string `json:"eventId"`
	Status  string `json:"status"`
	// W3CTraceID trace-id W3C (32 hex) — đồng bộ với envelope queue khi có traceId.
	W3CTraceID string `json:"w3cTraceId,omitempty"`
	// OpsTier / OpsTierLabelVi — phân loại vận hành theo eventType (cùng logic org-live).
	OpsTier        string `json:"opsTier"`
	OpsTierLabelVi string `json:"opsTierLabelVi"`
}
