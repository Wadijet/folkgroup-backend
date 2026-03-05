// Package metahooks - Debouncer tránh recompute trùng khi nhiều event cùng entity trong thời gian ngắn.
package metahooks

import (
	"sync"
	"time"

	"meta_commerce/internal/adsintel"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type pendingAction struct {
	timer *time.Timer
	fn    func()
}

var (
	debouncerMu sync.Mutex
	debouncer   = make(map[string]*pendingAction)
)

// scheduleAdsRecompute đăng ký recompute sau DebounceMs. Nếu đã có pending cho cùng key thì reset timer.
func scheduleAdsRecompute(key string, fn func()) {
	debouncerMu.Lock()
	defer debouncerMu.Unlock()
	if p, ok := debouncer[key]; ok && p.timer != nil {
		p.timer.Stop()
	}
	t := time.AfterFunc(adsintel.DebounceMs*time.Millisecond, func() {
		debouncerMu.Lock()
		delete(debouncer, key)
		debouncerMu.Unlock()
		fn()
	})
	debouncer[key] = &pendingAction{timer: t, fn: fn}
}

// entityKey tạo key debounce: objectType|objectId|adAccountId|ownerOrgID|source
func entityKey(objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, source string) string {
	return objectType + "|" + objectId + "|" + adAccountId + "|" + ownerOrgID.Hex() + "|" + source
}
