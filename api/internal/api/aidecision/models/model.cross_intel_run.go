package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrossIntelRunInputRef — một đầu vào tham chiếu entity + tùy chọn phiên bản intel (lớp A miền).
// Dùng khi thiết kế collection cross_intel_runs (cross-domain); chưa gắn worker/collection mặc định.
type CrossIntelRunInputRef struct {
	Domain              string `json:"domain" bson:"domain"` // ads_meta | ads_google | crm | order | …
	ObjectType          string `json:"objectType" bson:"objectType"`
	ObjectID            string `json:"objectId" bson:"objectId"`
	OwnerOrganizationID string `json:"ownerOrganizationId" bson:"ownerOrganizationId"`
	IntelRunID          string `json:"intelRunId,omitempty" bson:"intelRunId,omitempty"`
	AsOfMs              int64  `json:"asOfMs,omitempty" bson:"asOfMs,omitempty"`
}

// CrossIntelRun — mẫu bản ghi lớp A cho một lần suy luận cross-domain (output mỏng, inputs đầy đủ ref).
// Trạng thái terminal khớp khung module intelligence (success | failed | skipped).
type CrossIntelRun struct {
	ID                  primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId"`
	ComputedAt          int64                `json:"computedAt,omitempty" bson:"computedAt,omitempty"`
	FailedAt            int64                `json:"failedAt,omitempty" bson:"failedAt,omitempty"`
	Status              string               `json:"status" bson:"status"` // success | failed | skipped
	Inputs              []CrossIntelRunInputRef `json:"inputs,omitempty" bson:"inputs,omitempty"`
	Outputs             map[string]interface{} `json:"outputs,omitempty" bson:"outputs,omitempty"`
	TraceID             string               `json:"traceId,omitempty" bson:"traceId,omitempty"`
	ParentJobID         string               `json:"parentJobId,omitempty" bson:"parentJobId,omitempty"`
	CausalOrderingAt    int64                `json:"causalOrderingAt,omitempty" bson:"causalOrderingAt,omitempty"`
	ErrorCode           string               `json:"errorCode,omitempty" bson:"errorCode,omitempty"`
	ErrorMessage        string               `json:"errorMessage,omitempty" bson:"errorMessage,omitempty"`
}
