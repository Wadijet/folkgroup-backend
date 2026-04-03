// Package hooks — registry collection Mongo chứa dữ liệu đồng bộ từ hệ thống bên ngoài → event AI Decision.
// Chuỗi eventType đầy đủ cho pipeline (ads.context_*, crm_intel_recomputed, …): xem package aidecision/eventtypes.
//
// Bảng registry + ghi decision_events_queue mặc định (sau OnDataChanged, theo map + quy tắc Meta trong code):
//
//	Collection Mongo (init.go)              Prefix event              Ghi queue mặc định
//	--------------------------------------  ------------------------  --------------------
//	fb_conversations                        conversation              có
//	fb_messages                             message                   có
//	pc_pos_orders                           order                     có
//	commerce_orders                         (không ghi queue mặc định — chiếu từ pc_pos_orders; DatachangedEmitPerCollection nếu thêm key)
//	fb_pages                                fb_page                   không (DatachangedEmitPerCollection)
//	fb_message_items                        fb_message_item           có
//	fb_posts                                fb_post                   không (DatachangedEmitPerCollection)
//	fb_customers                            fb_customer               có
//	pc_pos_customers                        pos_customer              có
//	pc_pos_shops                            pos_shop                  không (DatachangedEmitPerCollection)
//	pc_pos_warehouses                       pos_warehouse             không (DatachangedEmitPerCollection)
//	pc_pos_products                         pos_product               không (DatachangedEmitPerCollection)
//	pc_pos_variations                       pos_variation             không (DatachangedEmitPerCollection)
//	pc_pos_categories                       pos_category              không (DatachangedEmitPerCollection)
//	crm_customers                           crm_customer              không (DatachangedEmitPerCollection)
//	crm_activity_history                    crm_activity              không (DatachangedEmitPerCollection)
//	crm_notes                               crm_note                  không (DatachangedEmitPerCollection)
//	cix_analysis_results                    cix_analysis_result       không (DatachangedEmitPerCollection)
//	meta_ad_insights                        meta_ad_insight           có
//	meta_ad_accounts                        meta_ad_account           không (meta_insight_only)
//	meta_campaigns                          meta_campaign             không (meta_insight_only)
//	meta_adsets                             meta_adset                không (meta_insight_only)
//	meta_ads                                meta_ad                   không (meta_insight_only)
//	meta_ad_insights_daily_snapshots        meta_ad_insight_daily_snapshot  không (meta_insight_only)
//	webhook_logs                            webhook_log               không (DatachangedEmitPerCollection)
//
// Cột “Ghi queue mặc định” = ShouldEmitDatachangedToDecisionQueue: map DatachangedEmitPerCollection (datachanged_emit_per_collection.go)
// nếu có key; không thì non-Meta → có, nhóm Meta → chỉ meta_ad_insights. Chi tiết: datachanged_emit_filter.go.
package hooks

import (
	"strings"
	"sync"

	"meta_commerce/internal/global"
)

// Bản đồ được dựng sau khi initColNames() chạy (sync.Once lần đầu gọi).
// Bảng collection + bật ghi queue mặc định: xem comment đầu file package.
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
