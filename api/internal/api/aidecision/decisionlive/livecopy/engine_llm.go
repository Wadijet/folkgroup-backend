package livecopy

import (
	"fmt"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
)

// engineDecisionSections — danh sách hành động trong cùng mốc decision (không phải mốc timeline thêm).
func engineDecisionSections(actionSuggestions []string) []decisionlive.DecisionLiveDetailSection {
	if len(actionSuggestions) == 0 {
		return nil
	}
	items := append([]string{}, actionSuggestions...)
	return []decisionlive.DecisionLiveDetailSection{
		{Title: "Các hành động được đề xuất (trước khi phân loại duyệt)", Items: items},
	}
}

// DecisionModeLabelVi nhãn chế độ quyết định (hiển thị timeline).
func DecisionModeLabelVi(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "rule":
		return "theo quy tắc"
	case "llm":
		return "theo AI"
	case "hybrid":
		return "kết hợp quy tắc và AI"
	case "":
		return "mặc định"
	default:
		return "theo cấu hình hệ thống"
	}
}

// BuildEngineSkippedNoCix — PhaseSkipped khi thiếu CIX payload.
func BuildEngineSkippedNoCix(correlationID string) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseSkipped,
		OutcomeKind:   decisionlive.OutcomeDataIncomplete,
		Summary:       "Chưa đủ dữ liệu phân tích hội thoại — tạm thời không đưa ra gợi ý.",
		CorrelationID: correlationID,
		ReasoningSummary: "Cần có kết quả phân tích tin nhắn trước; vui lòng chờ bước phân tích hoàn tất hoặc kiểm tra kết nối.",
		DetailBullets: []string{"Hệ thống chưa nhận được bản phân tích hội thoại cần thiết — không chạy các bước gợi ý tiếp theo."},
		Step: &decisionlive.TraceStep{
			Kind:      "rule",
			Title:     "Kiểm tra dữ liệu đầu vào",
			Reasoning: "Chưa có nội dung phân tích — không thể tiếp tục các bước sau.",
		},
	}
}

// BuildEngineParseEvent — PhaseParse.
func BuildEngineParseEvent(correlationID string, suggestionCount int) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseParse,
		OutcomeKind:   decisionlive.OutcomeNominal,
		Summary:       fmt.Sprintf("Đã đọc %d gợi ý từ phân tích hội thoại.", suggestionCount),
		CorrelationID: correlationID,
		ReasoningSummary: "Các gợi ý này sẽ được lọc bằng quy tắc và có thể bổ sung bằng AI nếu cần.",
		DetailBullets: []string{fmt.Sprintf("Có %d hành động gợi ý ban đầu để hệ thống xem xét.", suggestionCount)},
		Step: &decisionlive.TraceStep{
			Kind:      "rule",
			Title:     "Đọc gợi ý hành động",
			InputRef:  map[string]interface{}{"suggestionCount": suggestionCount},
			Reasoning: "Trích các hành động được đề xuất sẵn trong dữ liệu đầu vào.",
		},
	}
}

// BuildEngineEmptyActions — PhaseEmpty khi không còn action sau rule/LLM.
func BuildEngineEmptyActions(correlationID, decisionMode string, confidence float64, reasoningSummary string) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseEmpty,
		OutcomeKind:      decisionlive.OutcomeNoActions,
		Summary:          "Không có việc cần làm tiếp — không tạo đề xuất mới.",
		CorrelationID:    correlationID,
		DecisionMode:     decisionMode,
		Confidence:       confidence,
		Severity:         decisionlive.SeverityWarn,
		ReasoningSummary: reasoningSummary,
		DetailBullets: []string{"Sau khi áp quy tắc và AI (nếu có), không còn hành động phù hợp để đề xuất."},
	}
}

// BuildEngineDecisionEvent — PhaseDecision.
func BuildEngineDecisionEvent(correlationID, decisionMode string, confidence float64, reasoningSummary string, actionSuggestions []string) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseDecision,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Summary:          fmt.Sprintf("Đã chọn %d hướng xử lý (%s).", len(actionSuggestions), DecisionModeLabelVi(decisionMode)),
		CorrelationID:    correlationID,
		DecisionMode:     decisionMode,
		Confidence:       confidence,
		ReasoningSummary: reasoningSummary,
		Detail: map[string]interface{}{
			"selectedActions": actionSuggestions,
		},
		DetailBullets: []string{fmt.Sprintf("Đã có %d hành động ứng viên — bước sau: kiểm tra cần duyệt hay chạy tự động.", len(actionSuggestions))},
		DetailSections: engineDecisionSections(actionSuggestions),
		Step: &decisionlive.TraceStep{
			Kind:      "rule",
			Title:     "Tổng hợp kết quả phân tích",
			Reasoning: reasoningSummary,
			OutputRef: map[string]interface{}{"actions": actionSuggestions, "mode": decisionMode},
		},
	}
}

