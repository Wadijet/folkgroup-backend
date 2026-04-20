package decisionlive

import (
	"testing"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

func TestE2ELivePhaseCatalog_CoversPhases(t *testing.T) {
	rows := E2ELivePhaseCatalog()
	if len(rows) < 20 {
		t.Fatalf("mong đủ phase hằng số, got %d", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.LivePhase == "" {
			t.Fatal("livePhase rỗng")
		}
		seen[r.LivePhase] = true
		if r.LabelVi == "" {
			t.Fatalf("thiếu label cho phase %q", r.LivePhase)
		}
	}
	if !seen[PhaseQueueProcessing] || !seen[PhaseLLM] {
		t.Fatal("thiếu phase queue hoặc llm")
	}
}

func TestE2ELivePhaseCatalog_E2EStepIdNamTrongStepCatalog(t *testing.T) {
	valid := eventtypes.E2ECatalogResolvedStepIDs()
	for _, row := range E2ELivePhaseCatalog() {
		if row.E2EStepID == "" {
			t.Fatalf("phase %q: thiếu e2eStepId", row.LivePhase)
		}
		if _, ok := valid[row.E2EStepID]; !ok {
			t.Fatalf("phase %q: e2eStepId %q không có trong E2EStepCatalog (eventDetailId/stepId)", row.LivePhase, row.E2EStepID)
		}
	}
}
