// Package eventtypes — Xem names.go. File này: hằng số EventSource (eventSource) trên decision_events_queue.
//
// Quy ước: EventSource = kênh hoặc đơn vị phát; eventType = nghiệp vụ. Đổi giá trị cần migration queue và client lọc theo source.
// Tài liệu: docs/module-map/co-cau-module-aid-va-domain-queue.md
package eventtypes

// Các giá trị EventSource đã dùng trong production — chỉ thêm hằng số mới khi có luồng mới, đồng bộ doc + consumer.
const (
	EventSourceDatachanged   = "datachanged"
	EventSourceAIDecision    = "aidecision"
	EventSourceDebounce      = "debounce"
	EventSourceCixHTTP       = "cix_api"
	EventSourceCRM           = "crm"
	EventSourceCrmMergeQueue = "crm_merge_queue"
	EventSourceMetaAdsIntel  = "meta_ads_intel"
	EventSourceMetaAPI       = "meta_api"
	EventSourceMetaHooks     = "meta_hooks"
	EventSourceCrmIntel      = "crm_intel"
	EventSourceOrderIntel    = "order_intel"
	EventSourceCixIntel      = "cix_intel"
	EventSourceBulk          = "bulk"
	EventSourceAdmin         = "admin"
)
