package dto

// PublicationCreateInput dữ liệu đầu vào khi tạo publication
type PublicationCreateInput struct {
	VideoID        string                 `json:"videoId" validate:"required"`    // ID của video (L7) được publish (bắt buộc, dạng string ObjectID)
	Platform       string                 `json:"platform" validate:"required"`   // Platform: facebook, tiktok, youtube, instagram (bắt buộc)
	PlatformPostID string                 `json:"platformPostId,omitempty"`        // ID của post trên platform (tùy chọn)
	Status         string                 `json:"status,omitempty"`                // Trạng thái: draft, scheduled, published, archived (mặc định: draft)
	ScheduledAt    *int64                 `json:"scheduledAt,omitempty"`           // Thời gian lên lịch publish (tùy chọn, timestamp milliseconds)
	PublishedAt    *int64                 `json:"publishedAt,omitempty"`           // Thời gian thực tế publish (tùy chọn, timestamp milliseconds)
	MetricsRaw     map[string]interface{} `json:"metricsRaw,omitempty"`            // Raw metrics từ platform: {"views": 1000, "likes": 50, ...}
	Metadata       map[string]interface{} `json:"metadata,omitempty"`             // Metadata bổ sung (caption, hashtags, etc.)
}

// PublicationUpdateInput dữ liệu đầu vào khi cập nhật publication
type PublicationUpdateInput struct {
	VideoID        string                 `json:"videoId,omitempty"`             // ID của video
	Platform       string                 `json:"platform,omitempty"`             // Platform
	PlatformPostID string                 `json:"platformPostId,omitempty"`        // ID của post trên platform
	Status         string                 `json:"status,omitempty"`                // Trạng thái
	ScheduledAt    *int64                 `json:"scheduledAt,omitempty"`           // Thời gian lên lịch publish
	PublishedAt    *int64                 `json:"publishedAt,omitempty"`          // Thời gian thực tế publish
	MetricsRaw     map[string]interface{} `json:"metricsRaw,omitempty"`            // Raw metrics từ platform
	Metadata       map[string]interface{} `json:"metadata,omitempty"`             // Metadata bổ sung
}
