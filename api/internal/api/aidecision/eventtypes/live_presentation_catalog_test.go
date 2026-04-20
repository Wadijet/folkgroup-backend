package eventtypes

import "testing"

func TestResolveLivePhaseLabelVi_EmptyAndUnknown(t *testing.T) {
	if got := ResolveLivePhaseLabelVi(""); got != "Bước luồng" {
		t.Fatalf("phase rỗng: muốn %q, got %q", "Bước luồng", got)
	}
	if got := ResolveLivePhaseLabelVi("phase_khong_ton_tai"); got != "phase_khong_ton_tai" {
		t.Fatalf("phase lạ: muốn giữ nguyên, got %q", got)
	}
}

func TestResolveLivePhaseLabelVi_UuTienE2E(t *testing.T) {
	want := E2ECatalogDescriptionUserViForStep("G4-S03")
	if want == "" {
		t.Fatalf("catalog G4-S03 rỗng")
	}
	if got := ResolveLivePhaseLabelVi("ads_evaluate"); got != want {
		t.Fatalf("ads_evaluate: muốn lấy text catalog G4-S03, got %q", got)
	}
}

func TestResolveLiveOutcomeLabelVi(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{kind: "nominal", want: "Đang xử lý"},
		{kind: "success", want: "Hoàn tất"},
		{kind: "processing_error", want: "Lỗi xử lý"},
		{kind: "khong_ton_tai", want: "Khác"},
	}
	for _, tc := range cases {
		if got := ResolveLiveOutcomeLabelVi(tc.kind); got != tc.want {
			t.Fatalf("kind=%s: want=%q got=%q", tc.kind, tc.want, got)
		}
	}
}

func TestResolveLiveFeedSourceLabelVi(t *testing.T) {
	cases := []struct {
		category string
		want     string
	}{
		{category: "conversation", want: "Hội thoại"},
		{category: "crm", want: "CRM"},
		{category: "queue", want: "Hàng đợi"},
		{category: "khong_ton_tai", want: "Khác"},
	}
	for _, tc := range cases {
		if got := ResolveLiveFeedSourceLabelVi(tc.category); got != tc.want {
			t.Fatalf("category=%s: want=%q got=%q", tc.category, tc.want, got)
		}
	}
}

func TestResolveLiveBusinessDomainLabelVi(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{code: "", want: "Chưa rõ module"},
		{code: "aidecision", want: "AI Decision"},
		{code: "CRM", want: "CRM"},
		{code: "khong_ton_tai", want: "khong_ton_tai"},
	}
	for _, tc := range cases {
		if got := ResolveLiveBusinessDomainLabelVi(tc.code); got != tc.want {
			t.Fatalf("code=%s: want=%q got=%q", tc.code, tc.want, got)
		}
	}
}

func TestResolveLiveE2EPublishNarrative(t *testing.T) {
	ref := E2ERef{StepID: "G2-S01", LabelVi: "Đang xử lý"}
	got := ResolveLiveE2EPublishNarrative(ref)
	want := "Trong quy trình: G2-S01 — Đang xử lý."
	if got != want {
		t.Fatalf("want=%q got=%q", want, got)
	}
}

func TestResolveLiveHandoffLine(t *testing.T) {
	if got := ResolveLiveHandoffLineFromDomainVi("CRM / khách hàng"); got != "Bước chuyển: CRM / khách hàng." {
		t.Fatalf("handoff domain: got %q", got)
	}
	got := ResolveLiveHandoffLineFromAIDEvent("aidecision", "crm.intelligence.recompute_requested")
	if got == "" {
		t.Fatalf("handoff event: expected non-empty line")
	}
	if got := ResolveLiveHandoffLineFromJobType("crm_intel_compute"); got == "" {
		t.Fatalf("handoff jobType: expected non-empty line")
	}
}

func TestResolveLiveQueueEventTypeLabelVi(t *testing.T) {
	cases := []struct {
		eventType string
		want      string
	}{
		{eventType: OrderChanged, want: "Đơn hàng"},
		{eventType: CixIntelRecomputed, want: "Phân tích hội thoại"},
		{eventType: AdsContextRequested, want: "Ngữ cảnh quảng cáo"},
		{eventType: "khac.bat_ky", want: "Cập nhật tự động"},
		{eventType: "", want: "Cập nhật hệ thống"},
	}
	for _, tc := range cases {
		if got := ResolveLiveQueueEventTypeLabelVi(tc.eventType); got != tc.want {
			t.Fatalf("eventType=%s: want=%q got=%q", tc.eventType, tc.want, got)
		}
	}
}
