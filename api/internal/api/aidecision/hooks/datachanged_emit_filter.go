// Package hooks — Bộ lọc: collection đăng ký source_sync_registry; chỉ một số collection ghi decision_events_queue.
//
// Cấu hình duy nhất: map DatachangedEmitPerCollection (datachanged_emit_per_collection.go) — key có trong map thì dùng giá trị bool.
// Collection không có trong map: non-Meta → emit; nhóm Meta Marketing → chỉ meta_ad_insights (cố định trong code).
//
// campaign_intel_recomputed → AI Decision từ worker sau recompute (meta_ads_intel).
package hooks

import (
	"strings"

	"meta_commerce/internal/global"
)

// ShouldEmitDatachangedToDecisionQueue quyết định sau OnDataChanged có gọi EmitEvent → decision_events_queue hay không.
func ShouldEmitDatachangedToDecisionQueue(collectionName string) bool {
	collectionName = strings.TrimSpace(collectionName)
	if collectionName == "" {
		return false
	}
	if DatachangedEmitPerCollection != nil {
		if v, ok := DatachangedEmitPerCollection[collectionName]; ok {
			return v
		}
	}
	if isMetaAdsSyncedCollection(collectionName) {
		c := global.MongoDB_ColNames
		return collectionName == c.MetaAdInsights
	}
	return true
}

// isMetaAdsSyncedCollection các collection Meta Marketing trong registry (đồng bộ API / snapshot).
func isMetaAdsSyncedCollection(name string) bool {
	c := global.MongoDB_ColNames
	switch name {
	case c.MetaAdAccounts, c.MetaCampaigns, c.MetaAdSets, c.MetaAds, c.MetaAdInsights, c.MetaAdInsightsDailySnapshots:
		return true
	default:
		return false
	}
}
