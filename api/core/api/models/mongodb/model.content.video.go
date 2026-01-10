package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// VideoStatus định nghĩa trạng thái của video
const (
	VideoStatusPending  = "pending"  // Đang chờ render
	VideoStatusRendering = "rendering" // Đang render
	VideoStatusReady    = "ready"    // Đã render xong, sẵn sàng publish
	VideoStatusFailed   = "failed"   // Render thất bại
	VideoStatusArchived = "archived" // Đã archive
)

// Video đại diện cho video (L7) - đã được duyệt và commit
// Link với script (L6) và có thể có nhiều publications (L8)
type Video struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của video

	// ===== CONTENT HIERARCHY =====
	ScriptID primitive.ObjectID `json:"scriptId" bson:"scriptId" index:"single:1"` // ID của script (L6) tạo ra video này

	// ===== VIDEO ASSETS =====
	AssetURL string `json:"assetUrl,omitempty" bson:"assetUrl,omitempty"` // URL của video file (tùy chọn, có thể chưa có khi pending)
	ThumbnailURL string `json:"thumbnailUrl,omitempty" bson:"thumbnailUrl,omitempty"` // URL của thumbnail (tùy chọn)

	// ===== STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: pending, rendering, ready, failed, archived

	// ===== METADATA =====
	Meta map[string]interface{} `json:"meta,omitempty" bson:"meta,omitempty"` // Metadata bổ sung (duration, resolution, format, etc.)

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== TIMESTAMPS =====
	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
