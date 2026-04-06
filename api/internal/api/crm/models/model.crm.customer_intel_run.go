// Package models — Mỗi lần chạy intel khách (job crm_intel_compute): một document lịch sử;
// crm_customers giữ kết quả mới nhất + intelLastRunId / intelLastComputedAt.
package models

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerIntelRun — lịch sử một lần tính/refresh intelligence (một khách hoặc job đa khách).
// MultiCustomerJob: recalculate_all, batch, classification_refresh, … — không cập nhật intelLastRunId từng khách.
type CrmCustomerIntelRun struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	UnifiedID           string             `json:"unifiedId,omitempty" bson:"unifiedId,omitempty" index:"single:1"`
	CustomerMongoID     primitive.ObjectID `json:"customerMongoId,omitempty" bson:"customerMongoId,omitempty"`

	Operation string `json:"operation" bson:"operation"`
	Status    string `json:"status" bson:"status"` // success | failed

	ParentIntelJobID      primitive.ObjectID `json:"parentIntelJobId,omitempty" bson:"parentIntelJobId,omitempty"`
	ParentDecisionEventID string             `json:"parentDecisionEventId,omitempty" bson:"parentDecisionEventId,omitempty"`

	ComputedAt   int64  `json:"computedAt" bson:"computedAt" index:"single:-1"`
	// CausalOrderingAt — unix ms của thay đổi nguồn (merge/datachanged); sort lịch sử theo nghiệp vụ, không phụ thuộc thứ tự worker.
	CausalOrderingAt int64 `json:"causalOrderingAt,omitempty" bson:"causalOrderingAt,omitempty"`
	// IntelSequence — số thứ tự monotonic trên crm_customers sau mỗi lần persist thành công một khách; tie-break khi CausalOrderingAt trùng.
	IntelSequence int64 `json:"intelSequence,omitempty" bson:"intelSequence,omitempty"`

	ErrorMessage string `json:"errorMessage,omitempty" bson:"errorMessage,omitempty"`

	// MetricsSummary — tóm tắt sau lần chạy (audit; không nhân đôi full currentMetrics).
	MetricsSummary bson.M `json:"metricsSummary,omitempty" bson:"metricsSummary,omitempty"`

	MultiCustomerJob     bool   `json:"multiCustomerJob,omitempty" bson:"multiCustomerJob,omitempty"`
	TotalProcessed       int    `json:"totalProcessed,omitempty" bson:"totalProcessed,omitempty"`
	TotalFailed          int    `json:"totalFailed,omitempty" bson:"totalFailed,omitempty"`
	OrgCount             int    `json:"orgCount,omitempty" bson:"orgCount,omitempty"`
	ClassificationMode   string `json:"classificationMode,omitempty" bson:"classificationMode,omitempty"`
}
