// Package eventtypes — wire <prefix>.changed theo collection mirror (đồng bộ bảng hooks/source_sync_registry.go; đổi map phải cập nhật cả hai).
package eventtypes

import (
	"strings"
	"sync"

	"meta_commerce/internal/global"
)

var (
	sourceMirrorPrefixOnce sync.Once
	sourceMirrorCollPrefix map[string]string
)

func sourceMirrorCollectionPrefixes() map[string]string {
	sourceMirrorPrefixOnce.Do(func() {
		c := global.MongoDB_ColNames
		sourceMirrorCollPrefix = map[string]string{
			c.FbConvesations: "conversation",
			c.FbMessages:     "message",
			c.PcPosOrders:    "order",

			c.FbPages:         "fb_page",
			c.FbMessageItems:  "fb_message_item",
			c.FbPosts:         "fb_post",
			c.FbCustomers:     "fb_customer",
			c.PcPosCustomers:  "pos_customer",
			c.PcPosShops:      "pos_shop",
			c.PcPosWarehouses: "pos_warehouse",
			c.PcPosProducts:   "pos_product",
			c.PcPosVariations: "pos_variation",
			c.PcPosCategories: "pos_category",

			c.CustomerCustomers:       "customer_customer",
			c.CustomerActivityHistory: "customer_activity",
			c.CustomerNotes:         "customer_note",

			c.CixAnalysisResults: "cix_analysis_result",

			c.MetaAdAccounts:               "meta_ad_account",
			c.MetaCampaigns:                "meta_campaign",
			c.MetaAdSets:                   "meta_adset",
			c.MetaAds:                      "meta_ad",
			c.MetaAdInsights:               "meta_ad_insight",
			c.MetaAdInsightsDailySnapshots: "meta_ad_insight_daily_snapshot",

			c.WebhookLogs: "webhook_log",
		}
	})
	return sourceMirrorCollPrefix
}

// EventTypeChangedForCollection trả về wire "<prefix>.changed" nếu collection mirror có trong registry (L1 datachanged / L2 sau merge).
func EventTypeChangedForCollection(collectionName string) (string, bool) {
	collectionName = strings.TrimSpace(collectionName)
	if collectionName == "" {
		return "", false
	}
	if i := strings.IndexByte(collectionName, ','); i >= 0 {
		collectionName = strings.TrimSpace(collectionName[:i])
	}
	prefix, ok := sourceMirrorCollectionPrefixes()[collectionName]
	if !ok || prefix == "" {
		return "", false
	}
	return prefix + ".changed", true
}
