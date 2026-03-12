// Package approval — Cơ chế duyệt độc lập. Không phụ thuộc meta_ads, ads.
package approval

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ActionPending document queue đề xuất.
type ActionPending struct {
	ID                   primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	Domain               string                 `json:"domain" bson:"domain" index:"single:1"`
	ActionType           string                 `json:"actionType" bson:"actionType" index:"single:1"`
	Reason               string                 `json:"reason" bson:"reason"`
	Payload              map[string]interface{} `json:"payload" bson:"payload"`
	ProposedAt           int64                  `json:"proposedAt" bson:"proposedAt" index:"single:-1"`
	Status               string                 `json:"status" bson:"status" index:"single:1"`
	ApprovedAt           int64                  `json:"approvedAt,omitempty" bson:"approvedAt,omitempty"`
	RejectedAt           int64                  `json:"rejectedAt,omitempty" bson:"rejectedAt,omitempty"`
	RejectedBy           string                 `json:"rejectedBy,omitempty" bson:"rejectedBy,omitempty"`
	DecisionNote         string                 `json:"decisionNote,omitempty" bson:"decisionNote,omitempty"`
	ExecutedAt           int64                  `json:"executedAt,omitempty" bson:"executedAt,omitempty"`
	ExecuteResponse      map[string]interface{} `json:"executeResponse,omitempty" bson:"executeResponse,omitempty"`
	ExecuteError         string                 `json:"executeError,omitempty" bson:"executeError,omitempty"`
	RetryCount           int                    `json:"retryCount" bson:"retryCount"`                     // Số lần đã retry (cho domain dùng queue)
	NextRetryAt          *int64                 `json:"nextRetryAt,omitempty" bson:"nextRetryAt,omitempty" index:"single:1"` // Thời điểm retry tiếp (Unix sec)
	MaxRetries           int                    `json:"maxRetries" bson:"maxRetries"`                        // Số lần retry tối đa (mặc định 5)
	OwnerOrganizationID  primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	CreatedAt            int64                  `json:"createdAt" bson:"createdAt"`
	UpdatedAt            int64                  `json:"updatedAt" bson:"updatedAt"`
}

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusQueued   = "queued" // Đã duyệt, chờ worker xử lý (domain ads dùng queue)
	StatusRejected = "rejected"
	StatusExecuted = "executed"
	StatusFailed   = "failed"
	StatusCancelled = "cancelled" // User hủy đề xuất trước khi duyệt
)

// MaxRetriesDefault số lần retry mặc định cho domain dùng queue.
const MaxRetriesDefault = 5
