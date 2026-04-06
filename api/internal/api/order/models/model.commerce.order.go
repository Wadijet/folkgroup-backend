// Package ordermodels — Đơn hàng canonical đa nguồn (1:1 mỗi nguồn → một bản ghi); Pancake chỉ mirror ở pc_pos_orders.
package ordermodels

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/utility/identity"
)

// SourcePancakePOS giá trị field Source cho đơn đồng bộ từ Pancake POS (pc_pos_orders).
const SourcePancakePOS = "pancake_pos"

// CommerceOrder bản ghi đơn chuẩn trong hệ — Order Intelligence đọc từ đây.
// Đồng bộ từ pc_pos_orders: 1:1 qua sourceRecordMongoId (không merge đa nguồn vào một đơn).
type CommerceOrder struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Uid                 string             `json:"uid" bson:"uid" index:"compound:idx_commerce_uid_org"`
	Source              string             `json:"source" bson:"source" index:"compound:idx_commerce_source_ref"`
	SourceIds           map[string]string  `json:"sourceIds,omitempty" bson:"sourceIds,omitempty"`
	Links               map[string]identity.LinkItem `json:"links,omitempty" bson:"links,omitempty"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:idx_commerce_uid_org,compound:idx_commerce_source_ref"`

	// SourceRecordMongoID — _id của bản ghi nguồn (vd. pc_pos_orders).
	SourceRecordMongoID primitive.ObjectID `json:"sourceRecordMongoId" bson:"sourceRecordMongoId" index:"compound:idx_commerce_source_ref"`

	// Trường denormalized phục vụ intel (Pancake: copy từ PcPosOrder).
	OrderId      int64                  `json:"orderId" bson:"orderId"`
	Status       int                    `json:"status" bson:"status"`
	InsertedAt   int64                  `json:"insertedAt" bson:"insertedAt"`
	PosUpdatedAt int64                  `json:"posUpdatedAt" bson:"posUpdatedAt"`
	PageId       string                 `json:"pageId,omitempty" bson:"pageId,omitempty"`
	PostId       string                 `json:"postId,omitempty" bson:"postId,omitempty"`
	CustomerId   string                 `json:"customerId,omitempty" bson:"customerId,omitempty"`
	PosData      map[string]interface{} `json:"posData" bson:"posData"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`

	// IntelLastRunId — _id document order_intel_runs mới nhất (lớp lịch sử A).
	IntelLastRunId primitive.ObjectID `json:"intelLastRunId,omitempty" bson:"intelLastRunId,omitempty"`
	// IntelLastComputedAt — unix ms khớp OrderIntelRun.computedAt của lần chạy mới nhất.
	IntelLastComputedAt int64 `json:"intelLastComputedAt,omitempty" bson:"intelLastComputedAt,omitempty"`
	// IntelSequence — $inc mỗi lần ghi run intel thành công; tie-break sort lịch sử với causalOrderingAt.
	IntelSequence int64 `json:"intelSequence,omitempty" bson:"intelSequence,omitempty"`
}
