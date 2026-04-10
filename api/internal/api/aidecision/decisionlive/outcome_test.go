package decisionlive

import "testing"

func TestEnrichLiveOutcomeMetadata_QueueError(t *testing.T) {
	ev := &DecisionLiveEvent{Phase: PhaseQueueError, Severity: SeverityError}
	EnrichLiveOutcomeMetadata(ev)
	if ev.OutcomeKind != OutcomeProcessingError {
		t.Fatalf("outcomeKind=%q", ev.OutcomeKind)
	}
	if !ev.OutcomeAbnormal {
		t.Fatal("mong đợi bất thường")
	}
	if ev.OutcomeLabelVi == "" {
		t.Fatal("thiếu label")
	}
}

func TestEnrichLiveOutcomeMetadata_PreferredKind(t *testing.T) {
	ev := &DecisionLiveEvent{OutcomeKind: OutcomePolicySkipped}
	EnrichLiveOutcomeMetadata(ev)
	if ev.OutcomeKind != OutcomePolicySkipped {
		t.Fatal("không ghi đè kind đã gán")
	}
	if !ev.OutcomeAbnormal {
		t.Fatal("policy skip là bất thường")
	}
}

func TestEnrichLiveOutcomeMetadata_SuccessNotAbnormal(t *testing.T) {
	ev := &DecisionLiveEvent{OutcomeKind: OutcomeSuccess}
	EnrichLiveOutcomeMetadata(ev)
	if ev.OutcomeAbnormal {
		t.Fatal("success không abnormal")
	}
}
