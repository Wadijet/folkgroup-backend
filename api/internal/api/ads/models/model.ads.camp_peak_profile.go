// Package models — Hourly Peak Matrix (FolkForm v4.1 Section 05).
// camp_peak_profiles: peak hours của từng camp (sau 14 ngày data).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsCampPeakProfile lưu peak hours profile của campaign. Update mỗi tuần (Thứ 2).
type AdsCampPeakProfile struct {
	ID                   primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	CampaignId           string             `json:"campaignId" bson:"campaignId" index:"single:1"`
	AdAccountId          string             `json:"adAccountId" bson:"adAccountId" index:"single:1"`
	OwnerOrganizationID  primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	PeakHours            []int              `json:"peakHours" bson:"peakHours"`       // 7-22, VD: [8,9,10,19,20,21]
	DeadHours            []int              `json:"deadHours" bson:"deadHours"`       // Giờ CR thấp
	AvgCrPerHour         map[string]float64 `json:"avgCrPerHour" bson:"avgCrPerHour"` // "8": 0.12, "9": 0.15
	DataDaysCount        int                `json:"dataDaysCount" bson:"dataDaysCount"` // Số ngày dùng để tính (>= 14)
	DateGenerated        string             `json:"dateGenerated" bson:"dateGenerated"` // YYYY-MM-DD
	CreatedAt            int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt            int64              `json:"updatedAt" bson:"updatedAt"`
}
