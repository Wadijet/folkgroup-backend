package decisionlive

import "testing"

func TestCapDecisionLiveProcessTrace_NoOpWhenSmall(t *testing.T) {
	ev := &DecisionLiveEvent{
		ProcessTrace: []DecisionLiveProcessNode{
			{Kind: ProcessTraceKindStep, LabelVi: "a"},
		},
	}
	CapDecisionLiveProcessTrace(ev)
	if len(ev.ProcessTrace) != 1 {
		t.Fatalf("mong đợi giữ nguyên 1 nút: %d", len(ev.ProcessTrace))
	}
}

func TestCapDecisionLiveProcessTrace_TruncatesLargeTree(t *testing.T) {
	nodes := make([]DecisionLiveProcessNode, maxProcessTraceNodes+10)
	for i := range nodes {
		nodes[i] = DecisionLiveProcessNode{Kind: ProcessTraceKindStep, LabelVi: "x"}
	}
	ev := &DecisionLiveEvent{ProcessTrace: nodes}
	CapDecisionLiveProcessTrace(ev)
	if len(ev.ProcessTrace) > maxProcessTraceNodes {
		t.Fatalf("vẫn quá dài: %d", len(ev.ProcessTrace))
	}
}
