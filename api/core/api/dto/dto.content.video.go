package dto

// VideoCreateInput dữ liệu đầu vào khi tạo video
type VideoCreateInput struct {
	ScriptID    string                 `json:"scriptId" validate:"required"`    // ID của script (L6) tạo ra video này (bắt buộc, dạng string ObjectID)
	AssetURL    string                 `json:"assetUrl,omitempty"`              // URL của video file (tùy chọn)
	ThumbnailURL string                 `json:"thumbnailUrl,omitempty"`         // URL của thumbnail (tùy chọn)
	Status      string                 `json:"status,omitempty"`                // Trạng thái: pending, rendering, ready, failed, archived (mặc định: pending)
	Meta        map[string]interface{} `json:"meta,omitempty"`                 // Metadata bổ sung (duration, resolution, format, etc.)
}

// VideoUpdateInput dữ liệu đầu vào khi cập nhật video
type VideoUpdateInput struct {
	ScriptID    string                 `json:"scriptId,omitempty"`             // ID của script
	AssetURL    string                 `json:"assetUrl,omitempty"`             // URL của video file
	ThumbnailURL string                 `json:"thumbnailUrl,omitempty"`         // URL của thumbnail
	Status      string                 `json:"status,omitempty"`                // Trạng thái
	Meta        map[string]interface{} `json:"meta,omitempty"`                 // Metadata bổ sung
}
