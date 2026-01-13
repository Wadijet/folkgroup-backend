package dto

// DraftPublicationCreateInput dữ liệu đầu vào khi tạo draft publication
type DraftPublicationCreateInput struct {
	DraftVideoID   string                 `json:"draftVideoId" validate:"required" transform:"str_objectid"`      // ID của draft video (L7) được publish (bắt buộc, dạng string ObjectID)
	Platform       string                 `json:"platform" validate:"required"`                                     // Platform: facebook, tiktok, youtube, instagram (bắt buộc)
	PlatformPostID string                 `json:"platformPostId,omitempty"`                                          // ID của post trên platform (tùy chọn)
	Status         string                 `json:"status,omitempty" transform:"string,default=draft"`               // Trạng thái: draft, scheduled (mặc định: draft)
	ApprovalStatus string                 `json:"approvalStatus,omitempty" transform:"string,default=draft"`       // Trạng thái approval: pending, approved, rejected, draft (mặc định: draft)
	ScheduledAt    *int64                 `json:"scheduledAt,omitempty"`                                             // Thời gian lên lịch publish (tùy chọn, timestamp milliseconds)
	Metadata       map[string]interface{} `json:"metadata,omitempty"`                                                 // Metadata bổ sung (caption, hashtags, etc.)
}

// DraftPublicationUpdateInput dữ liệu đầu vào khi cập nhật draft publication
type DraftPublicationUpdateInput struct {
	DraftVideoID   string                 `json:"draftVideoId,omitempty" transform:"str_objectid,optional"` // ID của draft video
	Platform       string                 `json:"platform,omitempty"`                                         // Platform
	PlatformPostID string                 `json:"platformPostId,omitempty"`                                   // ID của post trên platform
	Status         string                 `json:"status,omitempty"`                                           // Trạng thái
	ApprovalStatus string                 `json:"approvalStatus,omitempty"`                                   // Trạng thái approval
	ScheduledAt    *int64                 `json:"scheduledAt,omitempty"`                                      // Thời gian lên lịch publish
	Metadata       map[string]interface{} `json:"metadata,omitempty"`                                           // Metadata bổ sung
}
