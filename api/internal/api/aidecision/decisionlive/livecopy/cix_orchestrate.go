package livecopy

import (
	"fmt"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	cixmodels "meta_commerce/internal/api/cix/models"
)

// BuildCixIntegratedEvent — PhaseCixIntegrated sau ReceiveCixPayload.
func BuildCixIntegratedEvent(traceID string, caseDoc *aidecisionmodels.DecisionCase, result *cixmodels.CixAnalysisResult) decisionlive.DecisionLiveEvent {
	bullets := []string{
		fmt.Sprintf("CIX trả về %d gợi ý hành động; phiên phân tích %s, khách %s.", len(result.ActionSuggestions), result.SessionUid, result.CustomerUid),
		"Hệ thống ghi kết quả phân tích vào hồ sơ xử lý (case); nếu đủ điều kiện theo quy tắc sẽ tự xếp hàng bước thực thi.",
		"Mốc này: phân tích hội thoại đã gắn vào case — có thể tiếp tục tới bước ra quyết định / thực thi.",
	}
	if strings.TrimSpace(result.TraceID) != "" {
		bullets = append(bullets, "Mã truy vết pipeline rule (CIX): "+strings.TrimSpace(result.TraceID))
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
		Summary:        "[Hoàn tất] Đã ghi nhận kết quả phân tích hội thoại (CIX) vào hồ sơ xử lý.",
		DetailBullets:  bullets,
		DetailSections: sections,
		Refs:           refs,
		ReasoningSummary: "Kết quả CIX là đầu vào cho bước ra quyết định; có thực thi ngay hay không tùy bảng quy tắc ngữ cảnh.",
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
		"Đã đủ các ngữ cảnh bắt buộc trên case; đã có kết quả phân tích hội thoại (CIX).",
		"Hệ thống đánh dấu case sẵn sàng và xếp hàng chạy engine ra quyết định (execute).",
		"Mốc này: kiểm tra cuối trước khi chạy engine — tránh thực thi khi còn thiếu dữ liệu.",
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
		Summary:        "[Sẵn sàng] Đủ điều kiện — sắp chạy engine ra quyết định (đã xếp hàng execute).",
		CorrelationID:  correlationID,
		DecisionCaseID: caseDoc.DecisionCaseID,
		DetailBullets:  bullets,
		DetailSections: sections,
		Refs:           refs,
		ReasoningSummary: "Theo quy tắc: mọi ngữ cảnh bắt buộc đã đủ trước khi xếp hàng bước execute.",
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
		"Có sự kiện đổi trên hội thoại / tin nhắn (thường từ đồng bộ hoặc sau khi dữ liệu nguồn cập nhật).",
		"Hệ thống tìm hoặc tạo hồ sơ xử lý loại «trả lời hội thoại»; có thể xếp hàng lấy thêm ngữ cảnh khách hoặc phân tích CIX.",
		"Mốc này: đã neo / cập nhật case; các bước tiếp theo tùy còn thiếu conversationId, customerId, v.v.",
	}
	bullets = append(bullets, "Loại hồ sơ: phản hồi hội thoại (conversation_response).")
	if caseDoc != nil {
		if createdNew {
			bullets = append(bullets, "Đã tạo hồ sơ xử lý mới.")
		} else {
			bullets = append(bullets, "Đã cập nhật hồ sơ đang mở.")
		}
		bullets = append(bullets, "Mã hồ sơ: "+caseDoc.DecisionCaseID)
	} else {
		bullets = append(bullets, "Không có hồ sơ sau bước neo case.")
	}
	if convID != "" {
		bullets = append(bullets, "Hội thoại: "+convID)
	} else {
		bullets = append(bullets, "Chưa có conversationId — không xếp hàng phân tích CIX.")
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
		subItems = append(subItems, fmt.Sprintf("Đã xếp hàng lấy thêm ngữ cảnh khách (%s).", eventtypes.CustomerContextRequested))
	}
	if emittedCix {
		subItems = append(subItems, fmt.Sprintf("Đã xếp hàng phân tích hội thoại (%s).", eventtypes.CixAnalysisRequested))
	}
	sections := []decisionlive.DecisionLiveDetailSection{}
	if len(subItems) > 0 {
		sections = append(sections, decisionlive.DecisionLiveDetailSection{Title: "Việc đã xếp hàng tiếp theo", Items: subItems})
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
		Summary:        "[Hoàn tất] Đã điều phối hồ sơ hội thoại và các bước chuẩn bị ngữ cảnh.",
		CorrelationID:  evt.CorrelationID,
		DetailBullets:  bullets,
		DetailSections: sections,
		ReasoningSummary: "Neo case hội thoại và tùy dữ liệu có thể xếp hàng thêm ngữ cảnh khách hoặc phân tích CIX.",
		Step: &decisionlive.TraceStep{
			Kind:  "orchestrate",
			Title: "Điều phối sau sự kiện hội thoại",
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
		"Có sự kiện đơn hàng mới hoặc cập nhật (thường sau khi dữ liệu nguồn đổi).",
		"Hệ thống tìm hoặc tạo hồ sơ loại «rủi ro đơn» và xếp hàng phân tích đơn (Order Intelligence) khi có thể.",
		"Mốc này: case đã neo / cập nhật; phân tích đơn được xếp hàng nếu bước enqueue thành công.",
	}
	bullets = append(bullets, "Loại hồ sơ: rủi ro đơn (order_risk).")
	if caseDoc != nil {
		if createdNew {
			bullets = append(bullets, "Đã tạo hồ sơ rủi ro đơn mới.")
		} else {
			bullets = append(bullets, "Đã cập nhật hồ sơ rủi ro đơn đang mở.")
		}
		bullets = append(bullets, "Mã hồ sơ: "+caseDoc.DecisionCaseID)
	} else if orderUid != "" {
		bullets = append(bullets, "Không có hồ sơ sau bước neo case.")
	}
	if orderUid != "" {
		bullets = append(bullets, "Đơn (uid): "+orderUid)
		if enqueuedOrderIntelOK {
			bullets = append(bullets, "Xếp hàng phân tích đơn: thành công.")
		} else {
			bullets = append(bullets, "Xếp hàng phân tích đơn: lỗi — xem log worker.")
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
		Summary:       "[Hoàn tất] Đã điều phối hồ sơ rủi ro đơn và xếp hàng phân tích đơn.",
		CorrelationID: evt.CorrelationID,
		DetailBullets: bullets,
		ReasoningSummary: "Tách hồ sơ rủi ro đơn và kích hoạt chuỗi phân tích đơn khi có dữ liệu hợp lệ.",
		Step: &decisionlive.TraceStep{
			Kind:  "orchestrate",
			Title: "Điều phối rủi ro đơn hàng",
		},
		Refs: refs,
	}
	if caseDoc != nil {
		ev.DecisionCaseID = caseDoc.DecisionCaseID
	}
	return ev
}
