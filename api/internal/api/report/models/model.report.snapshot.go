// Package models - ReportSnapshot thuộc domain Report.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReportSnapshot lưu kết quả đã tính theo chu kỳ (report_snapshots)
type ReportSnapshot struct {
	ID                  primitive.ObjectID    `json:"id,omitempty" bson:"_id,omitempty"`                                                                   // MongoDB _id
	ReportKey           string                 `json:"reportKey" bson:"reportKey" index:"single:1,compound:report_period_org_unique"`             // Key báo cáo
	PeriodKey           string                 `json:"periodKey" bson:"periodKey" index:"single:1,compound:report_period_org_unique"`             // Vd: 2025-02-01
	PeriodType          string                 `json:"periodType" bson:"periodType"`                                                              // day | week | month
	OwnerOrganizationID primitive.ObjectID    `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:report_period_org_unique"` // Tổ chức sở hữu
	Dimensions          map[string]interface{} `json:"dimensions,omitempty" bson:"dimensions,omitempty"`                                          // (Optional) shopId, ...
	Metrics             map[string]interface{} `json:"metrics" bson:"metrics"`                                                                   // Map outputKey → value (vd: revenue, orderCount)
	ComputedAt          int64                 `json:"computedAt" bson:"computedAt"`                                                               // Unix seconds
	CreatedAt           int64                 `json:"createdAt" bson:"createdAt"`                                                                  // Unix seconds
	UpdatedAt           int64                 `json:"updatedAt" bson:"updatedAt"`                                                                  // Unix seconds
}
