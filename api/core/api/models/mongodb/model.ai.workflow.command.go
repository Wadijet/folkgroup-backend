package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowCommandType định nghĩa các loại workflow command
const (
	AIWorkflowCommandTypeStartWorkflow = "START_WORKFLOW" // Bắt đầu workflow run
	AIWorkflowCommandTypeExecuteStep  = "EXECUTE_STEP"   // Chạy một step riêng lẻ để tạo content cho một level
)

// AIWorkflowCommandStatus định nghĩa các trạng thái workflow command
const (
	AIWorkflowCommandStatusPending   = "pending"   // Chờ bot xử lý
	AIWorkflowCommandStatusExecuting = "executing" // Bot đang xử lý
	AIWorkflowCommandStatusCompleted = "completed" // Đã hoàn thành
	AIWorkflowCommandStatusFailed    = "failed"    // Thất bại
	AIWorkflowCommandStatusCancelled = "cancelled" // Đã hủy
)

// AIWorkflowCommand đại diện cho workflow command (Module 2)
// Collection: ai_workflow_commands
// Queue commands cho bot (folkgroup-agent) xử lý
type AIWorkflowCommand struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của workflow command

	// ===== COMMAND INFO =====
	CommandType string `json:"commandType" bson:"commandType" index:"single:1"` // Loại command: START_WORKFLOW, EXECUTE_STEP
	Status      string `json:"status" bson:"status" index:"single:1"`           // Trạng thái: pending, executing, completed, failed, cancelled

	// ===== WORKFLOW REFERENCE =====
	WorkflowID *primitive.ObjectID `json:"workflowId,omitempty" bson:"workflowId,omitempty" index:"single:1"` // ID của workflow definition (bắt buộc nếu CommandType = START_WORKFLOW)

	// ===== STEP REFERENCE =====
	StepID *primitive.ObjectID `json:"stepId,omitempty" bson:"stepId,omitempty" index:"single:1"` // ID của step definition (bắt buộc nếu CommandType = EXECUTE_STEP)

	// ===== ROOT REFERENCE =====
	RootRefID   *primitive.ObjectID `json:"rootRefId,omitempty" bson:"rootRefId,omitempty" index:"single:1"` // ID của root content (ví dụ: Layer L1) - link về Module 1
	RootRefType string              `json:"rootRefType,omitempty" bson:"rootRefType,omitempty" index:"single:1"` // Loại root reference: "layer", "stp", etc.

	// ===== PARAMS =====
	Params map[string]interface{} `json:"params,omitempty" bson:"params,omitempty"` // Tham số bổ sung cho command

	// ===== AGENT INFO =====
	AgentID         string                 `json:"agentId,omitempty" bson:"agentId,omitempty" index:"single:1"` // ID của agent đang xử lý command (tùy chọn)
	AssignedAt      int64                  `json:"assignedAt,omitempty" bson:"assignedAt,omitempty"`            // Thời gian agent nhận command
	LastHeartbeatAt int64                  `json:"lastHeartbeatAt,omitempty" bson:"lastHeartbeatAt,omitempty" index:"single:1"` // Thời gian agent update tiến độ lần cuối (heartbeat)
	Progress        map[string]interface{} `json:"progress,omitempty" bson:"progress,omitempty"`                 // Tiến độ chi tiết của command (ví dụ: {"step": "generating", "percentage": 50})

	// ===== RESULTS =====
	WorkflowRunID *primitive.ObjectID  `json:"workflowRunId,omitempty" bson:"workflowRunId,omitempty" index:"single:1"` // ID của workflow run được tạo (nếu có, cho START_WORKFLOW)
	StepRunID     *primitive.ObjectID  `json:"stepRunId,omitempty" bson:"stepRunId,omitempty" index:"single:1"`         // ID của step run được tạo (nếu có, cho EXECUTE_STEP)
	Result        map[string]interface{} `json:"result,omitempty" bson:"result,omitempty"` // Kết quả thực thi command
	Error         string                 `json:"error,omitempty" bson:"error,omitempty"`     // Lỗi nếu có

	// ===== TIMESTAMPS =====
	CreatedAt   int64 `json:"createdAt" bson:"createdAt" index:"single:1"`         // Thời gian tạo command
	ExecutedAt  int64 `json:"executedAt,omitempty" bson:"executedAt,omitempty"`   // Thời gian bắt đầu thực thi
	CompletedAt int64 `json:"completedAt,omitempty" bson:"completedAt,omitempty"` // Thời gian hoàn thành

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu command

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung
}
