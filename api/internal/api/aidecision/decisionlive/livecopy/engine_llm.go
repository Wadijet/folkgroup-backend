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
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseSkipped)
	sit := "Thiếu phân tích hội thoại (CIX) — không chạy tiếp engine."
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseSkipped,
		OutcomeKind:      decisionlive.OutcomeDataIncomplete,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		ReasoningSummary: frame,
		DetailBullets:    []string{sit},
		Step: &decisionlive.TraceStep{
			Kind:      "rule",
			Title:     frame,
			Reasoning: frame,
		},
	}
}

// BuildEngineParseEvent — PhaseParse.
func BuildEngineParseEvent(correlationID string, suggestionCount int) decisionlive.DecisionLiveEvent {
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseParse)
	sit := fmt.Sprintf("Đã đọc %d gợi ý ban đầu.", suggestionCount)
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseParse,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		ReasoningSummary: frame,
		DetailBullets:    []string{fmt.Sprintf("Số gợi ý ban đầu: %d", suggestionCount)},
		Step: &decisionlive.TraceStep{
			Kind:      "rule",
			Title:     frame,
			InputRef:  map[string]interface{}{"suggestionCount": suggestionCount},
			Reasoning: frame,
		},
	}
}

// BuildEngineEmptyActions — PhaseEmpty khi không còn action sau rule/LLM.
func BuildEngineEmptyActions(correlationID, decisionMode string, confidence float64, reasoningSummary string) decisionlive.DecisionLiveEvent {
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseEmpty)
	sit := fmt.Sprintf("Chế độ %s · độ tin %.2f · không còn hành động ứng viên.", DecisionModeLabelVi(decisionMode), confidence)
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseEmpty,
		OutcomeKind:      decisionlive.OutcomeNoActions,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		DecisionMode:     decisionMode,
		Confidence:       confidence,
		Severity:         decisionlive.SeverityWarn,
		ReasoningSummary: PublishReasoningCatalogPlusEngineLine(decisionlive.PhaseEmpty, reasoningSummary),
		DetailBullets:    []string{sit},
	}
}

// BuildEngineDecisionEvent — PhaseDecision.
func BuildEngineDecisionEvent(correlationID, decisionMode string, confidence float64, reasoningSummary string, actionSuggestions []string) decisionlive.DecisionLiveEvent {
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseDecision)
	sit := fmt.Sprintf("%d hướng ứng viên · %s · độ tin %.2f", len(actionSuggestions), DecisionModeLabelVi(decisionMode), confidence)
	rs := PublishReasoningCatalogPlusEngineLine(decisionlive.PhaseDecision, reasoningSummary)
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseDecision,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		DecisionMode:     decisionMode,
		Confidence:       confidence,
		ReasoningSummary: rs,
		Detail: map[string]interface{}{
			"selectedActions": actionSuggestions,
		},
		DetailBullets:  []string{fmt.Sprintf("Số hành động ứng viên: %d", len(actionSuggestions))},
		DetailSections: engineDecisionSections(actionSuggestions),
		Step: &decisionlive.TraceStep{
			Kind:      "rule",
			Title:     frame,
			Reasoning: rs,
			OutputRef: map[string]interface{}{"actions": actionSuggestions, "mode": decisionMode},
		},
	}
}

// BuildEnginePolicyEvent — PhasePolicy.
func BuildEnginePolicyEvent(correlationID string, needApproval, autoActions int) decisionlive.DecisionLiveEvent {
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhasePolicy)
	sit := fmt.Sprintf("Chờ duyệt: %d · Tự động: %d", needApproval, autoActions)
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhasePolicy,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		DetailBullets:    []string{sit},
		ReasoningSummary: frame,
		Step: &decisionlive.TraceStep{
			Kind:  "policy",
			Title: frame,
			InputRef: map[string]interface{}{
				"canApproval": needApproval,
				"auto":        autoActions,
			},
			Reasoning: frame,
		},
	}
}

// BuildEngineProposeSuccess — PhasePropose khi có action IDs.
func BuildEngineProposeSuccess(correlationID string, actionIDs []string, detail map[string]interface{}) decisionlive.DecisionLiveEvent {
	if detail == nil {
		detail = map[string]interface{}{"actionIds": actionIDs, "count": len(actionIDs)}
	}
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhasePropose)
	sit := fmt.Sprintf("Đã ghi %d mục (đề xuất / việc).", len(actionIDs))
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhasePropose,
		OutcomeKind:      decisionlive.OutcomeSuccess,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		Detail:           detail,
		ReasoningSummary: frame,
		DetailBullets:    []string{fmt.Sprintf("Số mục: %d", len(actionIDs))},
		Step: &decisionlive.TraceStep{
			Kind:      "propose",
			Title:     frame,
			Reasoning: frame,
			OutputRef: detail,
		},
	}
}

// BuildEngineProposeNone — PhasePropose khi không tạo được bản ghi.
func BuildEngineProposeNone(correlationID string) decisionlive.DecisionLiveEvent {
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhasePropose)
	sit := "Không ghi nhận được mục sau bước đề xuất."
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhasePropose,
		OutcomeKind:      decisionlive.OutcomeProposalFailed,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		Severity:         decisionlive.SeverityWarn,
		ReasoningSummary: frame,
		DetailBullets:    []string{sit},
	}
}

// BuildEngineDoneEvent — PhaseDone cuối engine.
func BuildEngineDoneEvent(correlationID, srcTitle string) decisionlive.DecisionLiveEvent {
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseDone)
	sit := strings.TrimSpace(srcTitle)
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseDone,
		OutcomeKind:      decisionlive.OutcomeSuccess,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		ReasoningSummary: frame,
		DetailBullets:    nil,
	}
}

// BuildLLMEmptySuggestions — PhaseLLM khi không có gợi ý ban đầu.
func BuildLLMEmptySuggestions(correlationID string) decisionlive.DecisionLiveEvent {
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseLLM)
	sit := "Chưa có danh sách gợi ý từ phân tích — bổ sung bằng AI trong tập hành động được phép."
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseLLM,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		ReasoningSummary: frame,
		DetailBullets:    []string{sit},
		Step: &decisionlive.TraceStep{
			Kind:      "llm",
			Title:     frame,
			Reasoning: frame,
		},
	}
}

// BuildLLMRefineSuggestions — PhaseLLM khi tinh chỉnh nhiều gợi ý.
func BuildLLMRefineSuggestions(correlationID string, suggestionCount int) decisionlive.DecisionLiveEvent {
	frame := PublishCatalogUserViForLivePhase(decisionlive.PhaseLLM)
	sit := fmt.Sprintf("Đang rút gọn từ %d gợi ý ban đầu (AI).", suggestionCount)
	return decisionlive.DecisionLiveEvent{
		Phase:            decisionlive.PhaseLLM,
		OutcomeKind:      decisionlive.OutcomeNominal,
		Summary:          PublishWithSituation(frame, sit),
		CorrelationID:    correlationID,
		ReasoningSummary: frame,
		DetailBullets:    []string{fmt.Sprintf("Số gợi ý ban đầu: %d", suggestionCount)},
		Step: &decisionlive.TraceStep{
			Kind:      "llm",
			Title:     frame,
			InputRef:  map[string]interface{}{"suggestionCount": suggestionCount},
			Reasoning: frame,
		},
	}
}
