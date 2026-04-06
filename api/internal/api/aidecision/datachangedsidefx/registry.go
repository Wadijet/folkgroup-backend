package datachangedsidefx

import (
	"sort"
	"sync"
)

// Contributor — một bước side-effect; không fail toàn bộ pipeline (lỗi ghi log tại contributor).
type Contributor func(*ApplyContext) error

type regEntry struct {
	Order int
	Name  string
	Fn    Contributor
}

var (
	regMu   sync.RWMutex
	entries []regEntry
)

// Register đăng ký contributor; Order nhỏ chạy trước. Chỉ gọi từ init package miền.
func Register(order int, name string, fn Contributor) {
	if fn == nil || name == "" {
		return
	}
	regMu.Lock()
	entries = append(entries, regEntry{Order: order, Name: name, Fn: fn})
	regMu.Unlock()
}

// Run chạy tất cả contributor đã đăng ký theo thứ tự Order.
func Run(ac *ApplyContext) {
	if ac == nil {
		return
	}
	fillRouteIfUnset(ac)
	regMu.RLock()
	cp := make([]regEntry, len(entries))
	copy(cp, entries)
	regMu.RUnlock()

	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Order != cp[j].Order {
			return cp[i].Order < cp[j].Order
		}
		return cp[i].Name < cp[j].Name
	})

	for _, e := range cp {
		_ = e.Fn(ac)
	}
}
