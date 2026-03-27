// Package hooks — registry collection Mongo chứa dữ liệu đồng bộ từ hệ thống bên ngoài → event AI Decision.
package hooks

import (
	"strings"
	"sync"

	"meta_commerce/internal/global"
)

// Bản đồ được dựng sau khi initColNames() chạy (sync.Once lần đầu gọi).
var (
	sourceSyncOnce       sync.Once
	sourceSyncCollPrefix map[string]string // collection name → event prefix (vd fb_page → fb_page.inserted)
)

// sourceSyncPrefixesMap trả về map collection → tiền tố event (entity.inserted / entity.updated).
// Không gồm: auth, notification, delivery, CTA, agent, content, AI, report, queue worker CRM,
// approval/ads internal, decision/learning/rule (trừ cix_analysis_results — fan-in AID sau tính CIX).
func sourceSyncPrefixesMap() map[string]string {
	sourceSyncOnce.Do(func() {
		c := global.MongoDB_ColNames
		sourceSyncCollPrefix = map[string]string{
			// Facebook / Messenger — cùng payload chuẩn; AI Decision tự phân nhánh
			c.FbConvesations: "conversation",
			c.FbMessages:     "message",
			// POS đơn hàng — event order.* ; enrich posData trong hooks
			c.PcPosOrders: "order",

			c.FbPages:         "fb_page",
			c.FbMessageItems:  "fb_message_item",
			c.FbPosts:         "fb_post",
			c.FbCustomers:     "fb_customer",
			c.PcOrders:        "pc_order",
			c.PcPosCustomers:  "pos_customer",
			c.PcPosShops:      "pos_shop",
			c.PcPosWarehouses: "pos_warehouse",
			c.PcPosProducts:   "pos_product",
			c.PcPosVariations: "pos_variation",
			c.PcPosCategories: "pos_category",

			// CRM (khách đã merge / hoạt động / ghi chú — thường từ ingest / đồng bộ)
			c.CrmCustomers:       "crm_customer",
			c.CrmActivityHistory: "crm_activity",
			c.CrmNotes:           "crm_note",

			// CIX — kết quả phân tích hội thoại (ghi DB → datachanged → AID)
			c.CixAnalysisResults: "cix_analysis_result",

			// Meta Marketing API
			c.MetaAdAccounts:               "meta_ad_account",
			c.MetaCampaigns:                "meta_campaign",
			c.MetaAdSets:                   "meta_adset",
			c.MetaAds:                      "meta_ad",
			c.MetaAdInsights:               "meta_ad_insight",
			c.MetaAdInsightsDailySnapshots: "meta_ad_insight_daily_snapshot",

			// Webhook thô (debug / audit luồng ngoài)
			c.WebhookLogs: "webhook_log",
		}
	})
	return sourceSyncCollPrefix
}

// IsSourceSyncDataChangedEvent trả về true nếu eventType có dạng <prefix>.inserted|.updated
// với prefix thuộc registry (dùng cho worker / lane sau này).
func IsSourceSyncDataChangedEvent(eventType string) bool {
	dot := strings.LastIndexByte(eventType, '.')
	if dot <= 0 {
		return false
	}
	pfx := eventType[:dot]
	sfx := eventType[dot+1:]
	if sfx != "inserted" && sfx != "updated" {
		return false
	}
	for _, v := range sourceSyncPrefixesMap() {
		if v == pfx {
			return true
		}
	}
	// Các event đặc biệt đã có trong registry prefix (conversation, message, cix_analysis_result) + order
	switch pfx {
	case "conversation", "message", "order", "cix_analysis_result":
		return true
	default:
		return false
	}
}
