// Package reportdto - DTO cho Report Dirty Period (CRUD).
package reportdto

// ReportDirtyPeriodCreateInput dùng cho tạo report dirty period (tầng transport). Thường do MarkDirty tạo.
type ReportDirtyPeriodCreateInput struct {
	ReportKey string `json:"reportKey" validate:"required"`
	PeriodKey string `json:"periodKey" validate:"required"`
}

// ReportDirtyPeriodUpdateInput dùng cho cập nhật report dirty period (vd: set processedAt).
type ReportDirtyPeriodUpdateInput struct {
	ProcessedAt *int64 `json:"processedAt,omitempty"`
}
