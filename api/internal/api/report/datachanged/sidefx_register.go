// Package datachanged — Đăng ký side-effect báo cáo (Redis touch) sau datachanged.
package datachanged

import (
	"strings"

	"meta_commerce/internal/api/aidecision/datachangedsidefx"
	"meta_commerce/internal/api/aidecision/eventintake"
)

func init() {
	datachangedsidefx.Register(20, "report_redis_touch", func(ac *datachangedsidefx.ApplyContext) error {
		if !ac.Route.ReportTouchPipeline {
			return nil
		}
		if !ac.Dec.AllowReport {
			return nil
		}
		if ac.ReportWin > 0 {
			var tid, cid string
			if ac.Evt != nil {
				tid = strings.TrimSpace(ac.Evt.TraceID)
				cid = strings.TrimSpace(ac.Evt.CorrelationID)
			}
			return eventintake.ScheduleDeferredSideEffect(ac.Ctx, eventintake.DeferredKindReport, ac.OrgHex, ac.Src, ac.IDHex, ac.ReportWin, tid, cid)
		}
		RecordTouchFromDataChange(ac.Ctx, ac.E)
		return nil
	})
}
