package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIGenerationBatchStatus định nghĩa các trạng thái generation batch
const (
	AIGenerationBatchStatusPending   = "pending"   // Chờ generate
	AIGenerationBatchStatusGenerating = "generating" // Đang generate
	AIGenerationBatchStatusCompleted = "completed" // Đã generate xong
	AIGenerationBatchStatusFailed    = "failed"    // Thất bại
)

// AIGenerationBatch đại diện cho generation batch (Module 2)
// Collection: ai_generation_batches
type AIGenerationBatch struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của generation batch

	// ===== REFERENCES =====
	StepRunID primitive.ObjectID `json:"stepRunId" bson:"stepRunId" index:"single:1"` // ID của step run tạo ra batch này

	// ===== BATCH INFO =====
	Status      string `json:"status" bson:"status" index:"single:1"` // Trạng thái: pending, generating, completed, failed
	TargetCount int    `json:"targetCount" bson:"targetCount"`       // Số lượng candidates muốn generate
	ActualCount int    `json:"actualCount" bson:"actualCount"`         // Số lượng candidates đã generate thực tế

	// ===== CANDIDATES =====
	CandidateIDs []primitive.ObjectID `json:"candidateIds,omitempty" bson:"candidateIds,omitempty"` // Danh sách ID của candidates trong batch này

	// ===== RESULTS =====
	Error       string                 `json:"error,omitempty" bson:"error,omitempty"`         // Lỗi nếu có
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty" bson:"errorDetails,omitempty"` // Chi tiết lỗi

	// ===== TIMESTAMPS =====
	StartedAt   int64 `json:"startedAt,omitempty" bson:"startedAt,omitempty"` // Thời gian bắt đầu generate
	CompletedAt int64 `json:"completedAt,omitempty" bson:"completedAt,omitempty"` // Thời gian hoàn thành
	CreatedAt   int64 `json:"createdAt" bson:"createdAt" index:"single:1"`         // Thời gian tạo

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu generation batch

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung
}
