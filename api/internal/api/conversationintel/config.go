// Package conversationintel — Gom tin nhắn chi tiết (fb_message_items) → yêu cầu Conversation Intelligence (CIX).
package conversationintel

import (
	"os"
	"strconv"
	"strings"
)

// DebounceMs mặc định gom nhiều message_item cùng hội thoại trước khi emit (ms), giống tinh thần adsintel.DebounceMs.
const DebounceMs = 3000

// DebounceMsUrgent khi nội dung tin nhắn match từ khóa gấp — emit ngay (0 ms).
const DebounceMsUrgent = 0

// EffectiveDebounceMs đọc env CONVERSATION_INTEL_DEBOUNCE_MS (rỗng → DebounceMs).
func EffectiveDebounceMs() int {
	s := strings.TrimSpace(os.Getenv("CONVERSATION_INTEL_DEBOUNCE_MS"))
	if s == "" {
		return DebounceMs
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return DebounceMs
	}
	return n
}
