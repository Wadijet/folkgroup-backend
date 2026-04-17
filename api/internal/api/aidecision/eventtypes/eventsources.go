// Package eventtypes — Xem names.go. File này: hằng số EventSource (eventSource) trên decision_events_queue.
//
// Quy ước: EventSource = kênh hoặc đơn vị phát; eventType = nghiệp vụ. Đổi giá trị cần migration queue và client lọc theo source.
// Tài liệu: docs/module-map/co-cau-module-aid-va-domain-queue.md
package eventtypes

import "strings"

// Các giá trị EventSource đã dùng trong production — chỉ thêm hằng số mới khi có luồng mới, đồng bộ doc + consumer.
const (
	// EventSourceL1Datachanged — enqueue sau ghi L1 / hook datachanged (ưu tiên emit mới).
	EventSourceL1Datachanged = "l1_datachanged"
	// EventSourceL2Datachanged — enqueue sau cập nhật canonical L2 (minh hoạ: CRM merge xong → AID), wire thường <prefix>.changed giống G1-S04.
	EventSourceL2Datachanged = "l2_datachanged"
	// EventSourceDatachanged — giá trị lịch sử trên queue; consumer vẫn chấp nhận khi đọc bản ghi cũ.
	EventSourceDatachanged   = "datachanged"
	EventSourceAIDecision    = "aidecision"
	EventSourceDebounce      = "debounce"
	EventSourceCixHTTP       = "cix_api"
	EventSourceCRM           = "crm"
	EventSourceCrmMergeQueue = "crm_merge_queue"
	EventSourceMetaAdsIntel  = "meta_ads_intel"
	EventSourceMetaAPI       = "meta_api"
	EventSourceMetaHooks     = "meta_hooks"
	EventSourceCrmIntel      = "customer_intel" // nguồn intel khách (đồng bộ domain customer_*)
	EventSourceOrderIntel    = "order_intel"
	EventSourceCixIntel      = "cix_intel"
	EventSourceBulk          = "bulk"
	EventSourceAdmin         = "admin"
)

// IsL1DatachangedEventSource — true nếu eventSource là enqueue sau thay đổi mirror/L1 (chuỗi datachanged).
func IsL1DatachangedEventSource(s string) bool {
	switch strings.TrimSpace(s) {
	case EventSourceDatachanged, EventSourceL1Datachanged:
		return true
	default:
		return false
	}
}

// IsL2DatachangedEventSource — true nếu eventSource là báo thay đổi sau merge L2 (đối chiếu l1_datachanged).
func IsL2DatachangedEventSource(s string) bool {
	return strings.TrimSpace(s) == EventSourceL2Datachanged
}
