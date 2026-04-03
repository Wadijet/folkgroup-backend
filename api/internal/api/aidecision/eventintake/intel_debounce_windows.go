// Package eventintake — Cửa sổ thời gian intelligence + tín hiệu «gấp».
// Debounce ở đây phục vụ cùng mục tiêu với datachanged_defer: gom nhiều sự kiện thành ít lần chuyển việc xuống domain;
// window=0 nghĩa là không gom — chuyển ngay (vẫn qua nhánh quyết định tương ứng trong worker).
package eventintake

import (
	"strings"

	"go.mongodb.org/mongo-driver/bson"
)

// PayloadMarksIntelUrgent — payload ghi đè trì hoãn intelligence: chạy ngay (vẫn qua cùng nhánh schedule, window=0).
func PayloadMarksIntelUrgent(m map[string]interface{}) bool {
	if m == nil {
		return false
	}
	return payloadBoolTrue(m, "immediateSideEffects") ||
		payloadBoolTrue(m, "forceImmediateSideEffects") ||
		payloadBoolTrue(m, "urgentSideEffects")
}

// MessageIntelCriticalPatterns — đồng bộ ý nghĩa với aidecision debounce tin (critical flush).
var MessageIntelCriticalPatterns = []string{"huỷ đơn", "hủy đơn", "cancel", "tôi muốn huỷ"}

// MessageTextMarksIntelUrgent — nội dung tin khớp pattern gấp → bỏ debounce CIX.
func MessageTextMarksIntelUrgent(lower string) bool {
	lower = strings.ToLower(strings.TrimSpace(lower))
	if lower == "" {
		return false
	}
	for _, p := range MessageIntelCriticalPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

// ExtractFbMessageItemTextLower — lấy chuỗi chữ từ messageData (fb_message_items) để so pattern gấp.
func ExtractFbMessageItemTextLower(raw bson.M) string {
	if raw == nil {
		return ""
	}
	md, _ := raw["messageData"].(bson.M)
	if md == nil {
		return ""
	}
	for _, k := range []string{"text", "message", "body", "content"} {
		if s, ok := md[k].(string); ok {
			t := strings.TrimSpace(s)
			if t != "" {
				return strings.ToLower(t)
			}
		}
	}
	return ""
}
