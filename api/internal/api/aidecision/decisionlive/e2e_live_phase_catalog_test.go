package decisionlive

import "testing"

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
