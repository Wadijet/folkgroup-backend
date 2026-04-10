package eventtypes

import "testing"

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

func TestE2EStepCatalog_NotEmpty(t *testing.T) {
	st := E2EStepCatalog()
	if len(st) < 40 {
		t.Fatalf("bảng §5.3 mong ≥40 dòng, got %d", len(st))
	}
	for i, row := range st {
		if row.DescriptionTechnicalVi == "" || row.DescriptionUserVi == "" {
			t.Fatalf("step[%d] %s/%s thiếu descriptionTechnicalVi hoặc descriptionUserVi", i, row.StageID, row.StepID)
		}
	}
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
