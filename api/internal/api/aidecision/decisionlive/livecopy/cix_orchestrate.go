package livecopy

import (
	"fmt"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	cixmodels "meta_commerce/internal/api/cix/models"
)

// BuildCixIntegratedEvent — PhaseCixIntegrated sau ReceiveCixPayload.
func BuildCixIntegratedEvent(traceID string, caseDoc *aidecisionmodels.DecisionCase, result *cixmodels.CixAnalysisResult) decisionlive.DecisionLiveEvent {
	bullets := []string{
		fmt.Sprintf("Đầu vào: %d gợi ý hành động từ CIX; phiên %s, khách %s.", len(result.ActionSuggestions), result.SessionUid, result.CustomerUid),
		"Cơ chế: ghi contextPackets.cix vào decision_cases_runtime; TryExecuteIfReady khi đủ policy.",
		"Kết quả mốc này: CIX đã tích hợp vào case — có thể dẫn tới execute_requested.",
	}
	if strings.TrimSpace(result.TraceID) != "" {
		bullets = append(bullets, "Trace rule (CIX): "+strings.TrimSpace(result.TraceID))
	}
	sections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Gợi ý hành động (CIX)", Items: append([]string{}, result.ActionSuggestions...)},
	}
	if len(result.PipelineRuleTraceIDs) > 0 {
		pipe := make([]string, len(result.PipelineRuleTraceIDs))
		copy(pipe, result.PipelineRuleTraceIDs)
		sections = append(sections, decisionlive.DecisionLiveDetailSection{Title: "Pipeline rule (thứ tự)", Items: pipe})
	}
	refs := map[string]string{"traceId": traceID}
	if caseDoc != nil {
		refs["decisionCaseId"] = caseDoc.DecisionCaseID
	}
	ev := decisionlive.DecisionLiveEvent{
		Phase:          decisionlive.PhaseCixIntegrated,
		Severity:       decisionlive.SeverityInfo,
		Summary:        "[Hoàn tất] Đã ghi nhận kết quả phân tích CIX vào decision case.",
		DetailBullets:  bullets,
		DetailSections: sections,
		Refs:           refs,
		ReasoningSummary: "Tích hợp CIX: nguồn gợi ý cho engine; điều kiện execute phụ thuộc policy matrix.",
		Step: &decisionlive.TraceStep{
			Kind:  "cix",
			Title: "Tích hợp CIX vào case",
		},
	}
	if caseDoc != nil {
		ev.DecisionCaseID = caseDoc.DecisionCaseID
		ev.CorrelationID = caseDoc.CorrelationID
	}
	return ev
}

// BuildExecuteReadyEvent — PhaseExecuteReady trước EmitExecuteRequested.
func BuildExecuteReadyEvent(traceID, correlationID string, caseDoc *aidecisionmodels.DecisionCase) decisionlive.DecisionLiveEvent {
	bullets := []string{
		"Đầu vào: đủ requiredContexts; đã có contextPackets.cix.",
		"Cơ chế: cập nhật status ready_for_decision → emit aidecision.execute_requested.",
		"Kết quả mốc này: cổng an toàn trước khi engine chạy — tránh execute thiếu ngữ cảnh.",
	}
	bullets = append(bullets, "Case: "+caseDoc.DecisionCaseID)
	var reqList []string
	if len(caseDoc.RequiredContexts) > 0 {
		reqList = append(reqList, caseDoc.RequiredContexts...)
	} else {
		reqList = []string{"(không khai báo danh sách trên case — đã thỏa policy)"}
	}
	sections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Ngữ cảnh bắt buộc (policy)", Items: reqList},
	}
	refs := map[string]string{
		"traceId":        traceID,
		"decisionCaseId": caseDoc.DecisionCaseID,
	}
	return decisionlive.DecisionLiveEvent{
		Phase:          decisionlive.PhaseExecuteReady,
		Severity:       decisionlive.SeverityInfo,
		Summary:        "[Sẵn sàng] Đủ điều kiện — chuẩn bị thực thi quyết định (execute_requested).",
		CorrelationID:  correlationID,
		DecisionCaseID: caseDoc.DecisionCaseID,
		DetailBullets:  bullets,
		DetailSections: sections,
		Refs:           refs,
		ReasoningSummary: "Policy matrix: mọi required context đã có trước khi xếp hàng execute.",
		Step: &decisionlive.TraceStep{
			Kind:  "gate",
			Title: "Kiểm tra đủ ngữ cảnh trước khi execute",
		},
	}
}

