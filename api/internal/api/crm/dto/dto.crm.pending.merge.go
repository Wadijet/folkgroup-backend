// Package dto — DTO cho CrmPendingMerge (queue nội bộ, đọc qua API).
package dto

// CrmPendingMergeCreateInput dùng cho tạo (transport); thường job do hệ thống tạo.
type CrmPendingMergeCreateInput struct {
	CollectionName string `json:"collectionName" validate:"required"`
	BusinessKey    string `json:"businessKey" validate:"required"`
	Operation      string `json:"operation" validate:"required"`
}

// CrmPendingMergeUpdateInput cập nhật (vd processedAt); API ReadOnly thường không expose.
type CrmPendingMergeUpdateInput struct {
	ProcessedAt  *int64 `json:"processedAt,omitempty"`
	ProcessError string `json:"processError,omitempty"`
}
