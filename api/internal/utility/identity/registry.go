package identity

import (
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
)

// ColConfig cấu hình identity cho 1 collection.
type ColConfig struct {
	Prefix     string
	SourceKeys []SourceKeyConfig  // path -> source key
	LinkKeys   []LinkKeyConfig    // path -> link key, source
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

var registry = map[string]ColConfig{
	global.MongoDB_ColNames.CrmCustomers: {
		Prefix: utility.UIDPrefixCustomer,
		// sourceIds đã có struct riêng, không extract từ payload
	},
	global.MongoDB_ColNames.PcPosCustomers: {
		Prefix: utility.UIDPrefixCustomer,
		SourceKeys: []SourceKeyConfig{{Path: "posData.id", Source: "pos"}},
	},
	global.MongoDB_ColNames.FbCustomers: {
		Prefix: utility.UIDPrefixCustomer,
		SourceKeys: []SourceKeyConfig{{Path: "panCakeData.id", Source: "facebook"}},
	},
	global.MongoDB_ColNames.PcPosOrders: {
		Prefix: utility.UIDPrefixOrder,
		SourceKeys: []SourceKeyConfig{{Path: "posData.id", Source: "pos"}},
		LinkKeys: []LinkKeyConfig{
			{Path: "customerId", Key: "customer", Source: "pos"},
			{Path: "posData.customer.id", Key: "customer", Source: "pos"},
			{Path: "posData.customer_id", Key: "customer", Source: "pos"},
		},
	},
	global.MongoDB_ColNames.FbConvesations: {
		Prefix: utility.UIDPrefixConversation,
		LinkKeys: []LinkKeyConfig{
			{Path: "customerId", Key: "customer", Source: "facebook"},
			{Path: "panCakeData.customer_id", Key: "customer", Source: "facebook"},
			{Path: "panCakeData.customers.0.id", Key: "customer", Source: "facebook"},
			{Path: "panCakeData.page_customer.id", Key: "customer", Source: "facebook"},
		},
	},
	global.MongoDB_ColNames.CrmActivityHistory: {
		Prefix: utility.UIDPrefixActivity,
		// links từ unifiedId trong activity
	},
	global.MongoDB_ColNames.CrmNotes: {
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
		if k != "" {
			names = append(names, k)
		}
	}
	return names
}