// BuildOrchestrateConversationEvent — PhaseOrchestrate sau ResolveOrCreate conversation.
func BuildOrchestrateConversationEvent(
	evt *aidecisionmodels.DecisionEvent,
	caseDoc *aidecisionmodels.DecisionCase,
	createdNew bool,
	convID, custID, channel, normalizedRecordUid string,
	emittedCustomer, emittedCix bool,
) decisionlive.DecisionLiveEvent {
	bullets := []string{
		"Đầu vào: sự kiện nguồn/datachanged hội thoại; ResolveOrCreate case conversation_response.",
		"Cơ chế: có thể emit customer.context_requested và cix.analysis_requested.",
		"Kết quả mốc này: case neo hoặc cập nhật; chuỗi tác vụ phụ tùy payload.",
	}
	bullets = append(bullets, "Loại case: conversation_response.")
	if caseDoc != nil {
		if createdNew {
			bullets = append(bullets, "Đã tạo decision case mới.")
		} else {
			bullets = append(bullets, "Đã cập nhật case đang mở.")
		}
		bullets = append(bullets, "Case: "+caseDoc.DecisionCaseID)
	} else {
		bullets = append(bullets, "Không có case sau ResolveOrCreate.")
	}
	if convID != "" {
		bullets = append(bullets, "Hội thoại: "+convID)
	} else {
		bullets = append(bullets, "Chưa có conversationId — không gửi cix.analysis_requested.")
	}
	if custID != "" {
		bullets = append(bullets, "Khách: "+custID)
	}
	if channel != "" {
		bullets = append(bullets, "Kênh: "+channel)
	}
	if normalizedRecordUid != "" {
		bullets = append(bullets, "Bản ghi chuẩn hoá: "+normalizedRecordUid)
	}
	var subItems []string
	if emittedCustomer {
		subItems = append(subItems, "Đã gửi customer.context_requested.")
	}
	if emittedCix {
		subItems = append(subItems, "Đã gửi cix.analysis_requested.")
	}
	sections := []decisionlive.DecisionLiveDetailSection{}
	if len(subItems) > 0 {
		sections = append(sections, decisionlive.DecisionLiveDetailSection{Title: "Tác vụ đã xếp hàng", Items: subItems})
	}
	refs := map[string]string{
		"eventId":     evt.EventID,
		"eventType":   evt.EventType,
		"eventSource": evt.EventSource,
	}
	if caseDoc != nil {
		refs["decisionCaseId"] = caseDoc.DecisionCaseID
	}
	ev := decisionlive.DecisionLiveEvent{
		Phase:          decisionlive.PhaseOrchestrate,
		Severity:       decisionlive.SeverityInfo,
		Summary:        "[Hoàn tất] Điều phối case hội thoại và tác vụ chuẩn bị ngữ cảnh.",
		CorrelationID:  evt.CorrelationID,
		DetailBullets:  bullets,
		DetailSections: sections,
		ReasoningSummary: "Orchestrate: neo case + phát event con theo thiếu/đủ conversationId và customerId.",
		Step: &decisionlive.TraceStep{
			Kind:  "orchestrate",
			Title: "Điều phối sau sự kiện nguồn",
		},
		Refs: refs,
	}
	if caseDoc != nil {
		ev.DecisionCaseID = caseDoc.DecisionCaseID
	}
	return ev
}

// BuildOrchestrateOrderEvent — PhaseOrchestrate order_risk.
func BuildOrchestrateOrderEvent(
	evt *aidecisionmodels.DecisionEvent,
	caseDoc *aidecisionmodels.DecisionCase,
	createdNew bool,
	orderUid, custID, convID string,
	enqueuedOrderIntelOK bool,
) decisionlive.DecisionLiveEvent {
	bullets := []string{
		"Đầu vào: sự kiện nguồn/datachanged đơn; ResolveOrCreate case order_risk.",
		"Cơ chế: EnqueueOrderIntelligenceFromParent (Order Intelligence domain).",
		"Kết quả mốc này: case + hàng đợi phân tích đơn (nếu enqueue thành công).",
	}
	bullets = append(bullets, "Loại case: order_risk.")
	if caseDoc != nil {
		if createdNew {
			bullets = append(bullets, "Đã tạo case order_risk.")
		} else {
			bullets = append(bullets, "Đã cập nhật case order_risk đang mở.")
		}
		bullets = append(bullets, "Case: "+caseDoc.DecisionCaseID)
	} else if orderUid != "" {
		bullets = append(bullets, "Không có case sau ResolveOrCreate.")
	}
	if orderUid != "" {
		bullets = append(bullets, "Đơn (uid): "+orderUid)
		if enqueuedOrderIntelOK {
			bullets = append(bullets, "EnqueueOrderIntelligence: thành công.")
		} else {
			bullets = append(bullets, "EnqueueOrderIntelligence: lỗi — xem log.")
		}
	}
	if custID != "" {
		bullets = append(bullets, "Khách: "+custID)
	}
	if convID != "" {
		bullets = append(bullets, "Hội thoại: "+convID)
	}
	refs := map[string]string{
		"eventId":     evt.EventID,
		"eventType":   evt.EventType,
		"eventSource": evt.EventSource,
	}
	if caseDoc != nil {
		refs["decisionCaseId"] = caseDoc.DecisionCaseID
	}
	ev := decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseOrchestrate,
		Severity:      decisionlive.SeverityInfo,
		Summary:       "[Hoàn tất] Điều phối case rủi ro đơn và hàng đợi phân tích đơn.",
		CorrelationID: evt.CorrelationID,
		DetailBullets: bullets,
		ReasoningSummary: "Orchestrate đơn: tách case order_risk và intelligence pipeline.",
		Step: &decisionlive.TraceStep{
			Kind:  "orchestrate",
			Title: "Điều phối order_risk",
		},
		Refs: refs,
	}
	if caseDoc != nil {
		ev.DecisionCaseID = caseDoc.DecisionCaseID
	}
	return ev
}
