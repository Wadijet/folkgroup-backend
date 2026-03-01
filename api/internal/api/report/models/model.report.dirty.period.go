// Package models - ReportDirtyPeriod thuộc domain Report.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReportDirtyPeriod đánh dấu chu kỳ cần tính lại (report_dirty_periods)
type ReportDirtyPeriod struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`                                                                   // MongoDB _id
	ReportKey           string              `json:"reportKey" bson:"reportKey" index:"single:1,compound:dirty_org_period,compound:dirty_org_unprocessed"`   // Key báo cáo
	PeriodKey           string              `json:"periodKey" bson:"periodKey" index:"compound:dirty_org_period,compound:dirty_org_unprocessed"`             // Vd: 2025-02-01
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:dirty_org_period,compound:dirty_org_unprocessed"` // Tổ chức
	MarkedAt            int64              `json:"markedAt" bson:"markedAt" index:"compound:dirty_worker_marked,order:1"`                                  // Unix seconds — sort cho worker
	ProcessedAt         *int64              `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:dirty_org_unprocessed,compound:dirty_worker_marked"` // null = chưa xử lý
}
