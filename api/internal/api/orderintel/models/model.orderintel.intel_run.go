// Package models — Lớp A: mỗi lần worker order_intel_compute kết thúc (thành công/thất bại) — audit, sort theo causalOrderingAt + intelSequence.
package models

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	OrderIntelRunStatusSuccess = "success"
	OrderIntelRunStatusFailed  = "failed"
)

// OrderIntelRun — một lần chạy intel đơn; commerce_orders giữ pointer intelLastRunId / intelLastComputedAt / intelSequence.
type OrderIntelRun struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:order_intel_run_org_uid;compound:order_intel_run_org_orderid;compound:order_intel_run_parent_job_unique"`

	OrderUid string `json:"orderUid,omitempty" bson:"orderUid,omitempty" index:"compound:order_intel_run_org_uid"`
	OrderID  int64  `json:"orderId,omitempty" bson:"orderId,omitempty" index:"compound:order_intel_run_org_orderid"`

	CommerceOrderMongoID primitive.ObjectID `json:"commerceOrderMongoId,omitempty" bson:"commerceOrderMongoId,omitempty"`

	Operation string `json:"operation" bson:"operation"` // nguồn job: aidecision_order, recompute, ...
	Status    string `json:"status" bson:"status"`         // success | failed

	// ParentIntelJobID — khóa idempotent với ownerOrganizationId (một run / một job order_intel_compute, kể cả retry).
	ParentIntelJobID primitive.ObjectID `json:"parentIntelJobId,omitempty" bson:"parentIntelJobId,omitempty" index:"compound:order_intel_run_parent_job_unique"`
	ParentEventID     string             `json:"parentEventId,omitempty" bson:"parentEventId,omitempty"`
	ParentEventType   string             `json:"parentEventType,omitempty" bson:"parentEventType,omitempty"`
	TraceID           string             `json:"traceId,omitempty" bson:"traceId,omitempty"`
	CorrelationID     string             `json:"correlationId,omitempty" bson:"correlationId,omitempty"`

	ComputedAt int64 `json:"computedAt" bson:"computedAt" index:"single:-1"`
	// CausalOrderingAt — mốc nghiệp vụ (payload causalOrderingAtMs hoặc lúc enqueue); sort lịch sử khi worker không FIFO.
	CausalOrderingAt int64 `json:"causalOrderingAt,omitempty" bson:"causalOrderingAt,omitempty"`
	// IntelSequence — bản sao số thứ tự monotonic trên commerce_orders sau lần chạy thành công; tie-break với CausalOrderingAt.
	IntelSequence int64 `json:"intelSequence,omitempty" bson:"intelSequence,omitempty"`

	ErrorCode    string `json:"errorCode,omitempty" bson:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty" bson:"errorMessage,omitempty"`

	// Raw — facts tại thời điểm tính (derive lại layer trên nếu version hóa logic).
	Raw OrderIntelRaw `json:"raw" bson:"raw"`
	// IntelSummary — tóm tắt layer1/layer2/layer3/flags sau lần chạy (không nhân đôi full snapshot collection).
	IntelSummary bson.M `json:"intelSummary,omitempty" bson:"intelSummary,omitempty"`
}
