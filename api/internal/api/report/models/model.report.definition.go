// Package models chứa các model thuộc domain Report.
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// ReportMetricDefinition định nghĩa một metric trong báo cáo.
// type: "base" (mặc định) = aggregation từ collection; "derived" = tính từ công thức.
type ReportMetricDefinition struct {
	OutputKey        string            `json:"outputKey" bson:"outputKey"`                                     // Tên trường kết quả (vd: revenue, orderCount)
	Type             string            `json:"type,omitempty" bson:"type,omitempty"`                          // "base" | "derived"; rỗng = base
	AggType          string            `json:"aggType,omitempty" bson:"aggType,omitempty"`                     // sum | avg | count | countIf | min | max (cho base)
	FieldPath        string            `json:"fieldPath,omitempty" bson:"fieldPath,omitempty"`                 // Đường dẫn field trong document nguồn (cho base)
	CountIfExpr      string            `json:"countIfExpr,omitempty" bson:"countIfExpr,omitempty"`             // Biểu thức cho countIf (cho base)
	SourceCollection string            `json:"sourceCollection,omitempty" bson:"sourceCollection,omitempty"`  // Collection nguồn; rỗng = dùng cấp report
	TimeField        string            `json:"timeField,omitempty" bson:"timeField,omitempty"`                 // Field thời gian; rỗng = dùng cấp report
	TimeFieldUnit    string            `json:"timeFieldUnit,omitempty" bson:"timeFieldUnit,omitempty"`          // second | millisecond; rỗng = dùng cấp report
	// Derived metric: công thức tham chiếu
	FormulaRef string            `json:"formulaRef,omitempty" bson:"formulaRef,omitempty"` // pct_of_total | avg_from_sum_count | ratio
	Params     map[string]string `json:"params,omitempty" bson:"params,omitempty"`        // Tham số cho công thức (vd: value, total)
	Scope      string            `json:"scope,omitempty" bson:"scope,omitempty"`          // "total" | "perDimension"
}

// ReportDefinition định nghĩa một báo cáo theo chu kỳ (lưu trong report_definitions)
type ReportDefinition struct {
	ID               primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                         // MongoDB _id (tự sinh nếu không gửi)
	Key              string                 `json:"key" bson:"key" index:"unique,compound:report_def_key_active"`                             // Unique report key (vd: order_daily)
	Name             string                   `json:"name" bson:"name"`                                         // Tên báo cáo
	PeriodType       string                   `json:"periodType" bson:"periodType"`                             // day | week | month | year
	PeriodLabel      string                   `json:"periodLabel,omitempty" bson:"periodLabel,omitempty"`       // Tên hiển thị chu kỳ (vd: Theo ngày)
	SourceCollection string                   `json:"sourceCollection" bson:"sourceCollection"`                 // Collection nguồn (Phase 1: một report một collection)
	TimeField        string                   `json:"timeField" bson:"timeField"`                               // Field thời gian trong document nguồn (vd: insertedAt)
	TimeFieldUnit    string                   `json:"timeFieldUnit,omitempty" bson:"timeFieldUnit,omitempty"`   // Đơn vị lưu trữ: "second" (mặc định) | "millisecond" — engine dùng để build filter đúng
	Dimensions       []string                 `json:"dimensions" bson:"dimensions"`                             // Group by (vd: ["ownerOrganizationId"])
	Metrics          []ReportMetricDefinition `json:"metrics" bson:"metrics"`                                    // Danh sách metric (outputKey, aggType, fieldPath, countIfExpr)
	Metadata         map[string]interface{}   `json:"metadata,omitempty" bson:"metadata,omitempty"`              // description, category, tags
	IsActive         bool                     `json:"isActive" bson:"isActive" index:"compound:report_def_key_active"`                         // LoadDefinition filter key + isActive
	CreatedAt        int64                    `json:"createdAt" bson:"createdAt"`                               // Unix seconds
	UpdatedAt        int64                    `json:"updatedAt" bson:"updatedAt"`                             // Unix seconds
}
