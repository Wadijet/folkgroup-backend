package eventtypes

import "testing"

func TestResolveE2EForQueueEnvelope_Datachanged(t *testing.T) {
	r := ResolveE2EForQueueEnvelope("conversation.changed", EventSourceDatachanged, PipelineStageAfterL1Change)
	if r.Stage != E2EStageG1 || r.StepID != "G1-S04" {
		t.Fatalf("datachanged conversation: got %+v", r)
	}
}

func TestResolveE2EForQueueEnvelope_L1Datachanged(t *testing.T) {
	r := ResolveE2EForQueueEnvelope("conversation.changed", EventSourceL1Datachanged, PipelineStageAfterL1Change)
	if r.Stage != E2EStageG1 || r.StepID != "G1-S04" {
		t.Fatalf("l1_datachanged conversation: got %+v", r)
	}
}

func TestResolveE2EForQueueEnvelope_L2DatachangedAfterMerge(t *testing.T) {
	r := ResolveE2EForQueueEnvelope("pos_customer.changed", EventSourceL2Datachanged, PipelineStageAfterL2Merge)
	if r.Stage != E2EStageG2 || r.StepID != "G2-S05-E01" {
		t.Fatalf("l2_datachanged sau merge: got %+v", r)
	}
}

func TestResolveE2EForQueueEnvelope_LegacyCrmMergeQueueRecompute(t *testing.T) {
	r := ResolveE2EForQueueEnvelope(CrmIntelligenceRecomputeRequested, EventSourceCrmMergeQueue, PipelineStageAfterL2Merge)
	if r.Stage != E2EStageG2 || r.StepID != "G2-S05-E01" {
		t.Fatalf("legacy crm_merge_queue: got %+v", r)
	}
}

func TestResolveE2EForQueueEnvelope_CrmRecomputeNeoG3S01(t *testing.T) {
	r := ResolveE2EForQueueEnvelope(CrmIntelligenceRecomputeRequested, EventSourceCRM, PipelineStageAfterL1Change)
	if r.Stage != E2EStageG3 || r.StepID != "G3-S01" {
		t.Fatalf("crm recompute phải neo G3-S01 (một dòng catalog): got %+v", r)
	}
}

func TestResolveE2EForQueueEnvelope_IntelRecomputedNeoG4S01(t *testing.T) {
	for _, et := range []string{CixIntelRecomputed, CrmIntelRecomputed, OrderIntelRecomputed, CampaignIntelRecomputed} {
		r := ResolveE2EForQueueEnvelope(et, EventSourceCrmIntel, PipelineStageDomainIntel)
		if r.Stage != E2EStageG4 || r.StepID != "G4-S01" {
			t.Fatalf("%s phải neo G4-S01 (AID nhận handoff intel): got %+v", et, r)
		}
	}
}

func TestResolveE2EForQueueEnvelope_ContextNeoG4S02MotDong(t *testing.T) {
	cases := []struct {
		et string
		es string
	}{
		{CustomerContextRequested, EventSourceAIDecision},
		{CustomerContextReady, EventSourceCRM},
		{AdsContextRequested, EventSourceAIDecision},
		{AdsContextReady, EventSourceMetaAdsIntel},
	}
	for _, c := range cases {
		r := ResolveE2EForQueueEnvelope(c.et, c.es, PipelineStageAIDCoordination)
		if r.Stage != E2EStageG4 || r.StepID != "G4-S02" {
			t.Fatalf("%s / %s phải neo G4-S02 (catalog một dòng): got %+v", c.et, c.es, r)
		}
	}
}

func TestResolveE2EForQueueEnvelope_LegacyPipelineStageAfterSourcePersist(t *testing.T) {
	r := ResolveE2EForQueueEnvelope("conversation.changed", EventSourceDatachanged, "after_source_persist")
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

func TestResolveE2EForQueueConsumerMilestone_HandlerDoneNeoG2S02(t *testing.T) {
	r := ResolveE2EForQueueConsumerMilestone(
		"conversation.changed", EventSourceL1Datachanged, PipelineStageAfterL1Change,
		E2EQueueMilestoneHandlerDone,
	)
	if r.Stage != E2EStageG2 || r.StepID != "G2-S02" {
		t.Fatalf("handler_done phải neo G2-S02 (catalog gộp consumer): got %+v", r)
	}
}

func TestResolveE2EForLivePhase_Queued(t *testing.T) {
	r := ResolveE2EForLivePhase("queued")
	if r.Stage != E2EStageG4 || r.StepID != "G4-S03-E01" {
		t.Fatalf("queued phase: got %+v", r)
	}
}

func TestResolveE2EForQueueEnvelope_MessageBatchReadyNeoG2S02(t *testing.T) {
	r := ResolveE2EForQueueEnvelope(MessageBatchReady, EventSourceDebounce, PipelineStageAIDCoordination)
	if r.Stage != E2EStageG2 || r.StepID != "G2-S02" {
		t.Fatalf("message.batch_ready phải neo G2-S02 (consumer processEvent), không còn catalog G4: got %+v", r)
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

func TestResolveE2EForLivePhase_IntelDomainComputeDoneNeoG3S05(t *testing.T) {
	r := ResolveE2EForLivePhase("intel_domain_compute_done")
	if r.Stage != E2EStageG3 || r.StepID != "G3-S05" {
		t.Fatalf("intel_domain_compute_done phải neo G3-S05 (persist sau worker): got %+v", r)
	}
}

func TestResolveE2EForLivePhase_UnknownPhaseKhongSinhStepIdCatalog(t *testing.T) {
	r := ResolveE2EForLivePhase("phase_khong_ton_tai_xyz")
	if r.StepID != "" {
		t.Fatalf("phase lạ không được gán e2eStepId giả dạng catalog, got %q", r.StepID)
	}
	if r.Stage != "" {
		t.Fatalf("phase lạ không gán stage mặc định, got %q", r.Stage)
	}
	if r.LabelVi == "" {
		t.Fatal("vẫn cần label giải thích")
	}
}
