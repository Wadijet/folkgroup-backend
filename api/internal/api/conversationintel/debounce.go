// Package conversationintel — Trailing debounce theo (org, conversationId).
package conversationintel

import (
	"sync"
	"time"
)

type pendingAction struct {
	timer *time.Timer
	fn    func()
}

var (
	debouncerMu sync.Mutex
	debouncer   = make(map[string]*pendingAction)
)

// scheduleConversationIntel đăng ký emit sau debounceMs; debounceMs=0 → chạy ngay (time.AfterFunc 0).
// Cùng key → reset timer (trailing debounce), tương tự metahooks.scheduleAdsRecompute.
func scheduleConversationIntel(key string, debounceMs int, fn func()) {
	if debounceMs < 0 {
		debounceMs = 0
	}
	debouncerMu.Lock()
	defer debouncerMu.Unlock()
	if p, ok := debouncer[key]; ok && p.timer != nil {
		p.timer.Stop()
	}
	d := time.Duration(debounceMs) * time.Millisecond
	t := time.AfterFunc(d, func() {
		debouncerMu.Lock()
		delete(debouncer, key)
		debouncerMu.Unlock()
		fn()
	})
	debouncer[key] = &pendingAction{timer: t, fn: fn}
}

func debounceKey(ownerOrgHex, conversationID string) string {
	return "convintel|" + ownerOrgHex + "|" + conversationID
}
