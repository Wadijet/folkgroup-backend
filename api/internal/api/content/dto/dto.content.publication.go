package contentdto

// PublicationCreateInput dữ liệu đầu vào khi tạo publication
type PublicationCreateInput struct {
	VideoID        string                 `json:"videoId" validate:"required" transform:"str_objectid"`
	Platform       string                 `json:"platform" validate:"required"`
	PlatformPostID string                 `json:"platformPostId,omitempty"`
	Status         string                 `json:"status,omitempty" transform:"string,default=draft"`
	ScheduledAt    *int64                 `json:"scheduledAt,omitempty"`
	PublishedAt    *int64                 `json:"publishedAt,omitempty"`
	MetricsRaw     map[string]interface{} `json:"metricsRaw,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// PublicationUpdateInput dữ liệu đầu vào khi cập nhật publication
type PublicationUpdateInput struct {
	VideoID        string                 `json:"videoId,omitempty" transform:"str_objectid,optional"`
	Platform       string                 `json:"platform,omitempty"`
	PlatformPostID string                 `json:"platformPostId,omitempty"`
	Status         string                 `json:"status,omitempty"`
	ScheduledAt    *int64                 `json:"scheduledAt,omitempty"`
	PublishedAt    *int64                 `json:"publishedAt,omitempty"`
	MetricsRaw     map[string]interface{} `json:"metricsRaw,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}
