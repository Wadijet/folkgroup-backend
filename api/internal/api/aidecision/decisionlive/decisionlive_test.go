package decisionlive

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTraceStoreAppendAndSnapshot(t *testing.T) {
	s := newTraceStore()
	org := primitive.NewObjectID()
	trace := "trace_test_1"
	ev := DecisionLiveEvent{Phase: PhaseQueued, Summary: "x"}
	out := s.append(org, trace, ev)
	if out.Seq != 1 {
		t.Fatalf("seq mong đợi 1, got %d", out.Seq)
	}
	if out.ParentSpanID != "" {
		t.Fatalf("mốc đầu không có parentSpanId, got %q", out.ParentSpanID)
	}
	if out.SpanID == "" || out.W3CTraceID == "" {
		t.Fatalf("thiếu spanId/w3cTraceId sau append: span=%q w3c=%q", out.SpanID, out.W3CTraceID)
	}
	snap := s.snapshot(org, trace)
	if len(snap) != 1 || snap[0].Seq != 1 {
		t.Fatalf("snapshot sai: %+v", snap)
	}

	out2 := s.append(org, trace, DecisionLiveEvent{Phase: PhaseDone, Summary: "y"})
	if out2.ParentSpanID != out.SpanID {
		t.Fatalf("mốc 2 parentSpanId mong %q, got %q", out.SpanID, out2.ParentSpanID)
	}
	if out2.SpanID == "" || out2.SpanID == out.SpanID {
		t.Fatalf("mốc 2 phải có spanId mới khác mốc 1: %q / %q", out.SpanID, out2.SpanID)
	}
}

func TestOrgFeedStoreAppend(t *testing.T) {
	s := newOrgFeedStore()
	org := primitive.NewObjectID()
	ev := DecisionLiveEvent{Phase: PhaseQueued, Summary: "org"}
	out := s.appendOrg(org, ev)
	if out.FeedSeq != 1 {
		t.Fatalf("feedSeq mong đợi 1, got %d", out.FeedSeq)
	}
	snap := s.snapshotOrg(org)
	if len(snap) != 1 || snap[0].FeedSeq != 1 {
		t.Fatalf("org snapshot sai: %+v", snap)
	}
}

func TestInferSourceForFeed_Order(t *testing.T) {
	k, title := InferSourceForFeed(map[string]interface{}{
		"source": "order_intelligence", "orderUid": "ORD-1",
	}, "sess", "cust")
	if k != SourceOrder || title == "" {
		t.Fatalf("order: k=%s title=%q", k, title)
	}
}

func TestOrgTimelineEmptyIsJSONArrayNotNull(t *testing.T) {
	snap := OrgTimeline(primitive.NewObjectID())
	if snap == nil {
		t.Fatal("mong đợi slice rỗng không-nil (JSON [] chứ không phải null)")
	}
	if len(snap) != 0 {
		t.Fatalf("mong len 0, got %d", len(snap))
	}
}

func TestInferSourceForFeed_Conversation(t *testing.T) {
	k, title := InferSourceForFeed(map[string]interface{}{"actionSuggestions": []string{"a"}}, "conv-99", "")
	if k != SourceConversation {
		t.Fatalf("mong conversation, got %s", k)
	}
	if title == "" {
		t.Fatal("title rỗng")
	}
}
