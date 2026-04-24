package eventpipeline

import (
	"strconv"
	"strings"

	"meta_commerce/internal/api/aidecision/datachangedsidefx"

	// Các gói dưới đây dùng init() gọi datachangedsidefx.Register — gom ở đây để server luôn nạp đủ, không phụ thuộc binary có import worker hay không.
	_ "meta_commerce/internal/api/conversationintel/datachanged" // cix_job_intel
	_ "meta_commerce/internal/api/crm/datachanged"                // crm_merge_queue
	_ "meta_commerce/internal/api/meta/datachanged"               // meta_ads_profile
	_ "meta_commerce/internal/api/orderintel/datachanged"         // order_job_intel
	_ "meta_commerce/internal/api/report/datachanged"             // report_redis_touch
)

// package datachangedsidefx còn crm_refresh_register.go (Order 90) — cùng package với Register, init chạy khi import datachangedsidefx.

// EnsureSideEffectModulesLoaded: gọi từ InitRegistry; tác dụng chính là import package eventpipeline
// (các dòng _ ở trên) đã chạy. Snapshot đảm bảo datachangedsidefx (kể cả crm_refresh_register) đã nạp.
func EnsureSideEffectModulesLoaded() {
	_ = datachangedsidefx.Snapshot()
}

// LogLines trả tài liệu ngắn (log / debug) cho luồng + từng bước side-effect từ registry thật.
func LogLines() []string {
	var b strings.Builder
	b.WriteString("eventpipeline datachanged: ")
	b.WriteString(DatachangedSideEffectModuleLine)
	lines := []string{b.String()}
	lines = append(lines, "eventpipeline E2E pha/bước (bảng đầy đủ): "+DatachangedE2ECatalogPointer+"; "+DatachangedE2EDocPointer)
	lines = append(lines, "eventpipeline side-effect contributors (sau khi nạp module):")
	for _, c := range datachangedsidefx.Snapshot() {
		lines = append(lines, "  - order "+strconv.Itoa(c.Order)+" "+c.Name)
	}
	return lines
}
