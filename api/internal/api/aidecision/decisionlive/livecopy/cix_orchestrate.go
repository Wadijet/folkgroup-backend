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
	var bullets []string
	if strings.TrimSpace(result.TraceID) != "" {
		bullets = append(bullets, "Mã tham chiếu phân tích: "+strings.TrimSpace(result.TraceID))
	}
	sections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Gợi ý hành động", Items: append([]string{}, result.ActionSuggestions...)},
	}
	if len(result.PipelineRuleTraceIDs) > 0 {
		pipe := make([]string, len(result.PipelineRuleTraceIDs))
		copy(pipe, result.PipelineRuleTraceIDs)
		sections = append(sections, decisionlive.DecisionLiveDetailSection{Title: "Mã quy tắc (cho đội kỹ thuật)", Items: pipe})
	}
	refs := map[string]string{"traceId": traceID}
	if caseDoc != nil {
		refs["decisionCaseId"] = caseDoc.DecisionCaseID
	}
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseCixIntegrated)
	sit := fmt.Sprintf("%d gợi ý hành động · phiên %s · khách %s", len(result.ActionSuggestions), result.SessionUid, result.CustomerUid)
	ev := decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseCixIntegrated,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Severity:         decisionlive.SeverityInfo,
		Summary:          PublishWithSituation(frame, sit),
		DetailBullets:    bullets,
		DetailSections:   sections,
		Refs:             refs,
		ReasoningSummary: frame,
		Step: &decisionlive.TraceStep{
			Kind:      "cix",
			Title:     frame,
			Reasoning: frame,
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
	bullets := []string{"Hồ sơ: " + caseDoc.DecisionCaseID}
	var reqList []string
	if len(caseDoc.RequiredContexts) > 0 {
		reqList = append(reqList, caseDoc.RequiredContexts...)
	} else {
		reqList = []string{"(theo cấu hình hiện tại, không còn mục bắt buộc thiếu)"}
	}
	sections := []decisionlive.DecisionLiveDetailSection{
		{Title: "Thông tin đã kiểm tra", Items: reqList},
	}
	refs := map[string]string{
		"traceId":        traceID,
		"decisionCaseId": caseDoc.DecisionCaseID,
	}
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseExecuteReady)
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseExecuteReady,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Severity:         decisionlive.SeverityInfo,
		Summary:          PublishWithSituation(frame, caseDoc.DecisionCaseID),
		CorrelationID:    correlationID,
		DecisionCaseID:   caseDoc.DecisionCaseID,
		DetailBullets:    bullets,
		DetailSections:   sections,
		Refs:             refs,
		ReasoningSummary: frame,
		Step: &decisionlive.TraceStep{
			Kind:      "gate",
			Title:     frame,
			Reasoning: frame,
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
	orchTid, orchCid := "", ""
	if evt != nil {
		orchTid = strings.TrimSpace(evt.TraceID)
		orchCid = strings.TrimSpace(evt.CorrelationID)
	}
	bullets := []string{
		"Hệ thống đã mở hoặc cập nhật hồ sơ xử lý cho cuộc hội thoại này.",
		fmt.Sprintf("Mã luồng: %s · mã liên kết: %s.", orchTid, orchCid),
	}
	if caseDoc != nil {
		line := "Case: " + caseDoc.DecisionCaseID
		if createdNew {
			line += " (mới)."
		} else {
			line += " (cập nhật)."
		}
		bullets = append(bullets, line)
	}
	if convID != "" {
		bullets = append(bullets, "Hội thoại: "+convID)
	} else {
		bullets = append(bullets, "Chưa có mã hội thoại — chưa thể xếp hàng phân tích sâu.")
	}
	if custID != "" {
		bullets = append(bullets, "Khách: "+custID)
	}
	if channel != "" {
		bullets = append(bullets, "Kênh: "+channel)
	}
	if normalizedRecordUid != "" {
		bullets = append(bullets, "Bản ghi: "+normalizedRecordUid)
	}
	var subItems []string
	if emittedCustomer {
		subItems = append(subItems, "Đã xếp hàng bổ sung thông tin khách.")
	}
	if emittedCix {
		subItems = append(subItems, "Đã xếp hàng phân tích nội dung hội thoại.")
	}
	sections := []decisionlive.DecisionLiveDetailSection{}
	if len(subItems) > 0 {
		sections = append(sections, decisionlive.DecisionLiveDetailSection{Title: "Việc tiếp theo đã xếp hàng", Items: subItems})
	}
	bullets = capDetailBullets(bullets, 8)
	refs := map[string]string{
		"eventId":     evt.EventID,
		"eventType":   evt.EventType,
		"eventSource": evt.EventSource,
	}
	if caseDoc != nil {
		refs["decisionCaseId"] = caseDoc.DecisionCaseID
	}
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseOrchestrate)
	ev := decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseOrchestrate,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Severity:         decisionlive.SeverityInfo,
		Summary:          PublishWithSituation(frame, "Hội thoại / tin nhắn"),
		CorrelationID:    evt.CorrelationID,
		DetailBullets:    bullets,
		DetailSections:   sections,
		ReasoningSummary: frame,
		Step: &decisionlive.TraceStep{
			Kind:      "orchestrate",
			Title:     frame,
			Reasoning: frame,
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
		"Hệ thống đã mở hoặc cập nhật hồ sơ theo dõi rủi ro cho đơn hàng.",
	}
	if caseDoc != nil {
		line := "Case: " + caseDoc.DecisionCaseID
		if createdNew {
			line += " (mới)."
		} else {
			line += " (cập nhật)."
		}
		bullets = append(bullets, line)
	}
	if orderUid != "" {
		line := "Đơn: " + orderUid
		if enqueuedOrderIntelOK {
			line += " · đã xếp hàng làm mới thông tin đơn"
		} else {
			line += " · chưa xếp hàng làm mới thông tin (có lỗi)"
		}
		bullets = append(bullets, line)
	}
	if custID != "" {
		bullets = append(bullets, "Khách: "+custID)
	}
	if convID != "" {
		bullets = append(bullets, "Hội thoại: "+convID)
	}
	bullets = capDetailBullets(bullets, 8)
	refs := map[string]string{
		"eventId":     evt.EventID,
		"eventType":   evt.EventType,
		"eventSource": evt.EventSource,
	}
	if caseDoc != nil {
		refs["decisionCaseId"] = caseDoc.DecisionCaseID
	}
	orderOutcome := decisionlive.OutcomeNominal
	if orderUid != "" && !enqueuedOrderIntelOK {
		orderOutcome = decisionlive.OutcomePartialFailure
	}
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseOrchestrate)
	ev := decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseOrchestrate,
		OutcomeKind:      orderOutcome,
		Severity:         decisionlive.SeverityInfo,
		Summary:          PublishWithSituation(frame, "Theo dõi đơn / rủi ro"),
		CorrelationID:    evt.CorrelationID,
		DetailBullets:    bullets,
		ReasoningSummary: frame,
		Step: &decisionlive.TraceStep{
			Kind:      "orchestrate",
			Title:     frame,
			Reasoning: frame,
		},
		Refs: refs,
	}
	if caseDoc != nil {
		ev.DecisionCaseID = caseDoc.DecisionCaseID
	}
	return ev
}
