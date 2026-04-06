// Package queuedebounce — trailing debounce in-process cho queue miền: gom theo khóa, mỗi lần Schedule lùi deadline.
//
// Không thay Mongo/outbox; dùng khi nhiều tín hiệu gần nhau chỉ cần một lần xử lý sau cửa sổ (giống eventintake defer / CRM intel sau ingest).
package queuedebounce

import (
	"sync"
	"time"
)

// Table — trailing: Schedule cùng key → due = now + window (ghi đè deadline).
type Table[K comparable] struct {
	mu    sync.Mutex
	slots map[K]time.Time
}

// NewTable tạo bảng debounce rỗng.
func NewTable[K comparable]() *Table[K] {
	return &Table[K]{slots: make(map[K]time.Time)}
}

// Schedule đăng ký hoặc lùi hạn. window <= 0 thì bỏ qua.
func (t *Table[K]) Schedule(key K, window time.Duration) {
	if window <= 0 || t == nil {
		return
	}
	due := time.Now().Add(window)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.slots == nil {
		t.slots = make(map[K]time.Time)
	}
	t.slots[key] = due
}

// TakeDue trả mọi key đã đến hoặc quá hạn và xóa khỏi bảng.
func (t *Table[K]) TakeDue(now time.Time) []K {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.slots) == 0 {
		return nil
	}
	var out []K
	for k, due := range t.slots {
		if !now.Before(due) {
			out = append(out, k)
			delete(t.slots, k)
		}
	}
	return out
}
