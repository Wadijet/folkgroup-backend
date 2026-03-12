// Package models - CrmBulkJob queue cho worker xử lý sync, backfill, recalculate.
package models

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmBulkJobType loại job bulk: sync, backfill, rebuild, recalculate_one, recalculate_all.
const (
	CrmBulkJobSync             = "sync"
	CrmBulkJobBackfill         = "backfill"
	CrmBulkJobRebuild          = "rebuild"
	CrmBulkJobRecalculateOne   = "recalculate_one"
	CrmBulkJobRecalculateAll   = "recalculate_all"
)

// CrmBulkJob job bulk CRM: sync, backfill, rebuild, recalculate.
type CrmBulkJob struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	JobType             string             `json:"jobType" bson:"jobType" index:"single:1,compound:bulk_worker"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:bulk_worker"`
	Params              bson.M             `json:"params" bson:"params"` // sources, types, limit, unifiedId...
	IsPriority          bool               `json:"isPriority" bson:"isPriority"` // Job ưu tiên: bắt buộc chạy, không bị throttle
	CreatedAt           int64              `json:"createdAt" bson:"createdAt" index:"compound:bulk_worker,order:1"`
	ProcessedAt         *int64             `json:"processedAt,omitempty" bson:"processedAt,omitempty" index:"single:1,compound:bulk_worker"`
	ProcessError        string             `json:"processError,omitempty" bson:"processError,omitempty"`
	Result              bson.M             `json:"result,omitempty" bson:"result,omitempty"` // Kết quả khi thành công
}
