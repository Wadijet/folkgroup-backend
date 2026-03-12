// Package models — Rule 13 Throttle (1-2-6): trạng thái Ad Set đã bị cap.
// Dùng cho logic Gỡ cap: CPA_Mess < adaptive×0.75x trong 2 checkpoint → Remove.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsThrottleState lưu trạng thái Ad Set đã bị cap 15% (Rule 13).
// Mỗi document = 1 ad set đang bị cap. Dùng để check điều kiện gỡ cap.
type AdsThrottleState struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	CampaignId          string             `json:"campaignId" bson:"campaignId" index:"single:1"`
	AdSetId             string             `json:"adSetId" bson:"adSetId" index:"single:1"`
	AdAccountId         string             `json:"adAccountId" bson:"adAccountId" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	CappedAt            int64              `json:"cappedAt" bson:"cappedAt"`                         // Unix ms — thời điểm cap
	CheckpointOkCount   int                `json:"checkpointOkCount" bson:"checkpointOkCount"`       // Số checkpoint liên tiếp CPA_Mess < adaptive×0.75x
	LastCheckpointAt    int64              `json:"lastCheckpointAt" bson:"lastCheckpointAt"`         // Unix ms — checkpoint gần nhất
	CampaignBudget      float64            `json:"campaignBudget" bson:"campaignBudget"`             // Campaign budget khi cap (để gỡ cap)
	CreatedAt           int64              `json:"createdAt" bson:"createdAt"`
}
