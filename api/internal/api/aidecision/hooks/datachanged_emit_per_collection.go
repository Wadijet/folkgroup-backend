// Package hooks — cấu hình tạm thời trong code (bật/tắt emit queue theo collection).
package hooks

// DatachangedEmitPerCollection — tên collection Mongo → có ghi decision_events_queue sau datachanged hay không.
// Chỉ áp cho key có trong map; collection không khai báo: non-Meta → bật, nhóm Meta → chỉ meta_ad_insights (datachanged_emit_filter.go).
// Sửa map này khi cần tắt/bật từng nguồn.
var DatachangedEmitPerCollection = map[string]bool{
	// Tắt ghi queue AI Decision (datachanged) — theo yêu cầu vận hành
	"fb_posts":                 false,
	"fb_pages":                 false,
	"pc_pos_shops":             false,
	"pc_pos_warehouses":        false,
	"pc_pos_products":          false,
	"pc_pos_variations":        false,
	"pc_pos_categories":        false,
	"crm_customers":            false,
	"crm_activity_history":     false,
	"crm_notes":                false,
	"cix_analysis_results":     false,
	"webhook_logs":             false,
}
