package decisionlive

import (
	"strings"
	"testing"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

func TestEnrichLiveEventUIPresentation(t *testing.T) {
	ev := DecisionLiveEvent{
		Phase:            PhaseQueueProcessing,
		SourceTitle:      "fb_customer.updated",
		Summary:          "Đang bắt đầu xử lý.",
		ReasoningSummary: "Lý do phụ (fallback).",
	}
	enrichLiveEventUIPresentation(&ev)
	if ev.PhaseLabelVi != "Hệ thống vừa nhận việc" {
		t.Fatalf("phaseLabelVi: %q", ev.PhaseLabelVi)
	}
	if ev.UiTitle == "" {
		t.Fatal("thiếu uiTitle")
	}
	if ev.UiSummary != "Đang bắt đầu xử lý." {
		t.Fatalf("uiSummary: %q", ev.UiSummary)
	}
	ev2 := DecisionLiveEvent{
		Phase:            PhaseQueueProcessing,
		Summary:          "",
		ReasoningSummary: "Chỉ có reasoning.",
	}
	enrichLiveEventUIPresentation(&ev2)
	if ev2.UiSummary != "Chỉ có reasoning." {
		t.Fatalf("uiSummary fallback: %q", ev2.UiSummary)
	}
}

func TestPhaseLabelVi_IntelDomainKhongTraVeChuoiPhaseTiengAnh(t *testing.T) {
	cases := []struct {
		phase string
		want  string
	}{
		{PhaseIntelDomainComputeStart, eventtypes.E2ECatalogDescriptionUserViForStep("G3-S03")},
		{PhaseIntelDomainComputeDone, eventtypes.E2ECatalogDescriptionUserViForStep("G3-S05")},
		{PhaseIntelDomainComputeError, eventtypes.ResolveLivePhaseLabelVi(PhaseIntelDomainComputeError)},
	}
	for _, c := range cases {
		got := phaseLabelVi(c.phase)
		if got == "" || got == c.phase {
			t.Fatalf("phase %q: mong nhãn tiếng Việt, got %q", c.phase, got)
		}
		if strings.TrimSpace(c.want) != "" && got != c.want {
			t.Fatalf("phase %q: got %q want %q", c.phase, got, c.want)
		}
	}
}

func TestPhaseLabelVi_AdsEvaluateDungMoTaCatalogG4S03(t *testing.T) {
	want := eventtypes.E2ECatalogDescriptionUserViForStep("G4-S03")
	got := phaseLabelVi(PhaseAdsEvaluate)
	if want != "" && got != want {
		t.Fatalf("ads_evaluate: got %q want %q", got, want)
	}
}
