package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIWorkflowStepPolicy định nghĩa policy cho workflow step
type AIWorkflowStepPolicy struct {
	RetryCount    int    `json:"retryCount" bson:"retryCount"`       // Số lần retry khi step fail
	Timeout       int    `json:"timeout" bson:"timeout"`             // Timeout (seconds)
	OnFailure     string `json:"onFailure" bson:"onFailure"`          // Hành động khi fail: "stop", "continue", "rollback"
	OnSuccess     string `json:"onSuccess" bson:"onSuccess"`          // Hành động khi success: "continue", "stop"
	Parallel      bool   `json:"parallel" bson:"parallel"`           // Có thể chạy parallel với steps khác không
	Condition     string `json:"condition,omitempty" bson:"condition,omitempty"` // Điều kiện để chạy step (expression)
}

// AIWorkflowStepReference tham chiếu đến step definition
type AIWorkflowStepReference struct {
	StepID string `json:"stepId" bson:"stepId"` // ID của step definition
	Order  int    `json:"order" bson:"order"`   // Thứ tự thực thi (0-based)
	Policy *AIWorkflowStepPolicy `json:"policy,omitempty" bson:"policy,omitempty"` // Policy override cho step này
}

// AIWorkflow đại diện cho workflow definition (Module 2)
// Collection: ai_workflows
type AIWorkflow struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của workflow

	// ===== BASIC INFO =====
	Name        string `json:"name" bson:"name" index:"text"`                    // Tên workflow
	Description string `json:"description,omitempty" bson:"description,omitempty"` // Mô tả workflow
	Version     string `json:"version" bson:"version" index:"single:1"`           // Version của workflow (semver)

	// ===== WORKFLOW STRUCTURE =====
	Steps []AIWorkflowStepReference `json:"steps" bson:"steps"` // Danh sách steps trong workflow (theo thứ tự)

	// ===== EXECUTION CONFIG =====
	RootRefType string `json:"rootRefType" bson:"rootRefType" index:"single:1"` // Loại root reference: "pillar", "stp", "insight", etc.
	TargetLevel string `json:"targetLevel,omitempty" bson:"targetLevel,omitempty" index:"single:1"` // Level mục tiêu: "L1", "L2", ..., "L8" (tùy chọn)

	// ===== POLICIES =====
	DefaultPolicy *AIWorkflowStepPolicy `json:"defaultPolicy,omitempty" bson:"defaultPolicy,omitempty"` // Policy mặc định cho tất cả steps

	// ===== STATUS =====
	Status string `json:"status" bson:"status" index:"single:1"` // Trạng thái: "active", "archived", "draft"

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu workflow

	// ===== METADATA =====
	Metadata map[string]interface{} `json:"metadata,omitempty" bson:"metadata,omitempty"` // Metadata bổ sung
	CreatedAt int64                  `json:"createdAt" bson:"createdAt" index:"single:1"` // Thời gian tạo
	UpdatedAt int64                  `json:"updatedAt" bson:"updatedAt"`                   // Thời gian cập nhật
}
