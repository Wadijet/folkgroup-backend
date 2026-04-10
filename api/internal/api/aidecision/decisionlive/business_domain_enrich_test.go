package decisionlive

import (
	"testing"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

func TestEnrichLiveBusinessDomain_IntelMergeCRM(t *testing.T) {
	ev := DecisionLiveEvent{
		Phase: PhaseIntelDomainComputeStart,
		Refs: map[string]string{
			"intelDomain": IntelDomainCrmPendingMerge,
			"workerLane":  "domain_worker",
		},
	}
	enrichLiveBusinessDomain(&ev)
	if ev.BusinessDomain != BusinessDomainCRM {
		t.Fatalf("got %q", ev.BusinessDomain)
	}
	if ev.BusinessDomainLabelVi != "CRM (crm)" {
		t.Fatalf("label got %q", ev.BusinessDomainLabelVi)
	}
}

func TestEnrichLiveBusinessDomain_QueueCrmRecompute(t *testing.T) {
	ev := DecisionLiveEvent{
		Phase:      PhaseQueueDone,
		SourceKind: SourceQueue,
		Refs: map[string]string{
			"eventType":   eventtypes.CrmIntelligenceRecomputeRequested,
			"eventSource": "crm_merge_queue",
		},
	}
	enrichLiveEventFeedSource(&ev)
	enrichLiveBusinessDomain(&ev)
	if ev.BusinessDomain != BusinessDomainAIDecision {
		t.Fatalf("got %q want aidecision (consumer decision_events_queue)", ev.BusinessDomain)
	}
	if ev.BusinessDomainLabelVi != "AI Decision (aidecision)" {
		t.Fatalf("label got %q", ev.BusinessDomainLabelVi)
	}
}

func TestEnrichLiveBusinessDomain_PosSync(t *testing.T) {
	ev := DecisionLiveEvent{
		Phase:      PhaseQueueProcessing,
		SourceKind: SourceQueue,
		Refs: map[string]string{
			"eventType": "pos_product.updated",
		},
	}
	enrichLiveEventFeedSource(&ev)
	enrichLiveBusinessDomain(&ev)
	if ev.BusinessDomain != BusinessDomainAIDecision {
		t.Fatalf("got %q want aidecision (consumer decision_events_queue)", ev.BusinessDomain)
	}
	if ev.BusinessDomainLabelVi != "AI Decision (aidecision)" {
		t.Fatalf("label got %q", ev.BusinessDomainLabelVi)
	}
}

func TestEnrichLiveBusinessDomain_RefsOverride(t *testing.T) {
	ev := DecisionLiveEvent{
		Refs: map[string]string{"businessDomain": BusinessDomainOrder},
	}
	enrichLiveBusinessDomain(&ev)
	if ev.BusinessDomain != BusinessDomainOrder {
		t.Fatalf("got %q", ev.BusinessDomain)
	}
	if ev.BusinessDomainLabelVi != "Đơn hàng (order)" {
		t.Fatalf("label got %q", ev.BusinessDomainLabelVi)
	}
}

func TestEnrichLiveBusinessDomain_ExecutePipeline(t *testing.T) {
	ev := DecisionLiveEvent{Phase: PhaseLLM}
	enrichLiveEventFeedSource(&ev)
	enrichLiveBusinessDomain(&ev)
	if ev.BusinessDomain != BusinessDomainAIDecision {
		t.Fatalf("got %q", ev.BusinessDomain)
	}
	if ev.BusinessDomainLabelVi != "AI Decision (aidecision)" {
		t.Fatalf("label got %q", ev.BusinessDomainLabelVi)
	}
}
