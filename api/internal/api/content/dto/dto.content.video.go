package contentdto

// VideoCreateInput dữ liệu đầu vào khi tạo video
type VideoCreateInput struct {
	ScriptID     string                 `json:"scriptId" validate:"required" transform:"str_objectid"`
	AssetURL     string                 `json:"assetUrl,omitempty"`
	ThumbnailURL string                 `json:"thumbnailUrl,omitempty"`
	Status       string                 `json:"status,omitempty" transform:"string,default=pending"`
	Meta         map[string]interface{} `json:"meta,omitempty"`
}

// VideoUpdateInput dữ liệu đầu vào khi cập nhật video
type VideoUpdateInput struct {
	ScriptID     string                 `json:"scriptId,omitempty" transform:"str_objectid,optional"`
	AssetURL     string                 `json:"assetUrl,omitempty"`
	ThumbnailURL string                 `json:"thumbnailUrl,omitempty"`
	Status       string                 `json:"status,omitempty"`
	Meta         map[string]interface{} `json:"meta,omitempty"`
}
