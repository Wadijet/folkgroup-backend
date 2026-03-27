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
		{Title: "Hành động đã chọn (trước policy)", Items: items},
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
		return mode
	}
}

// BuildEngineSkippedNoCix — PhaseSkipped khi thiếu CIX payload.
func BuildEngineSkippedNoCix(correlationID string) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseSkipped,
		Summary:       "[Bỏ qua] Thiếu dữ liệu tình huống (CIX) — không thể đưa ra quyết định.",
		CorrelationID: correlationID,
		ReasoningSummary: "Điều kiện tiên quyết: phải có payload phân tích tình huống.",
		DetailBullets: []string{
			"Đầu vào: CIXPayload rỗng hoặc không truyền.",
			"Cơ chế: engine dừng trước parse — không gọi LLM/policy.",
			"Kết quả: không có Execution Plan.",
		},
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
		Summary:       fmt.Sprintf("[Đang chạy] Đã đọc %d gợi ý hành động từ tình huống (CIX).", suggestionCount),
		CorrelationID: correlationID,
		ReasoningSummary: "Parse: trích actionSuggestions từ payload để pipeline rule/LLM xử lý.",
		DetailBullets: []string{
			"Đầu vào: actionSuggestions trong CIXPayload.",
			"Cơ chế: trích danh sách chuỗi trước khi rule/LLM.",
			fmt.Sprintf("Kết quả: %d gợi ý đưa vào bước kế.", suggestionCount),
		},
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
		Summary:          "[Dừng có kiểm soát] Không có hành động nào được chọn — không tạo đề xuất.",
		CorrelationID:    correlationID,
		DecisionMode:     decisionMode,
		Confidence:       confidence,
		Severity:         decisionlive.SeverityWarn,
		ReasoningSummary: reasoningSummary,
		DetailBullets: []string{
			"Đầu vào: danh sách gợi ý sau rule/LLM rỗng.",
			"Cơ chế: không vào policy/propose.",
			"Kết quả: Execution Plan rỗng — có thể bình thường theo nghiệp vụ.",
		},
	}
}

// BuildEngineDecisionEvent — PhaseDecision.
func BuildEngineDecisionEvent(correlationID, decisionMode string, confidence float64, reasoningSummary string, actionSuggestions []string) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseDecision,
		Summary:          fmt.Sprintf("[Hoàn tất bước] Đã chọn %d hành động (%s).", len(actionSuggestions), DecisionModeLabelVi(decisionMode)),
		CorrelationID:    correlationID,
		DecisionMode:     decisionMode,
		Confidence:       confidence,
		ReasoningSummary: reasoningSummary,
		Detail: map[string]interface{}{
			"selectedActions": actionSuggestions,
		},
		DetailBullets: []string{
			"Đầu vào: tập gợi ý sau rule/LLM.",
			"Cơ chế: tổng hợp chế độ quyết định và độ tin cậy.",
			fmt.Sprintf("Kết quả: %d hành động chọn cho bước policy.", len(actionSuggestions)),
		},
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
	policyBullets := []string{
		fmt.Sprintf("Đầu vào: %d hành động cần duyệt, %d hành động tự động (theo policy).", needApproval, autoActions),
		"Cơ chế: CIX_APPROVAL_ACTIONS — tách approve vs auto.",
		fmt.Sprintf("Kết quả: %d chờ duyệt, %d tự động (nếu cấu hình cho phép).", needApproval, autoActions),
	}
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhasePolicy,
		Summary:       fmt.Sprintf("[Phân loại] %d hành động cần duyệt, %d hành động xử lý tự động.", needApproval, autoActions),
		CorrelationID: correlationID,
		DetailBullets: policyBullets,
		ReasoningSummary: "Policy: danh mục duyệt lấy từ biến môi trường / mặc định escalate, assign.",
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
		Summary:       fmt.Sprintf("[Hoàn tất] Đã tạo %d đề xuất hoặc tác vụ thực hiện.", len(actionIDs)),
		CorrelationID: correlationID,
		Detail:        detail,
		ReasoningSummary: "Propose: ghi action_pending / delivery theo từng hành động.",
		DetailBullets: []string{
			"Đầu vào: danh sách hành động sau policy.",
			"Cơ chế: proposeCixAction / proposeAndApproveAuto.",
			fmt.Sprintf("Kết quả: %d bản ghi liên quan (id trong detail).", len(actionIDs)),
		},
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
		Summary:       "[Cảnh báo] Không tạo được đề xuất — kiểm tra quyền duyệt hoặc kết nối thực thi.",
		CorrelationID: correlationID,
		Severity:      decisionlive.SeverityWarn,
		ReasoningSummary: "Propose thất bại: không có action id — xem log propose/approval.",
		DetailBullets: []string{
			"Đầu vào: danh sách hành động sau policy.",
			"Cơ chế: propose trả lỗi hoặc nil document.",
			"Kết quả: không có action id — cần điều tra.",
		},
	}
}

// BuildEngineDoneEvent — PhaseDone cuối engine.
func BuildEngineDoneEvent(correlationID, srcTitle string) decisionlive.DecisionLiveEvent {
	doneSummary := "[Hoàn tất] Đã xử lý xong luồng quyết định cho phiên này."
	if strings.TrimSpace(srcTitle) != "" {
		doneSummary = fmt.Sprintf("[Hoàn tất] Đã xử lý xong %s.", strings.TrimSpace(srcTitle))
	}
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseDone,
		Summary:       doneSummary,
		CorrelationID: correlationID,
		ReasoningSummary: "Kết thúc pipeline engine: case có thể đóng (closed_proposed) tùy cấu hình.",
		DetailBullets: []string{
			"Đầu vào: toàn bộ bước trước đã hoàn thành hoặc dừng có kiểm soát.",
			"Cơ chế: executor quản lý bước sau trên action.",
			"Kết quả: timeline engine kết thúc tại mốc này.",
		},
	}
}

// BuildLLMEmptySuggestions — PhaseLLM khi không có gợi ý ban đầu.
func BuildLLMEmptySuggestions(correlationID string) decisionlive.DecisionLiveEvent {
	return decisionlive.DecisionLiveEvent{
		Phase:         decisionlive.PhaseLLM,
		Summary:       "[Đang chạy] LLM gợi ý hành động khi chưa có gợi ý từ tình huống.",
		CorrelationID: correlationID,
		ReasoningSummary: "LLM: chọn trong tập allowed khi rule/CIX không đưa danh sách.",
		DetailBullets: []string{
			"Đầu vào: CIX + customer context + allowed actions.",
			"Cơ chế: decideWhenEmpty.",
			"Kết quả: danh sách selected_actions (nếu model trả về hợp lệ).",
		},
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
		Summary:       "[Đang chạy] LLM lọc và tinh chỉnh danh sách gợi ý hành động.",
		CorrelationID: correlationID,
		ReasoningSummary: "LLM refine: giảm nhiễu khi CIX trả về nhiều hành động.",
		DetailBullets: []string{
			fmt.Sprintf("Đầu vào: %d gợi ý từ rule/CIX.", suggestionCount),
			"Cơ chế: refineActions trong tập allowed.",
			"Kết quả: danh sách rút gọn cho bước decision.",
		},
		Step: &decisionlive.TraceStep{
			Kind:      "llm",
			Title:     "Tinh chỉnh danh sách gợi ý",
			InputRef:  map[string]interface{}{"suggestionCount": suggestionCount},
			Reasoning: "Có nhiều gợi ý — dùng AI để chọn bộ hành động phù hợp nhất.",
		},
	}
}
