package decisionlive

import "testing"

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
