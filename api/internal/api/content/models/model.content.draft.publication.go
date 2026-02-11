package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DraftPublication đại diện cho draft publication (L8)
// Bản nháp publication chưa được duyệt, link về draft video
type DraftPublication struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của draft publication

	// ===== CONTENT HIERARCHY =====
	DraftVideoID primitive.ObjectID `json:"draftVideoId" bson:"draftVideoId" index:"single:1"` // ID của draft video (L7) được publish

	// ===== PLATFORM =====
	Platform string `json:"platform" bson:"platform" index:"single:1"` // Platform: facebook, tiktok, youtube, instagram
	PlatformPostID string `json:"platformPostId,omitempty" bson:"platformPostId,omitempty"` // ID của post trên platform (tùy chọn)

	// ===== STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: draft, scheduled

	// ===== APPROVAL STATUS =====
	ApprovalStatus string `json:"approvalStatus" bson:"approvalStatus" index:"single:1"` // Trạng thái approval: pending, approved, rejected, draft

	// ===== SCHEDULING =====
	ScheduledAt *int64 `json:"scheduledAt,omitempty" bson:"scheduledAt,omitempty" index:"single:1"` // Thời gian lên lịch publish (tùy chọn)

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung (caption, hashtags, etc.)

	// ===== TIMESTAMPS =====
	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
