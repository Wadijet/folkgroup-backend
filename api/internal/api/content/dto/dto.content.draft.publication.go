package contentdto

// DraftPublicationCreateInput dữ liệu đầu vào khi tạo draft publication
type DraftPublicationCreateInput struct {
	DraftVideoID   string                 `json:"draftVideoId" validate:"required" transform:"str_objectid"`
	Platform       string                 `json:"platform" validate:"required"`
	PlatformPostID string                 `json:"platformPostId,omitempty"`
	Status         string                 `json:"status,omitempty" transform:"string,default=draft"`
	ApprovalStatus string                 `json:"approvalStatus,omitempty" transform:"string,default=draft"`
	ScheduledAt    *int64                 `json:"scheduledAt,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// DraftPublicationUpdateInput dữ liệu đầu vào khi cập nhật draft publication
type DraftPublicationUpdateInput struct {
	DraftVideoID   string                 `json:"draftVideoId,omitempty" transform:"str_objectid,optional"`
	Platform       string                 `json:"platform,omitempty"`
	PlatformPostID string                 `json:"platformPostId,omitempty"`
	Status         string                 `json:"status,omitempty"`
	ApprovalStatus string                 `json:"approvalStatus,omitempty"`
	ScheduledAt    *int64                 `json:"scheduledAt,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}
