// Package dto - DTO cho CrmPendingIngest (CRUD read-only, queue nội bộ).
package dto

// CrmPendingIngestCreateInput dùng cho tạo CrmPendingIngest (tầng transport).
// Lưu ý: CrmPendingIngest thường được tạo bởi hook; API chỉ hỗ trợ đọc (ReadOnlyConfig).
type CrmPendingIngestCreateInput struct {
	CollectionName string `json:"collectionName" validate:"required"`
	BusinessKey    string `json:"businessKey" validate:"required"`
	Operation      string `json:"operation" validate:"required"`
}

// CrmPendingIngestUpdateInput dùng cho cập nhật CrmPendingIngest (vd: set processedAt).
// Lưu ý: API ReadOnlyConfig không expose Update; struct cần cho BaseHandler.
type CrmPendingIngestUpdateInput struct {
	ProcessedAt  *int64 `json:"processedAt,omitempty"`
	ProcessError string `json:"processError,omitempty"`
}
