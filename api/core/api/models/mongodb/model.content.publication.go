package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PublicationStatus định nghĩa trạng thái của publication
const (
	PublicationStatusDraft     = "draft"     // Bản nháp
	PublicationStatusScheduled = "scheduled" // Đã lên lịch
	PublicationStatusPublished = "published" // Đã xuất bản
	PublicationStatusArchived = "archived"  // Đã archive
)

// PublicationPlatform định nghĩa các platform có thể publish
const (
	PublicationPlatformFacebook = "facebook"
	PublicationPlatformTikTok   = "tiktok"
	PublicationPlatformYouTube = "youtube"
	PublicationPlatformInstagram = "instagram"
)

// Publication đại diện cho publication (L8) - đã được duyệt và commit
// Link với video (L7) và có metrics từ platform
type Publication struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của publication

	// ===== CONTENT HIERARCHY =====
	VideoID primitive.ObjectID `json:"videoId" bson:"videoId" index:"single:1"` // ID của video (L7) được publish

	// ===== PLATFORM =====
	Platform string `json:"platform" bson:"platform" index:"single:1"` // Platform: facebook, tiktok, youtube, instagram
	PlatformPostID string `json:"platformPostId,omitempty" bson:"platformPostId,omitempty" index:"single:1"` // ID của post trên platform (tùy chọn, có sau khi publish)

	// ===== STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: draft, scheduled, published, archived

	// ===== METRICS RAW =====
	// MetricsRaw lưu raw metrics từ platform (views, likes, shares, comments)
	// Module 3 đọc MetricsRaw để tính toán performance
	MetricsRaw map[string]interface{} `json:"metricsRaw,omitempty" bson:"metricsRaw,omitempty"` // Raw metrics từ platform: {"views": 1000, "likes": 50, "shares": 10, "comments": 5, "platform_specific": {...}}

	// ===== SCHEDULING =====
	ScheduledAt *int64 `json:"scheduledAt,omitempty" bson:"scheduledAt,omitempty" index:"single:1"` // Thời gian lên lịch publish (tùy chọn)
	PublishedAt *int64 `json:"publishedAt,omitempty" bson:"publishedAt,omitempty" index:"single:1"` // Thời gian thực tế publish (tùy chọn)

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung (caption, hashtags, etc.)

	// ===== TIMESTAMPS =====
	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
