package eventtypes

import "testing"

func TestResolveE2EForQueueEnvelope_Datachanged(t *testing.T) {
	r := ResolveE2EForQueueEnvelope("conversation.inserted", EventSourceDatachanged, PipelineStageAfterL1Change)
	if r.Stage != E2EStageG1 || r.StepID != "G1-S04" {
		t.Fatalf("datachanged conversation: got %+v", r)
	}
}

func TestResolveE2EForQueueEnvelope_L1Datachanged(t *testing.T) {
	r := ResolveE2EForQueueEnvelope("conversation.inserted", EventSourceL1Datachanged, PipelineStageAfterL1Change)
	if r.Stage != E2EStageG1 || r.StepID != "G1-S04" {
		t.Fatalf("l1_datachanged conversation: got %+v", r)
	}
}

func TestResolveE2EForQueueEnvelope_LegacyPipelineStageAfterSourcePersist(t *testing.T) {
	r := ResolveE2EForQueueEnvelope("conversation.inserted", EventSourceDatachanged, "after_source_persist")
	if r.Stage != E2EStageG1 || r.StepID != "G1-S04" {
		t.Fatalf("legacy after_source_persist: got %+v", r)
	}
}

func TestResolveE2EForQueueConsumerMilestone_G2(t *testing.T) {
	r := ResolveE2EForQueueConsumerMilestone(
		CrmIntelRecomputed, EventSourceCrmIntel, PipelineStageDomainIntel,
		E2EQueueMilestoneProcessingStart,
	)
	if r.Stage != E2EStageG2 || r.StepID != "G2-S01" {
		t.Fatalf("consumer milestone override: got %+v", r)
	}
}

func TestResolveE2EForLivePhase_Queued(t *testing.T) {
	r := ResolveE2EForLivePhase("queued")
	if r.Stage != E2EStageG4 || r.StepID != "G4-S05-E01" {
		t.Fatalf("queued phase: got %+v", r)
	}
}

func TestResolveE2EForLivePhase_Orchestrate(t *testing.T) {
	r := ResolveE2EForLivePhase("orchestrate")
	if r.StepID != "G4-S01" {
		t.Fatalf("orchestrate: got %+v", r)
	}
}

func TestResolveE2EForLivePhase_IntelDomainWorker(t *testing.T) {
	r := ResolveE2EForLivePhase("intel_domain_compute_start")
	if r.Stage != E2EStageG3 || r.StepID != "G3-S03" {
		t.Fatalf("intel_domain_compute_start: got %+v", r)
	}
}
