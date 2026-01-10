package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ApprovalRequestStatus định nghĩa trạng thái của approval request
const (
	ApprovalRequestStatusPending  = "pending"  // Chờ duyệt
	ApprovalRequestStatusApproved = "approved" // Đã duyệt
	ApprovalRequestStatusRejected = "rejected" // Đã từ chối
)

// DraftApproval đại diện cho approval request và decision
// Track approval workflow cho drafts (có thể là approval cho workflow run hoặc individual draft)
type DraftApproval struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của approval request

	// ===== APPROVAL TARGET =====
	// Approval có thể cho:
	// 1. Workflow run: approve tất cả drafts của một workflow run
	// 2. Individual draft: approve một draft cụ thể
	WorkflowRunID *primitive.ObjectID `json:"workflowRunId,omitempty" bson:"workflowRunId,omitempty" index:"single:1"` // ID của workflow run (nếu approve cho workflow run)
	DraftNodeID *primitive.ObjectID `json:"draftNodeId,omitempty" bson:"draftNodeId,omitempty" index:"single:1"` // ID của draft node (nếu approve cho individual draft)
	DraftVideoID *primitive.ObjectID `json:"draftVideoId,omitempty" bson:"draftVideoId,omitempty" index:"single:1"` // ID của draft video (nếu approve cho individual draft)
	DraftPublicationID *primitive.ObjectID `json:"draftPublicationId,omitempty" bson:"draftPublicationId,omitempty" index:"single:1"` // ID của draft publication (nếu approve cho individual draft)

	// ===== APPROVAL STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: pending, approved, rejected

	// ===== REQUEST INFO =====
	RequestedBy primitive.ObjectID `json:"requestedBy" bson:"requestedBy" index:"single:1"` // ID của user yêu cầu approval
	RequestedAt int64 `json:"requestedAt" bson:"requestedAt"` // Thời gian yêu cầu approval

	// ===== DECISION INFO =====
	DecidedBy *primitive.ObjectID `json:"decidedBy,omitempty" bson:"decidedBy,omitempty" index:"single:1"` // ID của user quyết định (tùy chọn, có sau khi approve/reject)
	DecidedAt *int64 `json:"decidedAt,omitempty" bson:"decidedAt,omitempty"` // Thời gian quyết định (tùy chọn)
	DecisionNote string `json:"decisionNote,omitempty" bson:"decisionNote,omitempty"` // Ghi chú về quyết định (tùy chọn)

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung (tùy chọn)

	// ===== TIMESTAMPS =====
	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
