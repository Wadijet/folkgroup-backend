package decisionlive

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventopstier"
)

// enrichLiveEventOpsTier — Publish bước 2: điền opsTier / opsTierLabelVi trước metrics và fan-out.
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
		if isIntelDomainComputePhase(ev.Phase) {
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

// backfillLiveEventsDerivedFields — Bước cuối khi trả timeline (sau snapshot RAM hoặc đọc Mongo): suy opsTier + feedSource nếu thiếu (bản ghi cũ).
func backfillLiveEventsDerivedFields(events []DecisionLiveEvent) {
	for i := range events {
		BackfillW3CTraceIDOnly(&events[i])
		enrichLiveEventOpsTier(&events[i])
		enrichLiveEventFeedSource(&events[i])
		enrichLiveBusinessDomain(&events[i])
		EnrichLiveOutcomeMetadata(&events[i])
		enrichLiveEventUIPresentation(&events[i])
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

func isIntelDomainComputePhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case PhaseIntelDomainComputeStart, PhaseIntelDomainComputeDone, PhaseIntelDomainComputeError:
		return true
	default:
		return false
	}
}
