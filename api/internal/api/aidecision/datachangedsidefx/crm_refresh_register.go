package datachangedsidefx

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventintake"
)

func init() {
	Register(90, "customer_intel_refresh_defer", func(ac *ApplyContext) error {
		if !ac.Route.CustomerIntelRefreshDeferPipeline {
			return nil
		}
		if ac.RefreshWin <= 0 {
			return nil
		}
		var tid, cid string
		if ac.Evt != nil {
			tid = strings.TrimSpace(ac.Evt.TraceID)
			cid = strings.TrimSpace(ac.Evt.CorrelationID)
		}
		return eventintake.ScheduleDeferredSideEffect(ac.Ctx, eventintake.DeferredKindCrmRefresh, ac.OrgHex, ac.Src, ac.IDHex, ac.RefreshWin, tid, cid)
	})
}
