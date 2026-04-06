// Package datachanged — Đăng ký side-effect báo cáo (Redis touch) sau datachanged.
package datachanged

import (
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
			eventintake.ScheduleDeferredSideEffect(eventintake.DeferredKindReport, ac.OrgHex, ac.Src, ac.IDHex, ac.ReportWin)
			return nil
		}
		RecordTouchFromDataChange(ac.Ctx, ac.E)
		return nil
	})
}
