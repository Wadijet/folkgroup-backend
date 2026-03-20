// Package approval — Cơ chế duyệt. Bridge kết nối pkg/approval với app.
package approval

import (
	"context"
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
		// ResolveImmediate (Vision 08): đọc ApprovalModeConfig → auto Approve nếu mode=auto
		pkgapproval.SetResolver(pkgapproval.ResolverFunc(resolveImmediate))
	})
}

// resolveImmediate quyết định có nên auto-approve ngay sau Propose không.
func resolveImmediate(ctx context.Context, doc *pkgapproval.ActionPending) bool {
	scopeKey := ""
	ruleCode := ""
	if doc.Payload != nil {
		if s, ok := doc.Payload["adAccountId"].(string); ok {
			scopeKey = s
		}
		if s, ok := doc.Payload["ruleCode"].(string); ok {
			ruleCode = s
		}
	}
	mode, err := GetApprovalMode(ctx, doc.OwnerOrganizationID, doc.Domain, scopeKey, doc.ActionType, ruleCode)
	if err != nil {
		return false
	}
	return mode == pkgapproval.ApprovalModeAutoByRule || mode == pkgapproval.ApprovalModeFullyAuto
}

// GetEngine trả về engine (sau khi Init). Dùng cho RegisterExecutor, RegisterEventTypes.
func GetEngine() *pkgapproval.Engine {
	return defaultEngine
}
