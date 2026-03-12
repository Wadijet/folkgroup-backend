// Package dto - DTO cho CrmBulkJob (CRUD read-only, queue nội bộ).
package dto

import "go.mongodb.org/mongo-driver/bson"

// CrmBulkJobCreateInput dùng cho tạo CrmBulkJob (tầng transport).
// Lưu ý: CrmBulkJob thường được tạo bởi sync/backfill/recalculate handlers; API chỉ hỗ trợ đọc (ReadOnlyConfig).
type CrmBulkJobCreateInput struct {
	JobType string `json:"jobType" validate:"required"`
	Params  bson.M `json:"params,omitempty"`
}

// CrmBulkJobUpdateInput dùng cho cập nhật CrmBulkJob qua PUT update-by-id.
// Dùng cho: retry (processedAt=null, processError=""), đặt isPriority.
type CrmBulkJobUpdateInput struct {
	ProcessedAt  *int64 `json:"processedAt,omitempty"`
	ProcessError string `json:"processError,omitempty"`
	IsPriority   *bool  `json:"isPriority,omitempty"`
}
