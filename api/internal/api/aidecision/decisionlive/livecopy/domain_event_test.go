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
