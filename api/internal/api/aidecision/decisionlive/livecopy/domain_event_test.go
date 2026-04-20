package livecopy

import (
	"testing"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

func TestDomainNarrativeFromQueueEvent_MetaCampaign(t *testing.T) {
	evt := &aidecisionmodels.DecisionEvent{
		EventType:   eventtypes.CampaignIntelRecomputed,
		EventSource: eventtypes.EventSourceMetaAdsIntel,
		Payload: map[string]interface{}{
			"campaignId": "c1",
		},
	}
	d := DomainNarrativeFromQueueEvent(evt)
	if d.StepTitle == "" {
		t.Fatal("StepTitle rỗng")
	}
	if d.BusinessOneLine == "" {
		t.Fatal("BusinessOneLine rỗng")
	}
	want := eventtypes.E2ECatalogDescriptionUserViForStep("G4-S01")
	if want != "" && d.StepTitle != want {
		t.Fatalf("khung catalog G4-S01, got %q want %q", d.StepTitle, want)
	}
	if d.StepTitle != d.BusinessOneLine {
		t.Fatalf("StepTitle và BusinessOneLine cùng khung catalog, got %q / %q", d.StepTitle, d.BusinessOneLine)
	}
}

func TestDomainNarrativeFromQueueEvent_KhongConTieuDeChungChung(t *testing.T) {
	evt := &aidecisionmodels.DecisionEvent{
		EventType:     eventtypes.ConversationChanged,
		EventSource:   eventtypes.EventSourceL1Datachanged,
		PipelineStage: eventtypes.PipelineStageAfterL1Change,
	}
	d := DomainNarrativeFromQueueEvent(evt)
	if d.StepTitle == "" {
		t.Fatal("StepTitle rỗng")
	}
	if d.StepTitle == "Đang xử lý tự động" {
		t.Fatalf("không còn tiêu đề chung chung, got %q", d.StepTitle)
	}
	want := eventtypes.E2ECatalogDescriptionUserViForStep("G1-S04")
	if want != "" && d.StepTitle != want {
		t.Fatalf("neo khung G1-S04 catalog, got %q want %q", d.StepTitle, want)
	}
}

func TestBuildQueueConsumerEvent_Done(t *testing.T) {
	evt := &aidecisionmodels.DecisionEvent{
		EventID:     "evt_x",
		EventType:   eventtypes.OrderUpdated,
		EventSource: eventtypes.EventSourceDatachanged,
	}
	ev := BuildQueueConsumerEvent(evt, QueueMilestoneHandlerDone, nil, nil, nil)
	if ev.Phase == "" {
		t.Fatal("phase")
	}
	if len(ev.DetailBullets) < 1 {
		t.Fatalf("detail bullets: %v", ev.DetailBullets)
	}
	if len(ev.DetailSections) != 1 {
		t.Fatalf("mong đợi 1 detailSection (chi tiết kỹ thuật gộp): %+v", ev.DetailSections)
	}
	if len(ev.DetailSections[0].Items) < 2 {
		t.Fatalf("chi tiết kỹ thuật phải có ít nhất 2 dòng: %+v", ev.DetailSections)
	}
}
