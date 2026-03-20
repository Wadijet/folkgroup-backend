// Package models — LearningInsightAggregate: thống kê anonymized từ learning cases (cross-merchant).
//
// Phase 3: Cross-merchant learning. Chỉ aggregate từ org có consent (learning_share_consent).
// Không lưu ownerOrganizationId trong aggregate — anonymized.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LearningInsightAggregate thống kê aggregate anonymized theo domain+goalCode.
type LearningInsightAggregate struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Domain   string `json:"domain" bson:"domain" index:"single:1"`
	GoalCode string `json:"goalCode" bson:"goalCode" index:"single:1"`

	TotalCases    int     `json:"totalCases" bson:"totalCases"`
	SuccessCount  int     `json:"successCount" bson:"successCount"`
	FailedCount   int     `json:"failedCount" bson:"failedCount"`
	RejectedCount int     `json:"rejectedCount" bson:"rejectedCount"`
	SuccessRate   float64 `json:"successRate" bson:"successRate"`

	OrgCount int `json:"orgCount" bson:"orgCount"` // Số org đóng góp (anonymized)

	PeriodStart int64 `json:"periodStart" bson:"periodStart"` // Unix ms
	PeriodEnd   int64 `json:"periodEnd" bson:"periodEnd"`
	UpdatedAt   int64 `json:"updatedAt" bson:"updatedAt"`
}
