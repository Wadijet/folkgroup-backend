package decisionlive

import (
	"testing"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

func TestClassifyEventTypeFeedSource(t *testing.T) {
	tests := []struct {
		et   string
		want string
	}{
		{"", FeedSourceQueue},
		{"meta_ad_insight.updated", FeedSourceMetaSync},
		{"pos_product.updated", FeedSourcePosSync},
		{"crm_customer.inserted", FeedSourceCrm},
		{"conversation.changed", FeedSourceConversation},
		{"conversation.inserted", FeedSourceConversation},
		{"order.changed", FeedSourceOrder},
		{"order.updated", FeedSourceOrder},
		{eventtypes.CixAnalysisRequested, FeedSourceIntel},
		{eventtypes.AdsContextRequested, FeedSourceAds},
		{eventtypes.AdsIntelligenceRecomputeRequested, FeedSourceIntel},
		{eventtypes.CrmIntelligenceRecomputeRequested, FeedSourceCrm},
		{eventtypes.CampaignIntelRecomputed, FeedSourceIntel},
		{eventtypes.AIDecisionExecuteRequested, FeedSourceDecision},
		{"webhook_log.inserted", FeedSourceWebhook},
	}
	for _, tc := range tests {
		if g := classifyEventTypeFeedSource(tc.et); g != tc.want {
			t.Errorf("classifyEventTypeFeedSource(%q) = %q, want %q", tc.et, g, tc.want)
		}
	}
}

func TestEnrichLiveEventFeedSource_queue(t *testing.T) {
	ev := DecisionLiveEvent{
		SourceKind: SourceQueue,
		Refs:       map[string]string{"eventType": eventtypes.CampaignIntelRecomputed},
	}
	enrichLiveEventFeedSource(&ev)
	if ev.FeedSourceCategory != FeedSourceIntel {
		t.Fatalf("category %q", ev.FeedSourceCategory)
	}
	if ev.SourceKind != FeedSourceIntel {
		t.Fatalf("mong sourceKind intel sau recompute campaign, got %q", ev.SourceKind)
	}
	if ev.FeedSourceLabelVi == "" {
		t.Fatal("empty label")
	}
}

func TestEnrichLiveEventFeedSource_posVariationUsesSourceTitle(t *testing.T) {
	ev := DecisionLiveEvent{
		SourceKind:  SourceQueue,
		SourceTitle: "pos_variation.updated",
	}
	enrichLiveEventFeedSource(&ev)
	if ev.FeedSourceCategory != FeedSourcePosSync {
		t.Fatalf("category %q", ev.FeedSourceCategory)
	}
	if ev.SourceKind != FeedSourcePosSync {
		t.Fatalf("sourceKind %q", ev.SourceKind)
	}
}

// Pipeline Execute thường không gắn refs.eventType — phải suy từ phase (trước đây rơi toàn other).
func TestEnrichLiveEventFeedSource_executePipelineNoRefs(t *testing.T) {
	ev := DecisionLiveEvent{
		SourceKind: SourceUnknown,
		Phase:      PhaseParse,
		Summary:    "test",
	}
	enrichLiveEventFeedSource(&ev)
	if ev.FeedSourceCategory != FeedSourceDecision {
		t.Fatalf("mong decision, got %q", ev.FeedSourceCategory)
	}
	if ev.SourceKind != FeedSourceDecision {
		t.Fatalf("mong sourceKind decision, got %q", ev.SourceKind)
	}
}

func TestEnrichLiveEventFeedSource_emptySourceKindUsesPhase(t *testing.T) {
	ev := DecisionLiveEvent{
		Phase: PhaseConsuming,
	}
	enrichLiveEventFeedSource(&ev)
	if ev.FeedSourceCategory != FeedSourceDecision {
		t.Fatalf("mong decision, got %q", ev.FeedSourceCategory)
	}
	if ev.SourceKind != FeedSourceDecision {
		t.Fatalf("mong sourceKind decision, got %q", ev.SourceKind)
	}
}

func TestEnrichLiveEventFeedSource_queuePhaseNoRefs(t *testing.T) {
	ev := DecisionLiveEvent{
		Phase: PhaseQueueProcessing,
	}
	enrichLiveEventFeedSource(&ev)
	if ev.FeedSourceCategory != FeedSourceQueue {
		t.Fatalf("mong queue, got %q", ev.FeedSourceCategory)
	}
}
