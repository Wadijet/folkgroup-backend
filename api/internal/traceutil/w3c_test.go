package traceutil

import (
	"strings"
	"testing"
)

func TestNewTraceID_Format(t *testing.T) {
	id := NewTraceID()
	if len(id) != TraceIDHexLen {
		t.Fatalf("trace id length: %d", len(id))
	}
	if !IsValidTraceID(id) {
		t.Fatalf("invalid: %q", id)
	}
}

func TestNewSpanID_Format(t *testing.T) {
	id := NewSpanID()
	if len(id) != SpanIDHexLen {
		t.Fatalf("span id length: %d", len(id))
	}
	if !IsValidSpanID(id) {
		t.Fatalf("invalid: %q", id)
	}
}

func TestW3CTraceIDFromKey_Deterministic(t *testing.T) {
	a := W3CTraceIDFromKey("trace_abc123xyz")
	b := W3CTraceIDFromKey("trace_abc123xyz")
	if a != b {
		t.Fatalf("not stable: %s vs %s", a, b)
	}
	if !IsValidTraceID(a) {
		t.Fatalf("not w3c: %q", a)
	}
}

func TestW3CTraceIDFromKey_PassthroughHex(t *testing.T) {
	h := strings.Repeat("a", 32)
	if got := W3CTraceIDFromKey(h); got != h {
		t.Fatalf("want passthrough, got %q", got)
	}
}

func TestTraceParentValue(t *testing.T) {
	v := TraceParentValue(
		strings.Repeat("b", 32),
		strings.Repeat("c", 16),
		true,
	)
	if len(strings.Split(v, "-")) != 4 {
		t.Fatalf("bad traceparent: %q", v)
	}
}
