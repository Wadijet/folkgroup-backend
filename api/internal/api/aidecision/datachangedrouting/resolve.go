package datachangedrouting

import (
	"strings"

	"meta_commerce/internal/api/aidecision/datachangedemit"
	"meta_commerce/internal/api/aidecision/routecontract"
	"meta_commerce/internal/global"
)

// Resolve tính Decision: mirror code + ghi đè collection_overrides từ YAML (embed hoặc DATACHANGED_ROUTING_CONFIG).
func Resolve(collection string) routecontract.Decision {
	d := resolveBase(collection)
	return applyCollectionOverrides(d)
}

func resolveBase(collection string) routecontract.Decision {
	c := strings.TrimSpace(collection)
	return routecontract.Decision{
		Version:                      Version,
		Collection:                   c,
		RuleID:                       ruleIDForCollection(c),
		EmitToDecisionQueue:          datachangedemit.DefaultShouldEmitToDecisionQueue(c),
		CustomerPendingMergeCollection:    isCustomerPendingMergeCollection(c),
		ReportTouchPipeline:               true,
		AdsProfilePipeline:                true,
		CixIntelPipeline:                  c == global.MongoDB_ColNames.FbMessageItems,
		OrderIntelPipeline:                c == global.MongoDB_ColNames.PcPosOrders,
		CustomerIntelRefreshDeferPipeline: true,
	}
}

func isCustomerPendingMergeCollection(name string) bool {
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

func ruleIDForCollection(c string) string {
	if c == "" {
		return "empty_collection"
	}
	if isCustomerPendingMergeCollection(c) {
		return "customer_l1_merge_sources"
	}
	if c == global.MongoDB_ColNames.FbMessageItems {
		return "cix_fb_message_items"
	}
	if datachangedemit.IsMetaAdsSyncedCollection(c) {
		return "meta_ads_synced_family"
	}
	return "generic_source"
}
