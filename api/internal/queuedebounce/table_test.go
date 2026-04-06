package queuedebounce

import (
	"testing"
	"time"
)

func TestTable_TrailingReschedulesDue(t *testing.T) {
	type key struct{ A, B string }
	tab := NewTable[key]()
	k := key{"o1", "c1"}
	tab.Schedule(k, 50*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	tab.Schedule(k, 50*time.Millisecond)
	if due := tab.TakeDue(time.Now()); len(due) != 0 {
		t.Fatalf("chưa đến hạn phải rỗng, got %v", due)
	}
	time.Sleep(55 * time.Millisecond)
	got := tab.TakeDue(time.Now())
	if len(got) != 1 || got[0] != k {
		t.Fatalf("sau window mong đợi 1 key, got %v", got)
	}
	if len(tab.TakeDue(time.Now())) != 0 {
		t.Fatal("TakeDue lần hai phải rỗng")
	}
}

func TestMetaTable_MergeAndTakeDue(t *testing.T) {
	type k struct{ X string }
	type m struct{ V int }
	merge := func(prev, next m) m {
		return m{V: prev.V + next.V}
	}
	tab := NewMetaTable[k, m](merge)
	tab.Schedule(k{"a"}, time.Hour, m{1})
	tab.Schedule(k{"a"}, time.Hour, m{2})
	ent := tab.TakeDue(time.Now().Add(time.Hour))
	if len(ent) != 1 || ent[0].Key.X != "a" || ent[0].Meta.V != 3 {
		t.Fatalf("merge sai: %+v", ent)
	}
}
