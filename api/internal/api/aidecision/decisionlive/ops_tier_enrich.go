package decisionlive

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventopstier"
)

// enrichLiveEventOpsTier điền opsTier / opsTierLabelVi trước khi buffer / fan-out (gọi từ Publish).
func enrichLiveEventOpsTier(ev *DecisionLiveEvent) {
	if ev == nil || strings.TrimSpace(ev.OpsTier) != "" {
		return
	}
	et := ""
	if ev.Refs != nil {
		et = strings.TrimSpace(ev.Refs["eventType"])
		if et == "" {
			et = strings.TrimSpace(ev.Refs["event_type"])
		}
	}
	if et == "" && ev.SourceKind == SourceQueue && strings.TrimSpace(ev.SourceTitle) != "" {
		et = strings.TrimSpace(ev.SourceTitle)
	}
	if et == "" {
		if isOrchestrationPipelinePhase(ev.Phase) {
			ev.OpsTier = eventopstier.TierPipeline
			ev.OpsTierLabelVi = eventopstier.LabelVi(ev.OpsTier)
			return
		}
		if isExecutePipelinePhase(ev.Phase) {
			ev.OpsTier = eventopstier.TierDecision
			ev.OpsTierLabelVi = eventopstier.LabelVi(ev.OpsTier)
			return
		}
		ev.OpsTier = eventopstier.TierUnknown
		ev.OpsTierLabelVi = eventopstier.LabelVi(ev.OpsTier)
		return
	}
	ev.OpsTier, ev.OpsTierLabelVi = eventopstier.ClassifyEventType(et)
}

// backfillLiveEventsDerivedFields đảm bảo opsTier + feedSource khi trả replay/API (Mongo cũ, buffer trước khi có field).
func backfillLiveEventsDerivedFields(events []DecisionLiveEvent) {
	for i := range events {
		BackfillW3CTraceIDOnly(&events[i])
		enrichLiveEventOpsTier(&events[i])
		enrichLiveEventFeedSource(&events[i])
	}
}

func isOrchestrationPipelinePhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case PhaseOrchestrate, PhaseCixIntegrated, PhaseExecuteReady:
		return true
	default:
		return false
	}
}

func isExecutePipelinePhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case PhaseQueued, PhaseConsuming, PhaseSkipped, PhaseParse, PhaseLLM, PhaseDecision,
		PhasePolicy, PhasePropose, PhaseEmpty, PhaseDone, PhaseError:
		return true
	default:
		return false
	}
}
