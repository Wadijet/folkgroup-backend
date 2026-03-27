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
	bullets := []string{
		"Đầu vào: yêu cầu execute với CIX/context đã hydrate; lane fast.",
		"Cơ chế: worker consume aidecision.execute_requested → ExecuteWithCase (engine).",
		"Kết quả mốc này: job đã vào decision_events_queue — chờ consumer.",
	}
	bullets = append(bullets,
		"Chi tiết: nạp case (nếu có), policy matrix, rule/LLM theo ngữ cảnh.",
	)
	if decisionCaseID != "" {
		bullets = append(bullets, "Case neo: "+decisionCaseID)
	}
	queuedSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Sau mốc queued: các live_event tiếp theo trên timeline", Items: []string{
			"Một live_event consuming khi worker lấy job.",
			"Các live_event parse / llm / decision / policy / propose — mỗi phase một mốc riêng.",
			"Nội dung: nạp case, CIX, policy matrix, rule/LLM — diễn ra qua chuỗi mốc trên.",
		}},
	}
	return decisionlive.DecisionLiveEvent{
		Phase:           decisionlive.PhaseQueued,
		SourceKind:      sourceKind,
		SourceTitle:     sourceTitle,
		Summary:         queuedSummary,
		CorrelationID:   correlationID,
		DecisionCaseID:  decisionCaseID,
		W3CTraceID:      strings.TrimSpace(w3cTraceID),
		Refs:            refs,
		DetailBullets:   bullets,
		DetailSections:  queuedSections,
		ReasoningSummary: "Hàng đợi thực thi quyết định: một job execute_requested đại diện cho một vòng engine.",
		Step: &decisionlive.TraceStep{
			Kind:  "queue",
			Title: "Xếp hàng thực thi quyết định",
		},
	}
}

// BuildExecuteConsumingEvent — PhaseConsuming khi consumer bắt đầu ExecuteWithCase.
func BuildExecuteConsumingEvent(
	sourceKind, sourceTitle, summary, caseID, correlationID, w3cLive string,
	evt *aidecisionmodels.DecisionEvent,
) decisionlive.DecisionLiveEvent {
	consRefs := map[string]string{
		"eventId":     evt.EventID,
		"eventType":   "aidecision.execute_requested",
		"eventSource": evt.EventSource,
	}
	if caseID != "" {
		consRefs["decisionCaseId"] = caseID
	}
	if w3c := strings.TrimSpace(w3cLive); w3c != "" {
		consRefs["w3cTraceId"] = w3c
	}
	bullets := []string{
		"Đầu vào: payload execute_requested đã parse; case (nếu có) đã nạp.",
		"Cơ chế: ExecuteWithCase — policy, rule/LLM, propose/approval.",
		"Kết quả mốc này: bắt đầu thực thi engine — các phase parse/decision/propose sẽ publish tiếp.",
	}
	if caseID != "" {
		bullets = append(bullets, "Case: "+caseID)
	}
	consumingSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Diễn giải mốc consuming (các phase sau = live_event khác)", Items: []string{
			"Tại mốc này: engine bắt đầu ExecuteWithCase.",
			"Parse / rule / LLM / policy / propose sẽ publish thành từng live_event kế tiếp — không gói gọn trong một dòng.",
		}},
	}
	return decisionlive.DecisionLiveEvent{
		Phase:           decisionlive.PhaseConsuming,
		SourceKind:      sourceKind,
		SourceTitle:     sourceTitle,
		Summary:         summary,
		CorrelationID:   correlationID,
		DecisionCaseID:  caseID,
		W3CTraceID:      strings.TrimSpace(w3cLive),
		Refs:            consRefs,
		DetailBullets:   bullets,
		DetailSections:  consumingSections,
		ReasoningSummary: "Consumer đã chọn job execute và gọi engine — không còn ở trạng thái chờ queue.",
		Step: &decisionlive.TraceStep{
			Kind:  "execute",
			Title: "Bắt đầu ExecuteWithCase",
		},
	}
}
