// Package reportdto chứa DTO cho domain Report (trend, recompute).
// File: dto.report.go - giữ tên cấu trúc cũ (dto.<domain>.<entity>.go).
package reportdto

// ReportTrendQuery query cho GET trend: reportKey (loại báo cáo), from, to (date dd-mm-yyyy).
type ReportTrendQuery struct {
	ReportKey string `query:"reportKey"` // Key loại báo cáo (vd: order_daily)
	From      string `query:"from"`      // Ngày bắt đầu (dd-mm-yyyy)
	To        string `query:"to"`       // Ngày kết thúc (dd-mm-yyyy)
}

// ReportRecomputeBody body cho POST recompute: reportKey, from, to (date dd-mm-yyyy), giới hạn Phase 1 tối đa 31 ngày.
type ReportRecomputeBody struct {
	ReportKey string `json:"reportKey"` // Key loại báo cáo (vd: order_daily)
	From      string `json:"from"`      // Ngày bắt đầu (dd-mm-yyyy)
	To        string `json:"to"`        // Ngày kết thúc (dd-mm-yyyy)
}

// ReportDateFormat định dạng ngày dùng cho API report: dd-mm-yyyy.
const ReportDateFormat = "02-01-2006"
