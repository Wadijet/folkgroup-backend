package queuedebounce

import (
	"sync"
	"time"
)

// MergeMeta gộp metadata khi Schedule lặp lại cùng key (vd. giữ trace mới nhất nếu khác rỗng).
type MergeMeta[M any] func(prev, next M) M

// DueEntry — một mục đến hạn từ MetaTable.TakeDue.
type DueEntry[K comparable, M any] struct {
	Key  K
	Meta M
}

// MetaTable — trailing debounce kèm metadata (CRM intel: org+unifiedId + trace).
type MetaTable[K comparable, M any] struct {
	mu    sync.Mutex
	slots map[K]*metaSlot[M]
	merge MergeMeta[M]
}

type metaSlot[M any] struct {
	due  time.Time
	meta M
}

// NewMetaTable tạo bảng. merge nil → mỗi Schedule ghi đè meta bằng bản mới.
func NewMetaTable[K comparable, M any](merge MergeMeta[M]) *MetaTable[K, M] {
	return &MetaTable[K, M]{
		slots: make(map[K]*metaSlot[M]),
		merge: merge,
	}
}

// Schedule lùi due và cập nhật meta. window <= 0 thì bỏ qua.
func (t *MetaTable[K, M]) Schedule(key K, window time.Duration, meta M) {
	if window <= 0 || t == nil {
		return
	}
	due := time.Now().Add(window)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.slots == nil {
		t.slots = make(map[K]*metaSlot[M])
	}
	s := t.slots[key]
	if s == nil {
		t.slots[key] = &metaSlot[M]{due: due, meta: meta}
		return
	}
	s.due = due
	if t.merge != nil {
		s.meta = t.merge(s.meta, meta)
	} else {
		s.meta = meta
	}
}

// TakeDue trả mọi cặp (key, meta) đã đến hạn và xóa khỏi bảng.
func (t *MetaTable[K, M]) TakeDue(now time.Time) []DueEntry[K, M] {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.slots) == 0 {
		return nil
	}
	var out []DueEntry[K, M]
	for k, s := range t.slots {
		if s == nil || now.Before(s.due) {
			continue
		}
		out = append(out, DueEntry[K, M]{Key: k, Meta: s.meta})
		delete(t.slots, k)
	}
	return out
}
