// Package models — Counterfactual Kill Tracker (FolkForm v4.1 Section 2.3).
// B1: Snapshot khi kill — lưu đầy đủ metrics tại thời điểm kill.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsKillSnapshot lưu snapshot khi kill một campaign. Dùng cho Counterfactual Tracker B1–B2.
// Mỗi document = 1 lần kill. siblingCampIds lưu danh sách campaign anh em (pattern tương tự).
type AdsKillSnapshot struct {
	ID                  primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	CampaignId          string               `json:"campaignId" bson:"campaignId" index:"single:1"`
	AdAccountId         string               `json:"adAccountId" bson:"adAccountId" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	KillTime            int64                `json:"killTime" bson:"killTime" index:"single:-1"`       // Unix ms
	TriggerRule         string               `json:"triggerRule" bson:"triggerRule"`                  // sl_a, sl_b, sl_d, sl_e, chs_critical, ko_a, ko_b, ko_c, trim_eligible
	ModeDay             string               `json:"modeDay" bson:"modeDay"`                         // BLITZ, NORMAL, EFFICIENCY, PROTECT
	CpaMess             float64              `json:"cpaMess" bson:"cpaMess"`
	ConvRate            float64              `json:"convRate" bson:"convRate"`
	Mess                int64                `json:"mess" bson:"mess"`
	Mqs                 float64              `json:"mqs" bson:"mqs"`
	Chs                 float64              `json:"chs" bson:"chs"`
	Spend               float64              `json:"spend" bson:"spend"`
	SpendPct            float64              `json:"spendPct" bson:"spendPct"`
	SiblingCampIds      []string             `json:"siblingCampIds" bson:"siblingCampIds"` // B2: danh sách camp anh em
	ActionPendingId     primitive.ObjectID   `json:"actionPendingId,omitempty" bson:"actionPendingId,omitempty"`
	CreatedAt           int64                `json:"createdAt" bson:"createdAt"`
}
