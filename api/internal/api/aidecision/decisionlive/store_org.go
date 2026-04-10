package decisionlive

import (
	"sync"
	"sync/atomic"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MaxEventsPerOrgFeed — Giới hạn ring org-live trong RAM (Publish bước 6b); GET org có thể đọc Mongo qua OrgTimelineForAPI khi persist bật.
const MaxEventsPerOrgFeed = 512

// orgFeedStore — Ring gộp mọi trace trong một org (nguồn cho OrgTimeline RAM và fallback OrgTimelineForAPI).
type orgFeedStore struct {
	mu   sync.RWMutex
	seq  sync.Map // org hex -> *int64
	ring sync.Map // org hex -> *[]DecisionLiveEvent
}

func newOrgFeedStore() *orgFeedStore {
	return &orgFeedStore{}
}

func orgFeedStoreKey(ownerOrgID primitive.ObjectID) string {
	return ownerOrgID.Hex()
}

// appendOrg gắn FeedSeq, đẩy vào ring theo org; trả bản đã ghi (Publish bước 6b — trước broadcast org-live).
func (s *orgFeedStore) appendOrg(ownerOrgID primitive.ObjectID, ev DecisionLiveEvent) DecisionLiveEvent {
	if ownerOrgID.IsZero() {
		return ev
	}
	key := orgFeedStoreKey(ownerOrgID)
	p, _ := s.seq.LoadOrStore(key, new(int64))
	feedSeq := atomic.AddInt64(p.(*int64), 1)
	ev.FeedSeq = feedSeq

	s.mu.Lock()
	defer s.mu.Unlock()
	var buf *[]DecisionLiveEvent
	if v, ok := s.ring.Load(key); ok {
		b := v.(*[]DecisionLiveEvent)
		buf = b
	} else {
		b := make([]DecisionLiveEvent, 0, 64)
		buf = &b
		s.ring.Store(key, buf)
	}
	*buf = append(*buf, ev)
	if len(*buf) > MaxEventsPerOrgFeed {
		*buf = (*buf)[len(*buf)-MaxEventsPerOrgFeed:]
	}
	return ev
}

// snapshotOrg — Bản sao buffer org (OrgTimeline bước 1; OrgTimelineForAPI dùng khi không đọc Mongo).
func (s *orgFeedStore) snapshotOrg(ownerOrgID primitive.ObjectID) []DecisionLiveEvent {
	if ownerOrgID.IsZero() {
		return []DecisionLiveEvent{}
	}
	key := orgFeedStoreKey(ownerOrgID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.ring.Load(key)
	if !ok {
		// JSON: slice nil → "null"; client cần mảng rỗng [].
		return []DecisionLiveEvent{}
	}
	buf := *v.(*[]DecisionLiveEvent)
	out := make([]DecisionLiveEvent, len(buf))
	copy(out, buf)
	return out
}
