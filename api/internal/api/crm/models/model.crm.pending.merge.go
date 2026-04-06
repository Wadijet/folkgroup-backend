// Package models — CrmPendingMerge: queue L1→L2 CRM (khác CIO ingest).
// Worker đọc collection crm_pending_merge và gọi merge/touchpoint; không gọi trong hook datachanged.
package models

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmPendingMergeSnapshot — một bản ghi nguồn L1 trong job đã gộp (coalesce theo inbox customer).
// SnapshotKey = collectionName|entityPart (orderId, conversationId, customerId, …) để thay thế bản mới nhất cùng khóa.
type CrmPendingMergeSnapshot struct {
	CollectionName string `json:"collectionName" bson:"collectionName"`
	SnapshotKey    string `json:"snapshotKey" bson:"snapshotKey"`
	Operation      string `json:"operation,omitempty" bson:"operation,omitempty"`
	Document       bson.M `json:"document" bson:"document"`
}

// CrmPendingMerge job chờ worker: MergeFromPosCustomer, IngestOrderTouchpoint, …
// BusinessKey deduplicate (legacy): cùng (collectionName, businessKey) chỉ một job.
// CoalesceKey: gộp nhiều nguồn L1 cùng inboxCustomerId; MergeNotBefore debounce trailing.
type CrmPendingMerge struct {
	ID                  primitive.ObjectID        `json:"id,omitempty" bson:"_id,omitempty"`
	CollectionName      string                    `json:"collectionName" bson:"collectionName" index:"single:1,compound:pending_worker,compound:dedup_business"`
	BusinessKey         string                    `json:"businessKey" bson:"businessKey" index:"single:1,compound:pending_worker,compound:dedup_business"`
	Operation           string                    `json:"operation" bson:"operation"`
	Document            bson.M                    `json:"document" bson:"document"`
	OwnerOrganizationID primitive.ObjectID        `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:pending_worker"`
	CreatedAt           int64                     `json:"createdAt" bson:"createdAt" index:"compound:pending_worker,order:1"`
	ProcessedAt         *int64                    `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:pending_worker"`
	ProcessError        string                    `json:"processError,omitempty" bson:"processError,omitempty"`
	RetryCount          int                       `json:"retryCount" bson:"retryCount"`
	UpdatedAtNew        int64                     `json:"updatedAtNew,omitempty" bson:"updatedAtNew,omitempty"`
	UpdatedAtOld        int64                     `json:"updatedAtOld,omitempty" bson:"updatedAtOld,omitempty"`
	UpdatedAtDeltaMs    int64                     `json:"updatedAtDeltaMs,omitempty" bson:"updatedAtDeltaMs,omitempty"`
	SourceCollections   []string                  `json:"sourceCollections,omitempty" bson:"sourceCollections,omitempty"`
	SourceSnapshots     []CrmPendingMergeSnapshot `json:"sourceSnapshots,omitempty" bson:"sourceSnapshots,omitempty"`
	CoalesceKey         string                    `json:"coalesceKey,omitempty" bson:"coalesceKey,omitempty" index:"single:1"`
	InboxCustomerId     string                    `json:"inboxCustomerId,omitempty" bson:"inboxCustomerId,omitempty"`
	MergeNotBefore      int64                     `json:"mergeNotBefore,omitempty" bson:"mergeNotBefore,omitempty"`
}
