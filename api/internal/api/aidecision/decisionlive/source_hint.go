package decisionlive

import "strings"

// SourceConversation / SourceOrder / SourceUnknown — giá trị SourceKind trên DecisionLiveEvent.
const (
	SourceConversation = "conversation"
	SourceOrder        = "order"
	SourceUnknown      = "unknown"
	// SourceQueue — milestone từ consumer decision_events_queue (không phải engine ExecuteWithCase).
	SourceQueue = "queue"
)

// InferSourceForFeed suy luận nguồn kích hoạt từ CIX + phiên (mô tả ngắn cho live feed, tiếng Việt).
func InferSourceForFeed(cix map[string]interface{}, sessionUid, customerUid string) (kind string, title string) {
	kind = SourceUnknown
	title = ""
	if cix != nil {
		src, _ := cix["source"].(string)
		orderUID := strings.TrimSpace(stringFromAny(cix["orderUid"]))
		if src == "order_intelligence" || orderUID != "" {
			kind = SourceOrder
			if orderUID == "" {
				orderUID = "(chưa rõ mã đơn)"
			}
			title = "Đơn hàng " + orderUID
			return kind, title
		}
	}
	su := strings.TrimSpace(sessionUid)
	if su != "" {
		kind = SourceConversation
		title = "Hội thoại " + truncateRunes(su, 28)
		return kind, title
	}
	cu := strings.TrimSpace(customerUid)
	if cu != "" {
		kind = SourceConversation
		title = "Khách hàng " + truncateRunes(cu, 28)
		return kind, title
	}
	return kind, title
}

func stringFromAny(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
