// Package models — RuleSuggestion: gợi ý điều chỉnh rule từ phân tích learning cases.
//
// Phase 3: Auto rule generation. Worker phân tích learning_cases → gom theo domain+goalCode+result
// → tạo suggestion khi failure rate cao hoặc pattern bất thường.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RuleSuggestion gợi ý điều chỉnh rule từ learning outcomes.
type RuleSuggestion struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	SuggestionID        string             `json:"suggestionId" bson:"suggestionId" index:"unique:1"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`

	Domain   string `json:"domain" bson:"domain"`     // ads | crm | cix | ...
	GoalCode string `json:"goalCode" bson:"goalCode"`  // Mã mục tiêu (vd: pause_campaign, escalate)
	RuleCode string `json:"ruleCode,omitempty" bson:"ruleCode,omitempty"` // Mã rule liên quan (nếu có)

	// Thống kê từ learning cases
	TotalCases   int     `json:"totalCases" bson:"totalCases"`
	SuccessCount int     `json:"successCount" bson:"successCount"`
	FailedCount  int     `json:"failedCount" bson:"failedCount"`
	RejectedCount int    `json:"rejectedCount" bson:"rejectedCount"`
	FailureRate  float64 `json:"failureRate" bson:"failureRate"` // failedCount / totalCases

	// Gợi ý
	SuggestionType string `json:"suggestionType" bson:"suggestionType"` // review_rule | adjust_threshold | disable_rule
	Message        string `json:"message" bson:"message"`
	Priority       string `json:"priority" bson:"priority"` // high | normal | low

	Status     string `json:"status" bson:"status"` // pending | reviewed | applied | dismissed
	ReviewedAt int64  `json:"reviewedAt,omitempty" bson:"reviewedAt,omitempty"`
	ReviewedBy string `json:"reviewedBy,omitempty" bson:"reviewedBy,omitempty"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
