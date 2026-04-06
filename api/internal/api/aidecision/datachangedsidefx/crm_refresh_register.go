package datachangedsidefx

import (
	"meta_commerce/internal/api/aidecision/eventintake"
)

func init() {
	Register(90, "crm_intel_refresh_defer", func(ac *ApplyContext) error {
		if !ac.Route.CrmIntelRefreshDeferPipeline {
			return nil
		}
		if ac.RefreshWin <= 0 {
			return nil
		}
		eventintake.ScheduleDeferredSideEffect(eventintake.DeferredKindCrmRefresh, ac.OrgHex, ac.Src, ac.IDHex, ac.RefreshWin)
		return nil
	})
}
