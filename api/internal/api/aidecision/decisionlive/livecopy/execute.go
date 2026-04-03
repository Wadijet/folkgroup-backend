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
		"Đã có đủ ngữ cảnh (gồm phân tích hội thoại nếu cần) để chạy engine ra quyết định.",
		"Yêu cầu thực thi đã được ghi vào hàng đợi; worker sẽ lấy job và gọi engine (ExecuteWithCase).",
		"Mốc này: vừa xếp hàng — chưa chạy engine; các bước parse / quyết định / đề xuất nằm ở mốc sau.",
	}
	bullets = append(bullets,
		"Engine sẽ nạp hồ sơ (nếu có), áp quy tắc và có thể gọi LLM tùy cấu hình.",
	)
	if decisionCaseID != "" {
		bullets = append(bullets, "Hồ sơ liên quan: "+decisionCaseID)
	}
	queuedSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Các mốc tiếp theo trên timeline", Items: []string{
			"Một mốc khi worker bắt đầu consume job execute.",
			"Sau đó là chuỗi mốc: phân tích đầu vào → quyết định → chính sách → đề xuất (mỗi bước một dòng trên timeline).",
			"Toàn bộ diễn ra tuần tự — không gộp trong một mốc duy nhất.",
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
		ReasoningSummary: "Một job execute trên hàng đợi tương ứng một lượt chạy engine ra quyết định.",
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
		"Worker đã đọc xong yêu cầu execute và nạp hồ sơ (nếu có).",
		"Engine bắt đầu chạy: quy tắc, có thể LLM, rồi tạo đề xuất hoặc bước duyệt.",
		"Mốc này: mới khởi động engine — các bước chi tiết sẽ hiện thành từng dòng timeline phía sau.",
	}
	if caseID != "" {
		bullets = append(bullets, "Hồ sơ: "+caseID)
	}
	consumingSections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Các bước sau (mỗi bước = một mốc timeline)", Items: []string{
			"Ở mốc này engine vừa bắt đầu xử lý job execute.",
			"Phân tích, quyết định, chính sách và đề xuất được ghi riêng từng mốc — không gộp một dòng.",
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
		ReasoningSummary: "Consumer đã lấy job execute và gọi engine — không còn chờ trên hàng đợi.",
		Step: &decisionlive.TraceStep{
			Kind:  "execute",
			Title: "Bắt đầu chạy engine ra quyết định",
		},
	}
}
