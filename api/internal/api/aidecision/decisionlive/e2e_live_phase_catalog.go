// e2e_live_phase_catalog — Map phase timeline (DecisionLiveEvent.phase) → tham chiếu E2E; dùng cho API catalog frontend.
package decisionlive

import "meta_commerce/internal/api/aidecision/eventtypes"

// E2ELivePhaseCatalogRow — một dòng map phase live → Gx / Gx-Syy (khớp eventtypes.ResolveE2EForLivePhase).
type E2ELivePhaseCatalogRow struct {
	LivePhase string `json:"livePhase"`
	E2EStage  string `json:"e2eStage"`
	E2EStepID string `json:"e2eStepId"`
	LabelVi   string `json:"e2eStepLabelVi"`
}

// E2ELivePhaseCatalog — tất cả phase hằng số trong package (để UI tooltip / lọc đồng bộ resolver).
func E2ELivePhaseCatalog() []E2ELivePhaseCatalogRow {
	phases := []string{
		PhaseQueued, PhaseConsuming, PhaseSkipped, PhaseParse, PhaseLLM, PhaseDecision,
		PhasePolicy, PhasePropose, PhaseEmpty, PhaseDone, PhaseError,
		PhaseQueueProcessing, PhaseQueueDone, PhaseQueueError, PhaseDatachangedEffects,
		PhaseOrchestrate, PhaseCixIntegrated, PhaseExecuteReady, PhaseAdsEvaluate,
		PhaseIntelDomainComputeStart, PhaseIntelDomainComputeDone, PhaseIntelDomainComputeError,
	}
	out := make([]E2ELivePhaseCatalogRow, 0, len(phases))
	for _, p := range phases {
		r := eventtypes.ResolveE2EForLivePhase(p)
		out = append(out, E2ELivePhaseCatalogRow{
			LivePhase: p,
			E2EStage:  r.Stage,
			E2EStepID: r.StepID,
			LabelVi:   r.LabelVi,
		})
	}
	return out
}
