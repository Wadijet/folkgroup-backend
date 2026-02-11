// Package reportdto - DTO cho Report Snapshot (CRUD).
package reportdto

// ReportSnapshotCreateInput dùng cho tạo report snapshot (tầng transport). Thường do engine tạo, API ít dùng.
type ReportSnapshotCreateInput struct {
	ReportKey           string                 `json:"reportKey" validate:"required"`
	PeriodKey           string                 `json:"periodKey" validate:"required"`
	PeriodType          string                 `json:"periodType" validate:"required"`
	Dimensions          map[string]interface{} `json:"dimensions,omitempty"`
	Metrics             map[string]interface{} `json:"metrics" validate:"required"`
}

// ReportSnapshotUpdateInput dùng cho cập nhật report snapshot (tầng transport).
type ReportSnapshotUpdateInput struct {
	PeriodType string                 `json:"periodType"`
	Dimensions map[string]interface{} `json:"dimensions,omitempty"`
	Metrics    map[string]interface{} `json:"metrics"`
}
