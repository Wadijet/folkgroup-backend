// Package datachanged — Đăng ký side-effect merge queue CRM vào pipeline datachanged (chuẩn hoá dần).
//
// Nếu IDE báo "missing metadata" / không resolve import datachangedsidefx: chạy `go build ./...` trong thư mục api và reload cửa sổ / gopls — biên dịch Go vẫn hợp lệ.
package datachanged

import (
	"strings"

	"meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/datachangedsidefx"
	"meta_commerce/internal/api/aidecision/eventintake"
)

func init() {
	datachangedsidefx.Register(10, "crm_merge_queue", func(ac *datachangedsidefx.ApplyContext) error {
		if !ac.Route.CustomerPendingMergeCollection {
			return nil
		}
		if !ac.Dec.AllowCrmMergeQueue {
			return nil
		}
		if ac.IngestWin > 0 {
			var tid, cid string
			if ac.Evt != nil {
				tid = strings.TrimSpace(ac.Evt.TraceID)
				cid = strings.TrimSpace(ac.Evt.CorrelationID)
			}
			return eventintake.ScheduleDeferredSideEffect(ac.Ctx, eventintake.DeferredKindCrmMergeQueue, ac.OrgHex, ac.Src, ac.IDHex, ac.IngestWin, tid, cid)
		}
		var tid, cid string
		if ac.Evt != nil {
			tid = strings.TrimSpace(ac.Evt.TraceID)
			cid = strings.TrimSpace(ac.Evt.CorrelationID)
		}
		mergeBus := crmqueue.CompleteDomainJobBus(crmqueue.DomainQueueBusFieldsPtrFromDecisionEvent(ac.Evt), crmqueue.ProcessorDomainCRM, crmqueue.EnqueueSourceCRMDataChanged)
		EnqueueCrmMergeFromDataChange(ac.Ctx, ac.E, tid, cid, mergeBus)
		return nil
	})
}
