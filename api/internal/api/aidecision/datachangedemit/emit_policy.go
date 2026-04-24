// Package datachangedemit — quy tắc mặc định ghi decision_events_queue sau datachanged (không đọc YAML).
// YAML emit_to_decision_queue xử lý ở datachangedrouting + hooks.ShouldEmitDatachangedToDecisionQueue.
package datachangedemit

import (
	"strings"

	"meta_commerce/internal/global"
)

// EmitPerCollection — tên collection Mongo → có ghi decision_events_queue sau datachanged hay không (chỉ key có trong map).
// Collection không khai báo: non-Meta → bật; nhóm Meta Marketing → chỉ meta_ad_insights.
var EmitPerCollection = map[string]bool{
	"fb_src_posts":                  false,
	"fb_src_pages":                  false,
	"pc_pos_src_shops":              false,
	"pc_pos_src_warehouses":         false,
	"order_src_pcpos_products":      false,
	"order_src_pcpos_variations":    false,
	"order_src_pcpos_categories":    false,
	"order_src_manual_products":     false,
	"order_src_manual_variations":   false,
	"order_src_manual_categories":   false,
	"order_src_manual_shops":        false,
	"order_src_manual_warehouses":   false,
	"customer_core_records":         false,
	"customer_run_activity_history": false,
	"customer_core_notes":           false,
	"cix_run_analysis_results":      false,
	"webhook_run_logs":              false,
}

// DefaultShouldEmitToDecisionQueue — mặc định code (map + nhóm Meta); không áp YAML.
func DefaultShouldEmitToDecisionQueue(collectionName string) bool {
	c := strings.TrimSpace(collectionName)
	if c == "" {
		return false
	}
	if EmitPerCollection != nil {
		if v, ok := EmitPerCollection[c]; ok {
			return v
		}
	}
	if isMetaAdsSyncedCollection(c) {
		return c == global.MongoDB_ColNames.MetaAdInsights
	}
	return true
}

// IsMetaAdsSyncedCollection — true nếu collection thuộc nhóm Meta Marketing đồng bộ.
func IsMetaAdsSyncedCollection(name string) bool {
	return isMetaAdsSyncedCollection(strings.TrimSpace(name))
}

func isMetaAdsSyncedCollection(name string) bool {
	c := global.MongoDB_ColNames
	switch name {
	case c.MetaAdAccounts, c.MetaCampaigns, c.MetaAdSets, c.MetaAds, c.MetaAdInsights, c.MetaAdInsightsDailySnapshots:
		return true
	default:
		return false
	}
}
