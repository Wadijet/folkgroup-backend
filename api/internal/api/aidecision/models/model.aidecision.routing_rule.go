// Package models — Quy tắc routing event theo org (bước đầu tới rule store PLATFORM_L1 §15).
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RoutingBehaviorNoop — consumer bỏ qua handler đã đăng ký (event vẫn complete).
const RoutingBehaviorNoop = "noop"

// RoutingBehaviorPassThrough — ghi nhận explicit cho phép dispatch (giống không có rule).
const RoutingBehaviorPassThrough = "pass_through"

// DecisionRoutingRule override hành vi xử lý event_type cho một org.
type DecisionRoutingRule struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:routing_org_event_unique"`
	EventType           string             `json:"eventType" bson:"eventType" index:"compound:routing_org_event_unique"`
	Enabled             bool               `json:"enabled" bson:"enabled"`
	// Behavior: noop = không gọi handler; pass_through = dispatch bình thường (dùng để bật lại sau noop).
	Behavior  string `json:"behavior" bson:"behavior"`
	Note      string `json:"note,omitempty" bson:"note,omitempty"`
	UpdatedAt int64  `json:"updatedAt" bson:"updatedAt"`
	CreatedAt int64  `json:"createdAt" bson:"createdAt"`
}
