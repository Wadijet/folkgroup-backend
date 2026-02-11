// Package models chứa các model thuộc domain Report.
package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// ReportMetricDefinition định nghĩa một metric trong báo cáo (Phase 1: sum|avg|count|countIf|min|max)
type ReportMetricDefinition struct {
	OutputKey   string `json:"outputKey" bson:"outputKey"`         // Tên trường kết quả (vd: revenue, orderCount)
	AggType     string `json:"aggType" bson:"aggType"`              // sum | avg | count | countIf | min | max
	FieldPath   string `json:"fieldPath,omitempty" bson:"fieldPath,omitempty"`       // Đường dẫn field trong document nguồn (vd: posData.transfer_money)
	CountIfExpr string `json:"countIfExpr,omitempty" bson:"countIfExpr,omitempty"`  // Biểu thức cho countIf (vd: paidAt>0)
}

// ReportDefinition định nghĩa một báo cáo theo chu kỳ (lưu trong report_definitions)
type ReportDefinition struct {
	ID               primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                         // MongoDB _id (tự sinh nếu không gửi)
	Key              string                 `json:"key" bson:"key" index:"unique"`                             // Unique report key (vd: order_daily)
	Name             string                   `json:"name" bson:"name"`                                         // Tên báo cáo
	PeriodType       string                   `json:"periodType" bson:"periodType"`                             // day | week | month
	PeriodLabel      string                   `json:"periodLabel,omitempty" bson:"periodLabel,omitempty"`       // Tên hiển thị chu kỳ (vd: Theo ngày)
	SourceCollection string                   `json:"sourceCollection" bson:"sourceCollection"`                 // Collection nguồn (Phase 1: một report một collection)
	TimeField        string                   `json:"timeField" bson:"timeField"`                               // Field thời gian trong document nguồn (vd: insertedAt)
	TimeFieldUnit    string                   `json:"timeFieldUnit,omitempty" bson:"timeFieldUnit,omitempty"`   // Đơn vị lưu trữ: "second" (mặc định) | "millisecond" — engine dùng để build filter đúng
	Dimensions       []string                 `json:"dimensions" bson:"dimensions"`                             // Group by (vd: ["ownerOrganizationId"])
	Metrics          []ReportMetricDefinition `json:"metrics" bson:"metrics"`                                    // Danh sách metric (outputKey, aggType, fieldPath, countIfExpr)
	Metadata         map[string]interface{}   `json:"metadata,omitempty" bson:"metadata,omitempty"`              // description, category, tags
	IsActive         bool                     `json:"isActive" bson:"isActive"`                                  // Mặc định true
	CreatedAt        int64                    `json:"createdAt" bson:"createdAt"`                               // Unix seconds
	UpdatedAt        int64                    `json:"updatedAt" bson:"updatedAt"`                             // Unix seconds
}
