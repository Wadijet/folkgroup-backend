package contentdto

// DraftVideoCreateInput dữ liệu đầu vào khi tạo draft video
type DraftVideoCreateInput struct {
	DraftScriptID  string                 `json:"draftScriptId" validate:"required" transform:"str_objectid"`
	AssetURL       string                 `json:"assetUrl,omitempty"`
	ThumbnailURL   string                 `json:"thumbnailUrl,omitempty"`
	Status         string                 `json:"status,omitempty" transform:"string,default=pending"`
	ApprovalStatus string                 `json:"approvalStatus,omitempty" transform:"string,default=draft"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
}

// DraftVideoUpdateInput dữ liệu đầu vào khi cập nhật draft video
type DraftVideoUpdateInput struct {
	DraftScriptID  string                 `json:"draftScriptId,omitempty" transform:"str_objectid,optional"`
	AssetURL       string                 `json:"assetUrl,omitempty"`
	ThumbnailURL   string                 `json:"thumbnailUrl,omitempty"`
	Status         string                 `json:"status,omitempty"`
	ApprovalStatus string                 `json:"approvalStatus,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
}
