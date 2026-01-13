package dto

// DraftVideoCreateInput dữ liệu đầu vào khi tạo draft video
type DraftVideoCreateInput struct {
	DraftScriptID   string                 `json:"draftScriptId" validate:"required" transform:"str_objectid"`      // ID của draft script (L6) tạo ra video này (bắt buộc, dạng string ObjectID)
	AssetURL        string                 `json:"assetUrl,omitempty"`                                                // URL của video file (tùy chọn)
	ThumbnailURL    string                 `json:"thumbnailUrl,omitempty"`                                            // URL của thumbnail (tùy chọn)
	Status          string                 `json:"status,omitempty" transform:"string,default=pending"`             // Trạng thái: pending, rendering, ready, failed (mặc định: pending)
	ApprovalStatus  string                 `json:"approvalStatus,omitempty" transform:"string,default=draft"`       // Trạng thái approval: pending, approved, rejected, draft (mặc định: draft)
	Meta            map[string]interface{} `json:"meta,omitempty"`                                                    // Metadata bổ sung (duration, resolution, format, etc.)
}

// DraftVideoUpdateInput dữ liệu đầu vào khi cập nhật draft video
type DraftVideoUpdateInput struct {
	DraftScriptID  string                 `json:"draftScriptId,omitempty" transform:"str_objectid,optional"` // ID của draft script
	AssetURL       string                 `json:"assetUrl,omitempty"`                                           // URL của video file
	ThumbnailURL   string                 `json:"thumbnailUrl,omitempty"`                                       // URL của thumbnail
	Status         string                 `json:"status,omitempty"`                                             // Trạng thái
	ApprovalStatus string                 `json:"approvalStatus,omitempty"`                                      // Trạng thái approval
	Meta           map[string]interface{} `json:"meta,omitempty"`                                                // Metadata bổ sung
}
