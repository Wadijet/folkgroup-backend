// Package adsintel — Cấu hình module.
package adsintel

// DefaultWindowDays số ngày mặc định cho window (raw.window).
const DefaultWindowDays = 7

// DebounceMs thời gian debounce (ms) — tránh recompute trùng cho cùng entity.
const DebounceMs = 3000

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
