// Package datachanged — Đăng ký side-effect merge queue CRM vào pipeline datachanged (chuẩn hoá dần).
//
// Nếu IDE báo "missing metadata" / không resolve import datachangedsidefx: chạy `go build ./...` trong thư mục api và reload cửa sổ / gopls — biên dịch Go vẫn hợp lệ.
package datachanged

import (
	"meta_commerce/internal/api/aidecision/datachangedsidefx"
	"meta_commerce/internal/api/aidecision/eventintake"
)

func init() {
	datachangedsidefx.Register(10, "crm_merge_queue", func(ac *datachangedsidefx.ApplyContext) error {
		if !ac.Route.CrmPendingMergeCollection {
			return nil
		}
		if !ac.Dec.AllowCrmMergeQueue {
			return nil
		}
		if ac.IngestWin > 0 {
			eventintake.ScheduleDeferredSideEffect(eventintake.DeferredKindCrmMergeQueue, ac.OrgHex, ac.Src, ac.IDHex, ac.IngestWin)
			return nil
		}
		EnqueueCrmMergeFromDataChange(ac.Ctx, ac.E)
		return nil
	})
}
