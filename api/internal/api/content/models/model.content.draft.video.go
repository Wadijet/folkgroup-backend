package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DraftVideo đại diện cho draft video (L7)
// Bản nháp video chưa được duyệt, link về draft script
type DraftVideo struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của draft video

	// ===== CONTENT HIERARCHY =====
	DraftScriptID primitive.ObjectID `json:"draftScriptId" bson:"draftScriptId" index:"single:1"` // ID của draft script (L6) tạo ra video này

	// ===== VIDEO ASSETS =====
	AssetURL string `json:"assetUrl,omitempty" bson:"assetUrl,omitempty"` // URL của video file (tùy chọn)
	ThumbnailURL string `json:"thumbnailUrl,omitempty" bson:"thumbnailUrl,omitempty"` // URL của thumbnail (tùy chọn)

	// ===== STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: pending, rendering, ready, failed

	// ===== APPROVAL STATUS =====
	ApprovalStatus string `json:"approvalStatus" bson:"approvalStatus" index:"single:1"` // Trạng thái approval: pending, approved, rejected, draft

	// ===== METADATA =====
	Meta map[string]interface{} `json:"meta,omitempty" bson:"meta,omitempty"` // Metadata bổ sung (duration, resolution, format, etc.)

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== TIMESTAMPS =====
	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
