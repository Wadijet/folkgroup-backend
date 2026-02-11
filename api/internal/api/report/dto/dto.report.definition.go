// Package reportdto - DTO cho Report Definition (CRUD).
package reportdto

// ReportMetricDefinitionInput dùng cho create/update metric trong report definition.
type ReportMetricDefinitionInput struct {
	OutputKey   string `json:"outputKey"`
	AggType     string `json:"aggType"`     // sum | avg | count | countIf | min | max
	FieldPath   string `json:"fieldPath,omitempty"`
	CountIfExpr string `json:"countIfExpr,omitempty"`
}

// ReportDefinitionCreateInput dùng cho tạo report definition (tầng transport).
type ReportDefinitionCreateInput struct {
	Key              string                       `json:"key" validate:"required"`
	Name             string                      `json:"name" validate:"required"`
	PeriodType       string                      `json:"periodType" validate:"required"` // day | week | month
	PeriodLabel      string                      `json:"periodLabel,omitempty"`
	SourceCollection string                      `json:"sourceCollection" validate:"required"`
	TimeField        string                      `json:"timeField" validate:"required"`
	TimeFieldUnit    string                      `json:"timeFieldUnit,omitempty"` // "second" | "millisecond", mặc định second
	Dimensions       []string                    `json:"dimensions"`
	Metrics          []ReportMetricDefinitionInput `json:"metrics" validate:"required"`
	Metadata         map[string]interface{}      `json:"metadata,omitempty"`
	IsActive         bool                        `json:"isActive"`
}

// ReportDefinitionUpdateInput dùng cho cập nhật report definition (tầng transport).
type ReportDefinitionUpdateInput struct {
	Name             string                       `json:"name"`
	PeriodType       string                       `json:"periodType"`
	PeriodLabel      string                       `json:"periodLabel,omitempty"`
	SourceCollection string                       `json:"sourceCollection"`
	TimeField        string                       `json:"timeField"`
	TimeFieldUnit    string                       `json:"timeFieldUnit,omitempty"`
	Dimensions       []string                     `json:"dimensions"`
	Metrics          []ReportMetricDefinitionInput `json:"metrics"`
	Metadata         map[string]interface{}      `json:"metadata,omitempty"`
	IsActive         *bool                        `json:"isActive"`
}
