package datachangedsidefx

import (
	"strings"

	"meta_commerce/internal/api/aidecision/routecontract"
	"meta_commerce/internal/global"
)

// fillRouteIfUnset — nếu worker quên gán Route (zero → mọi pipeline false), bù pipeline từ Src.
// Không import datachangedrouting (tránh vòng import qua hooks → service).
func fillRouteIfUnset(ac *ApplyContext) {
	if ac == nil {
		return
	}
	if strings.TrimSpace(ac.Route.Collection) != "" {
		return
	}
	src := strings.TrimSpace(ac.Src)
	if src == "" {
		return
	}
	ac.Route = defaultRouteForSrc(src)
}

func defaultRouteForSrc(c string) routecontract.Decision {
	return routecontract.Decision{
		Version:                      "sidefx-fallback",
		Collection:                   c,
		RuleID:                       "sidefx_fallback_unset_route",
		EmitToDecisionQueue:          true,
		CustomerPendingMergeCollection:    isCustomerPendingMergeCollectionLocal(c),
		ReportTouchPipeline:               true,
		AdsProfilePipeline:                true,
		CixIntelPipeline:                  c == global.MongoDB_ColNames.FbMessageItems,
		OrderIntelPipeline:                c == global.MongoDB_ColNames.PcPosOrders,
		CustomerIntelRefreshDeferPipeline: true,
	}
}

func isCustomerPendingMergeCollectionLocal(name string) bool {
	switch name {
	case global.MongoDB_ColNames.PcPosCustomers,
		global.MongoDB_ColNames.FbCustomers,
		global.MongoDB_ColNames.PcPosOrders,
		global.MongoDB_ColNames.FbConvesations,
		global.MongoDB_ColNames.CustomerNotes:
		return true
	default:
		return false
	}
}
