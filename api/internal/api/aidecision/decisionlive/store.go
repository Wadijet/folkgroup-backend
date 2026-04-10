package decisionlive

import (
	"strings"
	"sync"
	"sync/atomic"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// traceStore — Ring buffer timeline theo (org, traceId): nguồn sự thật cho GET Timeline / replay WS trace (đối chiếu Publish bước 4).
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

// append gắn seq, nối parentSpanId → spanId mốc trước (cùng org+trace), sinh w3cTraceId/spanId mới, cắt ring nếu vượt MaxEventsPerTrace; trả bản đã ghi để broadcast (Publish bước 4 — live bật).
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
		// Chuỗi span W3C trên timeline: mốc sau có parent = spanId mốc liền trước (caller đã set parentSpanId thì giữ — phân nhánh tùy chỉnh).
		if len(*b) > 0 && strings.TrimSpace(ev.ParentSpanID) == "" {
			last := (*b)[len(*b)-1]
			if ps := strings.TrimSpace(last.SpanID); ps != "" {
				ev.ParentSpanID = ps
				if ev.Refs == nil {
					ev.Refs = make(map[string]string)
				}
				ev.Refs["parentSpanId"] = ps
			}
		}
	} else {
		b := make([]DecisionLiveEvent, 0, 32)
		buf = &b
		s.ring.Store(key, buf)
	}

	enrichW3CTraceContext(&ev, traceID)

	*buf = append(*buf, ev)
	if len(*buf) > MaxEventsPerTrace {
		*buf = (*buf)[len(*buf)-MaxEventsPerTrace:]
	}
	return ev
}

// snapshot trả bản sao slice theo thứ tự append (Timeline bước 1 — đọc ring sau khi Publish đã ghi).
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
