package identity

import (
	"meta_commerce/internal/utility"
)

// ColConfig cấu hình identity cho 1 collection.
type ColConfig struct {
	Prefix     string
	SourceKeys []SourceKeyConfig // path -> source key
	LinkKeys   []LinkKeyConfig   // path -> link key, source
}

type SourceKeyConfig struct {
	Path   string // dot notation: posData.id, panCakeData.id
	Source string // pos, facebook, zalo
}

type LinkKeyConfig struct {
	Path   string // customerId, posData.customer.id, panCakeData.customer_id
	Key    string // customer, session, order
	Source string // pos, facebook, zalo (để resolve)
}

// registry — key phải là tên collection thật (khớp cmd/server/init.go initColNames).
// Không dùng global.MongoDB_ColNames.Xxx làm key: lúc init package các field đó vẫn "" → mọi entry gộp thành một key rỗng → ShouldEnrich("pc_pos_orders") luôn false.
var registry = map[string]ColConfig{
	"customer_customers": {
		Prefix: utility.UIDPrefixCustomer,
		// sourceIds đã có struct riêng, không extract từ payload
	},
	"pc_pos_customers": {
		Prefix: utility.UIDPrefixCustomer,
		// sourceIds: mọi định danh ngoài của cùng entity khách (POS API id, khách inbox Pancake, PSID ghép page).
		SourceKeys: []SourceKeyConfig{
			{Path: "posData.id", Source: "pos"},
			{Path: "posData.customer_id", Source: "pancake_customer"},
			{Path: "posData.fb_id", Source: "facebook"},
		},
		// links.shop: cửa hàng POS (resolve uid pshp_ khi có resolver; không thì pending + externalRefs).
		LinkKeys: []LinkKeyConfig{
			{Path: "shopId", Key: "shop", Source: "pos"},
		},
	},
	"fb_customers": {
		Prefix:     utility.UIDPrefixCustomer,
		SourceKeys: []SourceKeyConfig{{Path: "panCakeData.id", Source: "facebook"}},
	},
	"pc_pos_orders": {
		Prefix:     utility.UIDPrefixOrder,
		SourceKeys: []SourceKeyConfig{{Path: "posData.id", Source: "pos"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "customerId", Key: "customer", Source: "pos"},
			{Path: "posData.customer.id", Key: "customer", Source: "pos"},
			{Path: "posData.customer_id", Key: "customer", Source: "pos"},
		},
	},
	// order_canonical — đơn canonical đa nguồn; posData giữ layout Pancake khi source=pancake_pos (enrich/CRUD sau này).
	"order_canonical": {
		Prefix:     utility.UIDPrefixOrder,
		SourceKeys: []SourceKeyConfig{{Path: "posData.id", Source: "pos"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "customerId", Key: "customer", Source: "pos"},
			{Path: "posData.customer.id", Key: "customer", Source: "pos"},
			{Path: "posData.customer_id", Key: "customer", Source: "pos"},
		},
	},
	"pc_pos_products": {
		Prefix:     utility.UIDPrefixPosProduct,
		SourceKeys: []SourceKeyConfig{{Path: "posData.id", Source: "pos"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "shopId", Key: "shop", Source: "pos"},
		},
	},
	"pc_pos_categories": {
		Prefix:     utility.UIDPrefixPosCategory,
		SourceKeys: []SourceKeyConfig{{Path: "posData.id", Source: "pos"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "shopId", Key: "shop", Source: "pos"},
		},
	},
	"pc_pos_variations": {
		Prefix:     utility.UIDPrefixPosVariation,
		SourceKeys: []SourceKeyConfig{{Path: "posData.id", Source: "pos"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "productId", Key: "product", Source: "pos"},
			{Path: "shopId", Key: "shop", Source: "pos"},
		},
	},
	"pc_pos_warehouses": {
		Prefix:     utility.UIDPrefixPosWarehouse,
		SourceKeys: []SourceKeyConfig{{Path: "panCakeData.id", Source: "pos"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "shopId", Key: "shop", Source: "pos"},
		},
	},
	"pc_pos_shops": {
		Prefix:     utility.UIDPrefixPosShop,
		SourceKeys: []SourceKeyConfig{{Path: "panCakeData.id", Source: "pos"}},
	},
	"meta_ad_accounts": {
		Prefix:     utility.UIDPrefixMetaAdAccount,
		SourceKeys: []SourceKeyConfig{{Path: "metaData.id", Source: "meta"}},
	},
	"meta_campaigns": {
		Prefix:     utility.UIDPrefixMetaCampaign,
		SourceKeys: []SourceKeyConfig{{Path: "metaData.id", Source: "meta"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "adAccountId", Key: "adAccount", Source: "meta"},
		},
	},
	"meta_adsets": {
		Prefix:     utility.UIDPrefixMetaAdSet,
		SourceKeys: []SourceKeyConfig{{Path: "metaData.id", Source: "meta"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "campaignId", Key: "campaign", Source: "meta"},
			{Path: "adAccountId", Key: "adAccount", Source: "meta"},
		},
	},
	"meta_ads": {
		Prefix:     utility.UIDPrefixMetaAd,
		SourceKeys: []SourceKeyConfig{{Path: "metaData.id", Source: "meta"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "adSetId", Key: "adSet", Source: "meta"},
			{Path: "campaignId", Key: "campaign", Source: "meta"},
			{Path: "adAccountId", Key: "adAccount", Source: "meta"},
		},
	},
	"meta_ad_insights": {
		Prefix:     utility.UIDPrefixMetaInsight,
		SourceKeys: []SourceKeyConfig{{Path: "objectId", Source: "meta"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "adAccountId", Key: "adAccount", Source: "meta"},
		},
	},
	"meta_ad_insights_daily_snapshots": {
		Prefix:     utility.UIDPrefixMetaInsight,
		SourceKeys: []SourceKeyConfig{{Path: "objectId", Source: "meta"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "adAccountId", Key: "adAccount", Source: "meta"},
		},
	},
	"fb_conversations": {
		Prefix: utility.UIDPrefixConversation,
		LinkKeys: []LinkKeyConfig{
			{Path: "customerId", Key: "customer", Source: "facebook"},
			{Path: "panCakeData.customer_id", Key: "customer", Source: "facebook"},
			{Path: "panCakeData.customers.0.id", Key: "customer", Source: "facebook"},
			{Path: "panCakeData.page_customer.id", Key: "customer", Source: "facebook"},
		},
	},
	"fb_messages": {
		Prefix:     utility.UIDPrefixFbMessage,
		SourceKeys: []SourceKeyConfig{{Path: "conversationId", Source: "facebook"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "customerId", Key: "customer", Source: "facebook"},
			{Path: "conversationId", Key: "conversation", Source: "facebook"},
		},
	},
	"fb_message_items": {
		Prefix:     utility.UIDPrefixFbMessageItem,
		SourceKeys: []SourceKeyConfig{{Path: "messageId", Source: "facebook"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "conversationId", Key: "conversation", Source: "facebook"},
		},
	},
	"customer_activity_history": {
		Prefix: utility.UIDPrefixActivity,
		// links từ unifiedId trong activity
	},
	"customer_notes": {
		Prefix: utility.UIDPrefixNote,
		LinkKeys: []LinkKeyConfig{
			{Path: "customerId", Key: "customer", Source: ""},
		},
	},
}

// GetConfig trả về config cho collection (nếu có).
func GetConfig(collectionName string) (ColConfig, bool) {
	c, ok := registry[collectionName]
	return c, ok
}

// ShouldEnrich kiểm tra collection có cần enrich identity không.
func ShouldEnrich(collectionName string) bool {
	_, ok := registry[collectionName]
	return ok
}

// GetAllEnrichedCollectionNames trả về danh sách tên collection cần enrich (cho backfill worker).
func GetAllEnrichedCollectionNames() []string {
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	return names
}
