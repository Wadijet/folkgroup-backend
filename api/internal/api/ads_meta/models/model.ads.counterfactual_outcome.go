// Package models — Counterfactual Kill Tracker (FolkForm v4.1 Section 2.3).
// B3–B4: Kết quả theo dõi siblings 4h sau kill → outcome (correct/false_positive/inconclusive).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OutcomeCorrect     = kill đúng — siblings xấu sau 4h
// OutcomeFalsePositive = kill nhầm — siblings tốt sau 4h (Conv Rate > 12%, có đơn Pancake)
// OutcomeInconclusive = không xác định được (thiếu data, không có sibling)
const (
	OutcomeCorrect       = "correct"
	OutcomeFalsePositive = "false_positive"
	OutcomeInconclusive  = "inconclusive"
)

// AdsCounterfactualOutcome lưu kết quả theo dõi siblings 4h sau kill. Dùng cho B3–B4.
// Mỗi document = 1 kill_snapshot đã được đánh giá sau 4h.
type AdsCounterfactualOutcome struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	KillSnapshotId      primitive.ObjectID `json:"killSnapshotId" bson:"killSnapshotId" index:"single:1"`
	CampaignId          string             `json:"campaignId" bson:"campaignId" index:"single:1"`
	AdAccountId         string             `json:"adAccountId" bson:"adAccountId" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	KillTime            int64              `json:"killTime" bson:"killTime" index:"single:-1"`
	EvaluatedAt         int64              `json:"evaluatedAt" bson:"evaluatedAt"` // Kill time + 4h
	// Metrics siblings 4h sau kill (aggregate từ tất cả sibling camps)
	SiblingCr4h       float64 `json:"siblingCr4h" bson:"siblingCr4h"`             // Conv Rate trung bình siblings 4h
	SiblingOrders4h   int64   `json:"siblingOrders4h" bson:"siblingOrders4h"`     // Tổng đơn Pancake siblings 4h
	SiblingCount      int     `json:"siblingCount" bson:"siblingCount"`           // Số sibling có data
	Outcome           string  `json:"outcome" bson:"outcome"`                     // correct | false_positive | inconclusive
	RevenueMissEst    float64 `json:"revenueMissEst,omitempty" bson:"revenueMissEst,omitempty"` // Ước lượng doanh thu mất nếu false_positive
	CreatedAt         int64   `json:"createdAt" bson:"createdAt"`
}
