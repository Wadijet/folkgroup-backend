// Package approval — Cơ chế duyệt. Bridge kết nối pkg/approval với app.
package approval

import (
	"sync"

	pkgapproval "meta_commerce/pkg/approval"

	"meta_commerce/internal/approval/bridge"
)

var (
	defaultEngine *pkgapproval.Engine
	initOnce      sync.Once
)

// Init khởi tạo engine với Storage (MongoDB) và Notifier (notifytrigger).
// Gọi trước khi sử dụng Propose/Approve/Reject/ListPending.
func Init() {
	initOnce.Do(func() {
		storage := bridge.NewMongoStorage()
		notifier := bridge.NewNotifytriggerNotifier()
		defaultEngine = pkgapproval.NewEngine(storage, notifier)
	})
}

// GetEngine trả về engine (sau khi Init). Dùng cho RegisterExecutor, RegisterEventTypes.
func GetEngine() *pkgapproval.Engine {
	return defaultEngine
}
