// Package eventtypes — Giai đoạn trong khung quy trình tổng (field pipelineStage trên decision_events_queue).
//
// Khác eventSource (kênh kỹ thuật phát) và eventType (loại sự kiện nghiệp vụ).
// Tài liệu bảng giá trị và checklist: docs/module-map/co-cau-module-aid-va-domain-queue.md (mục 11).
package eventtypes

import "strings"

// Các giá trị pipelineStage — ổn định; đổi chuỗi cần đồng bộ consumer / filter / dashboard.
const (
	// PipelineStageAfterL1Change — Sau thay đổi mirror/L1 (hook datachanged, refresh sau side-effect ingest tương đương).
	PipelineStageAfterL1Change = "after_l1_change"
	// pipelineStageLegacyAfterSourcePersist — wire cũ trên Mongo; IsPipelineStageAfterL1Change vẫn chấp nhận khi đọc bản ghi cũ.
	pipelineStageLegacyAfterSourcePersist = "after_source_persist"
	// PipelineStageAfterL2Merge — Sau merge L1→L2 (ví dụ emit CRM recompute từ crm_merge_queue).
	PipelineStageAfterL2Merge = "after_l2_merge"
	// PipelineStageDomainIntel — Worker miền đã tính intelligence / context_ready (fan-in sau intel).
	PipelineStageDomainIntel = "domain_intel"
	// PipelineStageAIDCoordination — Điều phối nội bộ AI Decision (orchestrate, debounce flush, execute_requested, ads.context_* từ case, …).
	PipelineStageAIDCoordination = "aid_coordination"
	// PipelineStageExternalIngest — HTTP/API hoặc lệnh từ ngoài đưa thẳng vào queue (POST /ai-decision/events, CIX HTTP, Meta API batch, …).
	PipelineStageExternalIngest = "external_ingest"
)

// IsPipelineStageAfterL1Change — true nếu pipelineStage là sau đổi L1 (chuỗi mới hoặc legacy after_source_persist).
func IsPipelineStageAfterL1Change(s string) bool {
	switch strings.TrimSpace(s) {
	case PipelineStageAfterL1Change, pipelineStageLegacyAfterSourcePersist:
		return true
	default:
		return false
	}
}
