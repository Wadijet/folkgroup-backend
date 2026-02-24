// Package reportdto - DTO cho Report Definition (CRUD).
package reportdto

// ReportMetricDefinitionInput dùng cho create/update metric trong report definition.
// type: "base" (mặc định) | "derived". derived dùng formulaRef + params + scope.
type ReportMetricDefinitionInput struct {
	OutputKey        string            `json:"outputKey"`
	Type             string            `json:"type,omitempty"`             // "base" | "derived"
	AggType          string            `json:"aggType,omitempty"`           // sum | avg | count | countIf | min | max (cho base)
	FieldPath        string            `json:"fieldPath,omitempty"`
	CountIfExpr      string            `json:"countIfExpr,omitempty"`
	SourceCollection string            `json:"sourceCollection,omitempty"`
	TimeField        string            `json:"timeField,omitempty"`
	TimeFieldUnit    string            `json:"timeFieldUnit,omitempty"`
	FormulaRef       string            `json:"formulaRef,omitempty"`       // pct_of_total | avg_from_sum_count | ratio (cho derived)
	Params           map[string]string  `json:"params,omitempty"`           // Tham số cho công thức (cho derived)
	Scope            string            `json:"scope,omitempty"`            // "total" | "perDimension" (cho derived)
}

// ReportDefinitionCreateInput dùng cho tạo report definition (tầng transport).
type ReportDefinitionCreateInput struct {
	Key              string                       `json:"key" validate:"required"`
	Name             string                      `json:"name" validate:"required"`
	PeriodType       string                      `json:"periodType" validate:"required"` // day | week | month | year
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
