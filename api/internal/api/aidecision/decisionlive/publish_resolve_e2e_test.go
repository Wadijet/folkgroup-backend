package decisionlive

import (
	"testing"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

func TestResolveE2ERefForPublish_PhaseOrchestrateUuTienG4S01(t *testing.T) {
	ev := &DecisionLiveEvent{
		Phase: PhaseOrchestrate,
		Refs: map[string]string{
			"eventType":     eventtypes.ConversationChanged,
			"eventSource":   eventtypes.EventSourceL1Datachanged,
			"pipelineStage": eventtypes.PipelineStageAfterL1Change,
		},
	}
	ref := resolveE2ERefForPublish(ev)
	if ref.Stage != eventtypes.E2EStageG4 || ref.StepID != "G4-S01" {
		t.Fatalf("phase orchestrate phải neo G4-S01 (§5.3 / live phase), got %+v", ref)
	}
}

func TestResolveE2ERefForPublish_PhaseConsumingUuTienG4S03(t *testing.T) {
	ev := &DecisionLiveEvent{
		Phase: PhaseConsuming,
		Refs: map[string]string{
			"eventType":   eventtypes.AIDecisionExecuteRequested,
			"eventSource": "aidecision",
		},
	}
	ref := resolveE2ERefForPublish(ev)
	if ref.Stage != eventtypes.E2EStageG4 || ref.StepID != "G4-S03" {
		t.Fatalf("phase consuming phải neo G4-S03 (engine), không G4-S03-E01 từ envelope execute, got %+v", ref)
	}
}

func TestResolveE2ERefForPublish_ThieuPhaseVanDungEnvelope(t *testing.T) {
	ev := &DecisionLiveEvent{
		Refs: map[string]string{
			"eventType":     "cix.analysis_requested",
			"eventSource":   "aidecision",
			"pipelineStage": "aid_coordination",
		},
	}
	ref := resolveE2ERefForPublish(ev)
	if ref.Stage != eventtypes.E2EStageG3 || ref.StepID != "G3-S01" {
		t.Fatalf("không có phase: vẫn map envelope queue, got %+v", ref)
	}
}
