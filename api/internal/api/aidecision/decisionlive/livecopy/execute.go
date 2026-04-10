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
		"Đã đủ thông tin cần thiết — hệ thống sẽ phân tích và gợi ý trong giây lát.",
		"Các bước tiếp theo sẽ hiện thêm trên dòng thời gian (đọc gợi ý, duyệt, tạo việc…).",
	}
	if decisionCaseID != "" {
		bullets = append(bullets, "Mã hồ sơ xử lý: "+decisionCaseID)
	}
	queuedSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Thông tin thêm", Items: []string{
			"Thứ tự thường gặp: nhận việc → đọc gợi ý → (có thể) hỗ trợ AI → chọn hành động → tạo đề xuất.",
			"Nếu cần hỗ trợ, gửi kèm mã trong phần tham chiếu của sự kiện.",
		}},
	}
	return decisionlive.DecisionLiveEvent{
		Phase:           decisionlive.PhaseQueued,
		OutcomeKind:     decisionlive.OutcomeNominal,
		SourceKind:      sourceKind,
		SourceTitle:     sourceTitle,
		Summary:         queuedSummary,
		CorrelationID:   correlationID,
		DecisionCaseID:  decisionCaseID,
		W3CTraceID:      strings.TrimSpace(w3cTraceID),
		Refs:            refs,
		DetailBullets:   bullets,
		DetailSections:  queuedSections,
		ReasoningSummary: "Mỗi lượt xử lý tương ứng một vòng phân tích và gợi ý hoàn chỉnh.",
		Step: &decisionlive.TraceStep{
			Kind:  "queue",
			Title: "Đang chờ phân tích trợ lý",
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
		"Trợ lý đã bắt đầu phân tích — các bước tiếp theo sẽ hiện dưới dạng dòng thời gian.",
	}
	if caseID != "" {
		bullets = append(bullets, "Hồ sơ: "+caseID)
	}
	consumingSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Thông tin thêm", Items: []string{
			"Thời điểm hiển thị giúp bạn đối chiếu thứ tự; mã hồ sơ nằm trong phần tham chiếu nếu cần gửi hỗ trợ.",
		}},
	}
	return decisionlive.DecisionLiveEvent{
		Phase:           decisionlive.PhaseConsuming,
		OutcomeKind:     decisionlive.OutcomeNominal,
		SourceKind:      sourceKind,
		SourceTitle:     sourceTitle,
		Summary:         summary,
		CorrelationID:   correlationID,
		DecisionCaseID:  caseID,
		W3CTraceID:      strings.TrimSpace(w3cLive),
		Refs:            consRefs,
		DetailBullets:   bullets,
		DetailSections:  consumingSections,
		ReasoningSummary: "Hệ thống đang phân tích trực tiếp, không còn chờ trong hàng đợi.",
		Step: &decisionlive.TraceStep{
			Kind:  "execute",
			Title: "Đang phân tích và chuẩn bị gợi ý",
		},
	}
}
