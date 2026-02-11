package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowRunStatus định nghĩa các trạng thái workflow run
const (
	AIWorkflowRunStatusPending   = "pending"   // Chờ xử lý
	AIWorkflowRunStatusRunning   = "running"   // Đang chạy
	AIWorkflowRunStatusCompleted = "completed"  // Hoàn thành
	AIWorkflowRunStatusFailed    = "failed"    // Thất bại
	AIWorkflowRunStatusCancelled = "cancelled" // Đã hủy
)

// AIWorkflowRun đại diện cho workflow run (Module 2)
// Collection: ai_workflow_runs
type AIWorkflowRun struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của workflow run

	// ===== WORKFLOW REFERENCE =====
	WorkflowID primitive.ObjectID `json:"workflowId" bson:"workflowId" index:"single:1"` // ID của workflow definition

	// ===== ROOT REFERENCE =====
	RootRefID   *primitive.ObjectID `json:"rootRefId,omitempty" bson:"rootRefId,omitempty" index:"single:1"` // ID của root content (ví dụ: Pillar L1) - link về Module 1
	RootRefType string              `json:"rootRefType,omitempty" bson:"rootRefType,omitempty" index:"single:1"` // Loại root reference: "pillar", "stp", etc.

	// ===== EXECUTION STATUS =====
	Status  string `json:"status" bson:"status" index:"single:1"` // Trạng thái: pending, running, completed, failed, cancelled
	CurrentStepIndex int `json:"currentStepIndex" bson:"currentStepIndex"` // Index của step hiện tại đang chạy (0-based)

	// ===== STEP RUNS =====
	StepRunIDs []primitive.ObjectID `json:"stepRunIds,omitempty" bson:"stepRunIds,omitempty"` // Danh sách ID của step runs (theo thứ tự)

	// ===== RESULTS =====
	Result      map[string]interface{} `json:"result,omitempty" bson:"result,omitempty"` // Kết quả workflow run (tùy chọn)
	Error       string                 `json:"error,omitempty" bson:"error,omitempty"`   // Lỗi nếu có
	ErrorDetails map[string]interface{} `json:"errorDetails,omitempty" bson:"errorDetails,omitempty"` // Chi tiết lỗi (stack trace, etc.)

	// ===== PARAMS =====
	Params map[string]interface{} `json:"params,omitempty" bson:"params,omitempty"` // Tham số bổ sung cho workflow run

	// ===== TIMESTAMPS =====
	StartedAt   int64 `json:"startedAt,omitempty" bson:"startedAt,omitempty" index:"single:1"` // Thời gian bắt đầu
	CompletedAt int64 `json:"completedAt,omitempty" bson:"completedAt,omitempty"`            // Thời gian hoàn thành
	CreatedAt   int64 `json:"createdAt" bson:"createdAt" index:"single:1"`                     // Thời gian tạo

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu workflow run

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung
}
