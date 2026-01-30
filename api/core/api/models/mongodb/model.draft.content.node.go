package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DraftApprovalStatus định nghĩa trạng thái approval của draft
const (
	DraftApprovalStatusPending  = "pending"  // Chờ duyệt
	DraftApprovalStatusApproved = "approved" // Đã duyệt
	DraftApprovalStatusRejected = "rejected" // Đã từ chối
	DraftApprovalStatusDraft    = "draft"    // Chưa gửi duyệt (chỉnh sửa)
)

// DraftContentNode đại diện cho draft content node (L1-L6)
// Bản nháp chưa được duyệt, có approval status và link về workflow run
type DraftContentNode struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của draft node

	// ===== CONTENT HIERARCHY =====
	Type     string              `json:"type" bson:"type" index:"single:1"`                    // Loại content node: pillar, stp, insight, contentLine, gene, script
	ParentID *primitive.ObjectID `json:"parentId,omitempty" bson:"parentId,omitempty" index:"single:1"` // ID của parent node (có thể là draft hoặc production)
	ParentDraftID *primitive.ObjectID `json:"parentDraftId,omitempty" bson:"parentDraftId,omitempty" index:"single:1"` // ID của parent draft node (nếu parent là draft)
	Name     string              `json:"name,omitempty" bson:"name,omitempty" index:"text"`     // Tên content node (tùy chọn)
	Text     string              `json:"text" bson:"text" index:"text"`                        // Nội dung text của node

	// ===== WORKFLOW LINK =====
	WorkflowRunID *primitive.ObjectID `json:"workflowRunId,omitempty" bson:"workflowRunId,omitempty" index:"single:1"` // ID của workflow run tạo ra draft này (link về Module 2)
	CreatedByRunID *primitive.ObjectID `json:"createdByRunId,omitempty" bson:"createdByRunId,omitempty" index:"single:1"` // ID của AI run tạo ra draft này (link về Module 2)
	CreatedByStepRunID *primitive.ObjectID `json:"createdByStepRunId,omitempty" bson:"createdByStepRunId,omitempty" index:"single:1"` // ID của step run (link về Module 2)
	CreatedByCandidateID *primitive.ObjectID `json:"createdByCandidateId,omitempty" bson:"createdByCandidateId,omitempty" index:"single:1"` // ID của candidate được chọn (link về Module 2)
	CreatedByBatchID *primitive.ObjectID `json:"createdByBatchId,omitempty" bson:"createdByBatchId,omitempty" index:"single:1"` // ID của generation batch (link về Module 2)

	// ===== APPROVAL STATUS =====
	ApprovalStatus string `json:"approvalStatus" bson:"approvalStatus" index:"single:1"` // Trạng thái approval: pending, approved, rejected, draft

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung (tùy chọn)

	// ===== TIMESTAMPS =====
	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
