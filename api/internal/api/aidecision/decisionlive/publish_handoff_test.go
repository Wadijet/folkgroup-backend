package decisionlive

import (
	"strings"
	"testing"
)

func TestEnrichPublishHandoffNarrative_AIDRefs(t *testing.T) {
	ev := DecisionLiveEvent{
		DetailBullets: []string{"Đã xử lý xong."},
		Refs: map[string]string{
			"eventType":    "cix.analysis_requested",
			"eventSource":  "aidecision",
			"pipelineStage": "aid_coordination",
		},
	}
	enrichPublishE2ERef(&ev)
	enrichPublishHandoffNarrative(&ev)
	if len(ev.DetailBullets) < 2 {
		t.Fatalf("bullets: %v", ev.DetailBullets)
	}
	if !strings.Contains(ev.DetailBullets[1], "Bước chuyển:") || !strings.Contains(ev.DetailBullets[1], "CIX") {
		t.Fatalf("expected handoff after E2E, got: %v", ev.DetailBullets)
	}
}

func TestEnrichPublishHandoffNarrative_ExplicitNote(t *testing.T) {
	ev := DecisionLiveEvent{
		DetailBullets: []string{"Ghi nhận."},
		Refs: map[string]string{
			"handoffNoteVi": "Bước chuyển: thử nghiệm — chỉ kiểm thử.",
		},
	}
	enrichPublishHandoffNarrative(&ev)
	if !strings.Contains(strings.Join(ev.DetailBullets, " "), "thử nghiệm") {
		t.Fatalf("bullets: %v", ev.DetailBullets)
	}
}

func TestEnrichPublishHandoffNarrative_JobType(t *testing.T) {
	ev := DecisionLiveEvent{
		DetailBullets: []string{"Xong."},
		Step: &TraceStep{
			OutputRef: map[string]interface{}{"jobType": "crm_intel_compute"},
		},
	}
	enrichPublishHandoffNarrative(&ev)
	if !strings.Contains(ev.DetailBullets[0], "CRM") {
		t.Fatalf("bullets: %v", ev.DetailBullets)
	}
}

func TestDetailBulletsContainHandoffNarrative_Dedup(t *testing.T) {
	ev := DecisionLiveEvent{
		DetailBullets: []string{"Bước chuyển: đã có sẵn."},
	}
	enrichPublishHandoffNarrative(&ev)
	if len(ev.DetailBullets) != 1 {
		t.Fatalf("expected no duplicate, got %v", ev.DetailBullets)
	}
}
