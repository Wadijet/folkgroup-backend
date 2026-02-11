// Package reportdto chứa DTO cho domain Report (trend, recompute).
// File: dto.report.go - giữ tên cấu trúc cũ (dto.<domain>.<entity>.go).
package reportdto

// ReportTrendQuery query cho GET trend: reportKey (loại báo cáo), from, to (date YYYY-MM-DD).
type ReportTrendQuery struct {
	ReportKey string `query:"reportKey"` // Key loại báo cáo (vd: order_daily)
	From      string `query:"from"`      // Ngày bắt đầu (YYYY-MM-DD)
	To        string `query:"to"`       // Ngày kết thúc (YYYY-MM-DD)
}

// ReportRecomputeBody body cho POST recompute: reportKey, from, to (date), giới hạn Phase 1 tối đa 31 ngày.
type ReportRecomputeBody struct {
	ReportKey string `json:"reportKey"` // Key loại báo cáo (vd: order_daily)
	From      string `json:"from"`      // Ngày bắt đầu (YYYY-MM-DD)
	To        string `json:"to"`        // Ngày kết thúc (YYYY-MM-DD)
}
