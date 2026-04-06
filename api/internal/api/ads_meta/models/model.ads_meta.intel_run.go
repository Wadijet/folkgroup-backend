// Package models — Lớp A: mỗi lần worker ads_intel_compute kết thúc (thành công/thất bại/bỏ qua một campaign);
// meta_campaigns giữ pointer intelLastRunId / intelLastComputedAt / intelSequence khi recompute_one thành công và xác định được campaign.
package models

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	AdsMetaIntelRunStatusSuccess = "success"
	AdsMetaIntelRunStatusFailed  = "failed"
	AdsMetaIntelRunStatusSkipped = "skipped"
)

// AdsMetaIntelRun — một lần chạy intel Meta Ads (theo job queue); sort đề xuất: causalOrderingAt tăng, intelSequence tăng, _id.
type AdsMetaIntelRun struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1;compound:ads_meta_intel_run_parent_job_unique"`

	CampaignId  string `json:"campaignId,omitempty" bson:"campaignId,omitempty"`
	AdAccountId string `json:"adAccountId,omitempty" bson:"adAccountId,omitempty"`
	JobKind     string `json:"jobKind" bson:"jobKind"`
	ObjectType  string `json:"objectType,omitempty" bson:"objectType,omitempty"`
	ObjectID    string `json:"objectId,omitempty" bson:"objectId,omitempty"`

	Operation string `json:"operation,omitempty" bson:"operation,omitempty"`
	Status    string `json:"status" bson:"status"`

	ParentIntelJobID      primitive.ObjectID `json:"parentIntelJobId,omitempty" bson:"parentIntelJobId,omitempty" index:"compound:ads_meta_intel_run_parent_job_unique"`
	ParentDecisionEventID string             `json:"parentDecisionEventId,omitempty" bson:"parentDecisionEventId,omitempty"`

	ComputedAt       int64 `json:"computedAt" bson:"computedAt" index:"single:-1"`
	CausalOrderingAt int64 `json:"causalOrderingAt,omitempty" bson:"causalOrderingAt,omitempty"`
	IntelSequence    int64 `json:"intelSequence,omitempty" bson:"intelSequence,omitempty"`

	ErrorMessage string `json:"errorMessage,omitempty" bson:"errorMessage,omitempty"`

	// IntelSummary — tóm tắt currentMetrics (raw/layer1/layer2/layer3/alertFlags) sau lần chạy, không nhân đôi full pipeline.
	IntelSummary bson.M `json:"intelSummary,omitempty" bson:"intelSummary,omitempty"`

	// MultiCampaignJob — true khi job recalculate_all (một bản ghi cho cả batch, không cập nhật pointer từng campaign).
	MultiCampaignJob bool `json:"multiCampaignJob,omitempty" bson:"multiCampaignJob,omitempty"`
}
