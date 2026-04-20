package livecopy

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// PublishCatalogUserViForLivePhase — một dòng khung §5.3 (descriptionUserVi) theo phase live (ResolveE2EForLivePhase).
func PublishCatalogUserViForLivePhase(phase string) string {
	ref := eventtypes.ResolveE2EForLivePhase(strings.TrimSpace(phase))
	if ref.StepID != "" {
		if s := eventtypes.E2ECatalogDescriptionUserViForStep(ref.StepID); s != "" {
			return s
		}
	}
	return strings.TrimSpace(ref.LabelVi)
}

// PublishCatalogUserViForQueueConsumerMilestone — khung catalog cho mốc consumer G2 (milestone ưu tiên trên envelope).
func PublishCatalogUserViForQueueConsumerMilestone(evt *aidecisionmodels.DecisionEvent, milestoneKey string) string {
	et, es, ps := "", "", ""
	if evt != nil {
		et = evt.EventType
		es = evt.EventSource
		ps = evt.PipelineStage
	}
	ref := eventtypes.ResolveE2EForQueueConsumerMilestone(et, es, ps, strings.TrimSpace(milestoneKey))
	if ref.StepID != "" {
		if s := eventtypes.E2ECatalogDescriptionUserViForStep(ref.StepID); s != "" {
			return s
		}
	}
	return strings.TrimSpace(ref.LabelVi)
}

// PublishWithSituation — nối khung catalog với phần tình huống (mã, đếm, lỗi rút gọn…); suffix rỗng thì chỉ catalog.
func PublishWithSituation(catalogFramework, situationVi string) string {
	catalogFramework = strings.TrimSpace(catalogFramework)
	situationVi = strings.TrimSpace(situationVi)
	if situationVi == "" {
		return catalogFramework
	}
	if catalogFramework == "" {
		return situationVi
	}
	return catalogFramework + " — " + situationVi
}

// PublishReasoningCatalogPlusEngineLine — ReasoningSummary: khung catalog + (tuỳ chọn) một dòng suy luận engine.
func PublishReasoningCatalogPlusEngineLine(phase, engineReasoningOneLine string) string {
	return PublishWithSituation(PublishCatalogUserViForLivePhase(phase), publishTruncateOneLine(engineReasoningOneLine, 280))
}

func publishTruncateOneLine(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
