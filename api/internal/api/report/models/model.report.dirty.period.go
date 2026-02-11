// Package models - ReportDirtyPeriod thuộc domain Report.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReportDirtyPeriod đánh dấu chu kỳ cần tính lại (report_dirty_periods)
type ReportDirtyPeriod struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`                                                                   // MongoDB _id
	ReportKey           string              `json:"reportKey" bson:"reportKey" index:"single:1,compound:dirty_processed"`   // Key báo cáo
	PeriodKey           string              `json:"periodKey" bson:"periodKey"`                                              // Vd: 2025-02-01
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId"`                         // Tổ chức
	MarkedAt            int64              `json:"markedAt" bson:"markedAt"`                                               // Unix seconds
	ProcessedAt         *int64              `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:dirty_processed"` // Unix seconds; null = chưa xử lý
}
