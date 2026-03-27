// Package adsintel — Cấu hình module.
package adsintel

// DefaultWindowDays số ngày mặc định cho window (raw.window).
const DefaultWindowDays = 7

// DebounceMs thời gian debounce (ms) cho nguồn đơn/hội thoại — tránh recompute trùng cùng entity.
const DebounceMs = 3000

// DebounceMsInsightBatch — gom recompute Ads Intelligence khi meta_ad_insights không gấp (mặc định 15 phút).
const DebounceMsInsightBatch = 15 * 60 * 1000

// ObjectType constants.
const (
	ObjectTypeAd       = "ad"
	ObjectTypeAdSet    = "adset"
	ObjectTypeCampaign = "campaign"
	ObjectTypeAccount  = "ad_account"
)

// Source constants cho UpdateRawFromSource.
const (
	SourceMeta             = "meta"
	SourcePancakePos       = "pancake.pos"
	SourcePancakeConversation = "pancake.conversation"
)
