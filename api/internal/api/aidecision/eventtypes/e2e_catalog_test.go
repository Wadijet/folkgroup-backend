package eventtypes

import (
	"strings"
	"testing"
)

func TestE2EStageCatalog_Len6(t *testing.T) {
	s := E2EStageCatalog()
	if len(s) != 6 {
		t.Fatalf("mong 6 giai đoạn G1–G6, got %d", len(s))
	}
	if s[0].ID != E2EStageG1 || s[5].ID != E2EStageG6 {
		t.Fatalf("đầu/cuối sai: %s / %s", s[0].ID, s[5].ID)
	}
	for i, row := range s {
		if row.UserSummaryVi == "" {
			t.Fatalf("stage[%d] %s thiếu userSummaryVi", i, row.ID)
		}
	}
}

func TestE2EStepCatalog_G1S04_DungMotDong(t *testing.T) {
	var n int
	for _, row := range E2EStepCatalog() {
		if row.StageID == E2EStageG1 && row.StepID == "G1-S04" {
			n++
			if row.EventDetailID != "" {
				t.Fatalf("G1-S04 catalog: không dùng eventDetailId tách E, got %q", row.EventDetailID)
			}
		}
	}
	if n != 1 {
		t.Fatalf("mong đúng 1 dòng G1-S04 trong catalog, got %d", n)
	}
}

func TestE2EStepCatalog_G3S01_DungMotDong(t *testing.T) {
	var n int
	for _, row := range E2EStepCatalog() {
		if row.StageID == E2EStageG3 && row.StepID == "G3-S01" {
			n++
			if row.EventDetailID != "" {
				t.Fatalf("G3-S01 catalog: không dùng eventDetailId tách E, got %q", row.EventDetailID)
			}
		}
	}
	if n != 1 {
		t.Fatalf("mong đúng 1 dòng G3-S01 trong catalog, got %d", n)
	}
}

func TestE2EStepCatalog_G3S06_DungMotDong(t *testing.T) {
	var n int
	for _, row := range E2EStepCatalog() {
		if row.StageID == E2EStageG3 && row.StepID == "G3-S06" {
			n++
			if row.EventDetailID != "" {
				t.Fatalf("G3-S06 catalog: không dùng eventDetailId tách E, got %q", row.EventDetailID)
			}
		}
	}
	if n != 1 {
		t.Fatalf("mong đúng 1 dòng G3-S06 trong catalog, got %d", n)
	}
}

func TestE2EStepCatalog_G4S02_DungMotDong(t *testing.T) {
	var n int
	for _, row := range E2EStepCatalog() {
		if row.StageID == E2EStageG4 && row.StepID == "G4-S02" {
			n++
			if row.EventDetailID != "" {
				t.Fatalf("G4-S02 catalog: không dùng eventDetailId tách E, got %q", row.EventDetailID)
			}
		}
	}
	if n != 1 {
		t.Fatalf("mong đúng 1 dòng G4-S02 trong catalog, got %d", n)
	}
}

func TestE2EStepCatalog_NotEmpty(t *testing.T) {
	st := E2EStepCatalog()
	// G3-S01 / G3-S06 / G4-S02 gộp một dòng; v34: bỏ dòng catalog G4 message.batch_ready (32 dòng); v33: G4-S02 gom context; v31: *_intel_recomputed → G4-S01.
	if len(st) < 32 {
		t.Fatalf("bảng §5.3 mong ≥32 dòng, got %d", len(st))
	}
	for i, row := range st {
		if row.DescriptionTechnicalVi == "" || row.DescriptionUserVi == "" {
			t.Fatalf("step[%d] %s/%s thiếu descriptionTechnicalVi hoặc descriptionUserVi", i, row.StageID, row.StepID)
		}
	}
}

func TestE2EStepCatalog_G2S02_CoGomVaGap(t *testing.T) {
	for _, row := range E2EStepCatalog() {
		if row.StageID != E2EStageG2 || row.StepID != "G2-S02" {
			continue
		}
		for _, s := range []struct {
			name, field string
		}{
			{"descriptionTechnicalVi", row.DescriptionTechnicalVi},
			{"descriptionUserVi", row.DescriptionUserVi},
		} {
			if !strings.Contains(s.field, "gom") || !strings.Contains(s.field, "gấp") {
				t.Fatalf("G2-S02 %s phải nêu rõ gom và gấp, got %q", s.name, s.field)
			}
		}
		return
	}
	t.Fatal("không tìm thấy dòng G2-S02 trong catalog")
}

func TestE2EQueueMilestoneCatalog_G2(t *testing.T) {
	q := E2EQueueMilestoneCatalog()
	if len(q) != 6 {
		t.Fatalf("mong 6 milestone consumer, got %d", len(q))
	}
	for _, row := range q {
		if row.StageID != E2EStageG2 {
			t.Fatalf("milestone %q phải G2 (pha merge — consumer), got %q", row.Key, row.StageID)
		}
		if row.UserLabelVi == "" {
			t.Fatalf("milestone %q thiếu userLabelVi", row.Key)
		}
	}
}
