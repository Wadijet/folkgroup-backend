package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIStepRunStatus định nghĩa các trạng thái step run
const (
	AIStepRunStatusPending   = "pending"   // Chờ xử lý
	AIStepRunStatusRunning   = "running"   // Đang chạy
	AIStepRunStatusCompleted = "completed" // Hoàn thành
	AIStepRunStatusFailed    = "failed"   // Thất bại
	AIStepRunStatusSkipped   = "skipped"   // Đã bỏ qua
)

// AIStepRun đại diện cho step run (Module 2)
// Collection: ai_step_runs
type AIStepRun struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của step run

	// ===== REFERENCES =====
	WorkflowRunID primitive.ObjectID `json:"workflowRunId" bson:"workflowRunId" index:"single:1"` // ID của workflow run
	StepID        primitive.ObjectID `json:"stepId" bson:"stepId" index:"single:1"`              // ID của step definition
	Order         int                `json:"order" bson:"order"`                                   // Thứ tự trong workflow (0-based)

	// ===== EXECUTION STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: pending, running, completed, failed, skipped

	// ===== INPUT/OUTPUT =====
	Input  map[string]interface{} `json:"input,omitempty" bson:"input,omitempty"`   // Input data cho step
	Output map[string]interface{} `json:"output,omitempty" bson:"output,omitempty"` // Output data từ step

	// ===== GENERATION BATCH =====
	GenerationBatchID *primitive.ObjectID `json:"generationBatchId,omitempty" bson:"generationBatchId,omitempty" index:"single:1"` // ID của generation batch (nếu step type là GENERATE)

	// ===== RESULTS =====
	Result      map[string]interface{} `json:"result,omitempty" bson:"result,omitempty"` // Kết quả step (tùy chọn)
	Error       string                 `json:"error,omitempty" bson:"error,omitempty"`   // Lỗi nếu có
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty" bson:"errorDetails,omitempty"` // Chi tiết lỗi

	// ===== TIMESTAMPS =====
	StartedAt   int64 `json:"startedAt,omitempty" bson:"startedAt,omitempty"` // Thời gian bắt đầu
	CompletedAt int64 `json:"completedAt,omitempty" bson:"completedAt,omitempty"` // Thời gian hoàn thành
	CreatedAt   int64 `json:"createdAt" bson:"createdAt" index:"single:1"`         // Thời gian tạo

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu step run

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung
}
