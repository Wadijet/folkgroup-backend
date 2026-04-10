// Package datachanged — Đăng ký side-effect Order Intelligence sau datachanged pc_pos_orders.
package datachanged

import (
	"strings"

	"meta_commerce/internal/api/aidecision/datachangedsidefx"
	"meta_commerce/internal/api/aidecision/eventintake"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
)

func init() {
	datachangedsidefx.Register(50, "order_intel_compute", func(ac *datachangedsidefx.ApplyContext) error {
		if ac.Src != global.MongoDB_ColNames.PcPosOrders {
			return nil
		}
		if !ac.Route.OrderIntelPipeline {
			return nil
		}
		if ac.OrderIntelDefer > 0 {
			var tid, cid string
			if ac.Evt != nil {
				tid = strings.TrimSpace(ac.Evt.TraceID)
				cid = strings.TrimSpace(ac.Evt.CorrelationID)
			}
			return eventintake.ScheduleDeferredSideEffect(ac.Ctx, eventintake.DeferredKindOrderIntelCompute, ac.OrgHex, ac.Src, ac.IDHex, ac.OrderIntelDefer, tid, cid)
		}
		if err := EnqueueIntelligenceFromParentEvent(ac.Ctx, ac.Evt); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"eventId": ac.Evt.EventID, "orgHex": ac.OrgHex,
			}).Warn("📋 [ORDER_INTEL] Không xếp job order_intel_compute từ pc_pos_orders datachanged")
		}
		return nil
	})
}
