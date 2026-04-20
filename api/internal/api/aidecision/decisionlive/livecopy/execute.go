package livecopy

import (
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// BuildExecuteQueuedEvent — PhaseQueued sau EmitExecuteRequested.
func BuildExecuteQueuedEvent(
	sourceKind, sourceTitle, queuedSummary, decisionCaseID, emitEventID, w3cTraceID string,
	correlationID string,
) decisionlive.DecisionLiveEvent {
	refs := map[string]string{
		"eventId":   emitEventID,
		"eventType": "aidecision.execute_requested",
	}
	if decisionCaseID != "" {
		refs["decisionCaseId"] = decisionCaseID
	}
	if w3cTraceID != "" {
		refs["w3cTraceId"] = w3cTraceID
	}
	var bullets []string
	if decisionCaseID != "" {
		bullets = append(bullets, "Mã hồ sơ xử lý: "+decisionCaseID)
	}
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseQueued)
	queuedSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Tham chiếu", Items: []string{
			"Neo catalog: " + frame,
			"Mã sự kiện enqueue: " + emitEventID,
		}},
	}
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseQueued,
		OutcomeKind:      decisionlive.OutcomeNominal,
		SourceKind:       sourceKind,
		SourceTitle:      sourceTitle,
		Summary:          PublishWithSituation(frame, strings.TrimSpace(queuedSummary)),
		CorrelationID:    correlationID,
		DecisionCaseID:   decisionCaseID,
		W3CTraceID:       strings.TrimSpace(w3cTraceID),
		Refs:             refs,
		DetailBullets:    bullets,
		DetailSections:   queuedSections,
		ReasoningSummary: frame,
		Step: &decisionlive.TraceStep{
			Kind:      "queue",
			Title:     frame,
			Reasoning: frame,
		},
	}
}

// BuildExecuteConsumingEvent — PhaseConsuming khi consumer bắt đầu ExecuteWithCase.
func BuildExecuteConsumingEvent(
	sourceKind, sourceTitle, summary, caseID, correlationID, w3cLive string,
	evt *aidecisionmodels.DecisionEvent,
) decisionlive.DecisionLiveEvent {
	eid, esrc := "", ""
	if evt != nil {
		eid = evt.EventID
		esrc = evt.EventSource
	}
	consRefs := map[string]string{
		"eventId":     eid,
		"eventType":   "aidecision.execute_requested",
		"eventSource": esrc,
	}
	if caseID != "" {
		consRefs["decisionCaseId"] = caseID
	}
	if w3c := strings.TrimSpace(w3cLive); w3c != "" {
		consRefs["w3cTraceId"] = w3c
	}
	var bullets []string
	if caseID != "" {
		bullets = append(bullets, "Hồ sơ: "+caseID)
	}
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseConsuming)
	consumingSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Tham chiếu", Items: []string{
			"Neo catalog: " + frame,
			"Mã việc queue: " + eid,
		}},
	}
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseConsuming,
		OutcomeKind:      decisionlive.OutcomeNominal,
		SourceKind:       sourceKind,
		SourceTitle:      sourceTitle,
		Summary:          PublishWithSituation(frame, strings.TrimSpace(summary)),
		CorrelationID:    correlationID,
		DecisionCaseID:   caseID,
		W3CTraceID:       strings.TrimSpace(w3cLive),
		Refs:             consRefs,
		DetailBullets:    bullets,
		DetailSections:   consumingSections,
		ReasoningSummary: frame,
		Step: &decisionlive.TraceStep{
			Kind:      "execute",
			Title:     frame,
			Reasoning: frame,
		},
	}
}
