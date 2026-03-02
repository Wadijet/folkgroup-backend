// Package models - CrmPendingIngest thuộc domain CRM.
// Queue cho worker xử lý Merge/Ingest thay vì chạy trực tiếp trong hook.
package models

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmPendingIngest job chờ worker xử lý: MergeFromPosCustomer, IngestOrderTouchpoint, ...
// Hook ghi vào đây; worker đọc và gọi logic CRM.
// BusinessKey dùng để deduplicate: cùng (collectionName, businessKey) chỉ giữ 1 job mới nhất.
type CrmPendingIngest struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	CollectionName      string             `json:"collectionName" bson:"collectionName" index:"single:1,compound:pending_worker,compound:dedup_business"`
	BusinessKey         string             `json:"businessKey" bson:"businessKey" index:"single:1,compound:pending_worker,compound:dedup_business"`
	Operation           string             `json:"operation" bson:"operation"`
	Document            bson.M             `json:"document" bson:"document"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:pending_worker"`
	CreatedAt           int64              `json:"createdAt" bson:"createdAt" index:"compound:pending_worker,order:1"`
	ProcessedAt         *int64             `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:pending_worker"`
	ProcessError        string             `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount          int                `json:"retryCount" bson:"retryCount"`
	// UpdatedAtNew: updated_at của document mới (ms). 0 = không trích được. Dùng debug phân tích extraction.
	UpdatedAtNew int64 `json:"updatedAtNew,omitempty" bson:"updatedAtNew,omitempty"`
	// UpdatedAtOld: updated_at của document cũ (ms). 0 = không có prevDoc hoặc không trích được.
	UpdatedAtOld int64 `json:"updatedAtOld,omitempty" bson:"updatedAtOld,omitempty"`
	// UpdatedAtDeltaMs: chênh lệch (mới - cũ) ms; -1 khi không có prevDoc hoặc không tính được.
	UpdatedAtDeltaMs int64 `json:"updatedAtDeltaMs,omitempty" bson:"updatedAtDeltaMs,omitempty"`
}
