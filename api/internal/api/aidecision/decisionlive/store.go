package decisionlive

import (
	"sync"
	"sync/atomic"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// traceStore — ring buffer replay theo (org, traceId); process đơn (trước mắt).
type traceStore struct {
	mu    sync.RWMutex
	seq   sync.Map // key string -> *int64
	ring  sync.Map // key string -> *[]DecisionLiveEvent
}

func newTraceStore() *traceStore {
	return &traceStore{}
}

func storeKey(ownerOrgID primitive.ObjectID, traceID string) string {
	return ownerOrgID.Hex() + ":" + traceID
}

// append gắn seq, cắt ring nếu vượt MaxEventsPerTrace; trả bản đã ghi để broadcast.
func (s *traceStore) append(ownerOrgID primitive.ObjectID, traceID string, ev DecisionLiveEvent) DecisionLiveEvent {
	key := storeKey(ownerOrgID, traceID)
	p, _ := s.seq.LoadOrStore(key, new(int64))
	seq := atomic.AddInt64(p.(*int64), 1)
	ev.Seq = seq

	s.mu.Lock()
	defer s.mu.Unlock()
	var buf *[]DecisionLiveEvent
	if v, ok := s.ring.Load(key); ok {
		b := v.(*[]DecisionLiveEvent)
		buf = b
	} else {
		b := make([]DecisionLiveEvent, 0, 32)
		buf = &b
		s.ring.Store(key, buf)
	}
	*buf = append(*buf, ev)
	if len(*buf) > MaxEventsPerTrace {
		*buf = (*buf)[len(*buf)-MaxEventsPerTrace:]
	}
	return ev
}

// Snapshot trả bản sao slice (replay REST / WS đầu nối).
func (s *traceStore) snapshot(ownerOrgID primitive.ObjectID, traceID string) []DecisionLiveEvent {
	key := storeKey(ownerOrgID, traceID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.ring.Load(key)
	if !ok {
		return []DecisionLiveEvent{}
	}
	buf := *v.(*[]DecisionLiveEvent)
	out := make([]DecisionLiveEvent, len(buf))
	copy(out, buf)
	return out
}
