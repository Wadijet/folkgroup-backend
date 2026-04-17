// Package eventtypes — Hằng số eventType cho decision_events_queue (một nguồn để dispatch / lane / tier / emit thống nhất).
//
// Quy ước chuỗi (ổn định — đổi giá trị cần migration dữ liệu queue & tích hợp ngoài):
//
//  1. Ngữ cảnh snapshot (bridge AID ↔ worker): {miền}.context_requested | {miền}.context_ready
//     Ví dụ: ads, customer.
//
//  2. Yêu cầu tính intelligence / phân tích: {miền}.intelligence.{động_từ}_requested hoặc {miền}.{động_từ}_requested
//     Ví dụ: crm.intelligence.compute_requested, ads.intelligence.recompute_requested, order.recompute_requested.
//     Legacy: order.intelligence_requested (một segment sau dấu chấm — giữ để tương thích queue cũ).
//
//  3. Sau worker intel (fan-in vào AID): {miền}_intel_recomputed (underscore — đã dùng trong production).
//
//  4. Đồng bộ Mongo (datachanged): {prefix}.changed theo hooks/datachanged.go (legacy queue có thể còn .inserted/.updated).
//
//  5. Đề xuất / thực thi: executor.propose_requested, ads.propose_requested, aidecision.execute_requested.
package eventtypes

// Tiền tố eventType (phân loại feed / enrich — không thay chuỗi đầy đủ).
const (
	PrefixAdsContext              = "ads.context_"
	PrefixAdsIntelligence         = "ads.intelligence."
	PrefixCampaignIntel           = "campaign_intel"
	PrefixCrmDot                  = "crm."
	PrefixCrmUnderscore           = "crm_" // crm_customer.*, crm_intel_* …
	PrefixCrmIntelUnderscore      = "crm_intel_"
	PrefixCustomerContext         = "customer.context_"
	PrefixCixDot                  = "cix."
	PrefixConversation            = "conversation."
	PrefixMessage                 = "message."
	PrefixOrder                   = "order."
	PrefixOrderIntelligenceLegacy = "order.intelligence_" // khớp order.intelligence_requested
	PrefixOrderRecompute          = "order.recompute_"
	PrefixOrderIntelUnderscore    = "order_intel_"
	PrefixCixIntelUnderscore      = "cix_intel_"
)

const (
	// --- Ads pipeline ---
	AdsContextRequested                    = "ads.context_requested"
	AdsContextReady                        = "ads.context_ready"
	AdsUpdated                             = "ads.updated"
	AdsIntelligenceRecomputeRequested      = "ads.intelligence.recompute_requested"
	AdsIntelligenceRecalculateAllRequested = "ads.intelligence.recalculate_all_requested"
	CampaignIntelRecomputed                = "campaign_intel_recomputed"
	MetaCampaignInserted                   = "meta_campaign.inserted" // legacy queue
	MetaCampaignUpdated                    = "meta_campaign.updated"   // legacy queue
	MetaCampaignChanged                    = "meta_campaign.changed"
	AdsProposeRequested                    = "ads.propose_requested"

	// --- Customer / CRM intelligence queue ---
	CustomerContextRequested          = "customer.context_requested"
	CustomerContextReady              = "customer.context_ready"
	CrmIntelligenceComputeRequested   = "crm.intelligence.compute_requested"
	CrmIntelligenceRecomputeRequested = "crm.intelligence.recompute_requested"
	CrmIntelRecomputed                = "crm_intel_recomputed"

	// --- CIX ---
	CixAnalysisRequested = "cix.analysis_requested"
	CixIntelRecomputed   = "cix_intel_recomputed"

	// --- Order ---
	OrderInserted              = "order.inserted" // legacy queue
	OrderUpdated               = "order.updated"  // legacy queue
	OrderChanged               = "order.changed"
	OrderIntelligenceRequested = "order.intelligence_requested"
	OrderRecomputeRequested    = "order.recompute_requested"
	OrderIntelRecomputed       = "order_intel_recomputed"

	// --- Conversation / message ---
	ConversationInserted        = "conversation.inserted" // legacy queue
	ConversationUpdated         = "conversation.updated"  // legacy queue
	ConversationChanged         = "conversation.changed"
	MessageInserted             = "message.inserted" // legacy queue
	MessageUpdated              = "message.updated"  // legacy queue
	MessageChanged              = "message.changed"
	ConversationMessageInserted = "conversation.message_inserted"
	MessageBatchReady           = "message.batch_ready"

	// --- Execute / propose ---
	AIDecisionExecuteRequested = "aidecision.execute_requested"
	ExecutorProposeRequested   = "executor.propose_requested"

	// --- Datachanged: khách (tier pipeline / nhãn UI) ---
	PosCustomerInserted = "pos_customer.inserted"
	PosCustomerUpdated  = "pos_customer.updated"
	FbCustomerInserted  = "fb_customer.inserted"
	FbCustomerUpdated   = "fb_customer.updated"
	CrmCustomerInserted = "crm_customer.inserted"
	CrmCustomerUpdated  = "crm_customer.updated"

	// --- Datachanged: POS / Meta chi tiết (livecopy narrative) ---
	PosVariationUpdated  = "pos_variation.updated"
	PosProductUpdated    = "pos_product.updated"
	PosShopUpdated       = "pos_shop.updated"
	PosWarehouseUpdated  = "pos_warehouse.updated"
	MetaAdUpdated        = "meta_ad.updated"
	MetaAdsetUpdated     = "meta_adset.updated"
	MetaAdInsightUpdated = "meta_ad_insight.updated"
	MetaAdAccountUpdated = "meta_ad_account.updated"
)