// BuildEnginePolicyEvent — PhasePolicy.
func BuildEnginePolicyEvent(correlationID string, needApproval, autoActions int) decisionlive.DecisionLiveEvent {
	policyBullets := []string{fmt.Sprintf("%d việc cần bạn hoặc quản trị duyệt · %d việc có thể chạy tự động.", needApproval, autoActions)}
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhasePolicy,
		OutcomeKind:   decisionlive.OutcomeNominal,
		Summary:       fmt.Sprintf("Đã phân loại: %d việc chờ duyệt, %d việc tự động.", needApproval, autoActions),
		CorrelationID: correlationID,
		DetailBullets: policyBullets,
		ReasoningSummary: "Danh mục «cần duyệt» do cấu hình hệ thống quyết định; phần còn lại có thể thực hiện ngay nếu cho phép.",
		Step: &decisionlive.TraceStep{
			Kind:  "policy",
			Title: "Phân loại theo quy tắc duyệt",
			InputRef: map[string]interface{}{
				"canApproval": needApproval,
				"auto":        autoActions,
			},
			Reasoning: "Hành động thuộc danh mục cần duyệt sẽ chờ xác nhận; các hành động còn lại có thể chạy tự động.",
		},
	}
}

// BuildEngineProposeSuccess — PhasePropose khi có action IDs.
func BuildEngineProposeSuccess(correlationID string, actionIDs []string, detail map[string]interface{}) decisionlive.DecisionLiveEvent {
	if detail == nil {
		detail = map[string]interface{}{"actionIds": actionIDs, "count": len(actionIDs)}
	}
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhasePropose,
		OutcomeKind:   decisionlive.OutcomeSuccess,
		Summary:       fmt.Sprintf("Đã tạo %d đề xuất hoặc việc cần làm.", len(actionIDs)),
		CorrelationID: correlationID,
		Detail:        detail,
		ReasoningSummary: "Bạn có thể xem và duyệt trong màn hình đề xuất hoặc việc được giao.",
		DetailBullets: []string{fmt.Sprintf("Có %d mục đã ghi nhận — chi tiết nằm trong phần mở rộng (nếu có).", len(actionIDs))},
		Step: &decisionlive.TraceStep{
			Kind:      "propose",
			Title:     "Ghi nhận đề xuất",
			OutputRef: detail,
		},
	}
}

// BuildEngineProposeNone — PhasePropose khi không tạo được bản ghi.
func BuildEngineProposeNone(correlationID string) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhasePropose,
		OutcomeKind:   decisionlive.OutcomeProposalFailed,
		Summary:       "Chưa tạo được đề xuất — có thể do quyền hoặc kết nối hệ thống.",
		CorrelationID: correlationID,
		Severity:      decisionlive.SeverityWarn,
		ReasoningSummary: "Vui lòng thử lại sau hoặc liên hệ quản trị nếu lặp lại nhiều lần.",
		DetailBullets: []string{"Hệ thống không ghi nhận được mã việc sau bước đề xuất — đội kỹ thuật có thể tra log nội bộ."},
	}
}

// BuildEngineDoneEvent — PhaseDone cuối engine.
func BuildEngineDoneEvent(correlationID, srcTitle string) decisionlive.DecisionLiveEvent {
	doneSummary := "Đã hoàn tất phân tích và gợi ý cho lượt này."
	if strings.TrimSpace(srcTitle) != "" {
		doneSummary = fmt.Sprintf("Đã xử lý xong %s.", strings.TrimSpace(srcTitle))
	}
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseDone,
		OutcomeKind:   decisionlive.OutcomeSuccess,
		Summary:       doneSummary,
		CorrelationID: correlationID,
		ReasoningSummary: "Các bước tiếp theo (nếu có) là duyệt hoặc thực hiện — tùy cấu hình cửa hàng.",
		DetailBullets: []string{"Vòng phân tích tự động đã kết thúc; việc gửi tin hoặc cập nhật hệ thống khác do bước sau đảm nhiệm."},
	}
}

// BuildLLMEmptySuggestions — PhaseLLM khi không có gợi ý ban đầu.
func BuildLLMEmptySuggestions(correlationID string) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseLLM,
		OutcomeKind:   decisionlive.OutcomeNominal,
		Summary:       "Đang dùng AI để gợi ý hành động vì chưa có danh sách từ phân tích hội thoại.",
		CorrelationID: correlationID,
		ReasoningSummary: "AI chỉ chọn trong các hành động được phép — an toàn theo cấu hình.",
		DetailBullets: []string{"Hệ thống bổ sung gợi ý bằng AI khi chưa có đủ gợi ý ban đầu."},
		Step: &decisionlive.TraceStep{
			Kind:      "llm",
			Title:     "Gợi ý khi chưa có danh sách ban đầu",
			Reasoning: "Chưa có gợi ý từ tình huống — hệ thống dùng AI để chọn trong các hành động được phép.",
		},
	}
}

// BuildLLMRefineSuggestions — PhaseLLM khi tinh chỉnh nhiều gợi ý.
func BuildLLMRefineSuggestions(correlationID string, suggestionCount int) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseLLM,
		OutcomeKind:   decisionlive.OutcomeNominal,
		Summary:       "Đang dùng AI để rút gọn danh sách gợi ý cho phù hợp hơn.",
		CorrelationID: correlationID,
		ReasoningSummary: "Khi có nhiều gợi ý, AI giúp chọn bộ hành động gọn và đúng ngữ cảnh hơn.",
		DetailBullets: []string{fmt.Sprintf("Đang xem xét %d gợi ý ban đầu.", suggestionCount)},
		Step: &decisionlive.TraceStep{
			Kind:      "llm",
			Title:     "Tinh chỉnh danh sách gợi ý",
			InputRef:  map[string]interface{}{"suggestionCount": suggestionCount},
			Reasoning: "Có nhiều gợi ý — dùng AI để chọn bộ hành động phù hợp nhất.",
		},
	}
}
