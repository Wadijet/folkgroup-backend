package decisionlive

import "testing"

func TestCountCompletionsSince(t *testing.T) {
	asOf := int64(300_000)
	ends := []int64{100, 200_000, 250_000, 299_000}
	if n := countCompletionsSince(ends, asOf, 60_000); n != 2 {
		t.Fatalf("1m window: muốn 2, có %d", n)
	}
	if n := countCompletionsSince(ends, asOf, 5*60_000); n != 4 {
		t.Fatalf("5m window: muốn 4 (cut=0), có %d", n)
	}
	if n := countCompletionsSince(nil, asOf, 60_000); n != 0 {
		t.Fatalf("nil slice: muốn 0, có %d", n)
	}
}
