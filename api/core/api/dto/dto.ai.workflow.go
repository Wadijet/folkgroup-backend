package dto

// AIWorkflowCreateInput dữ liệu đầu vào khi tạo AI workflow
type AIWorkflowCreateInput struct {
	Name        string                        `json:"name" validate:"required"`                    // Tên workflow
	Description string                        `json:"description,omitempty"`                      // Mô tả workflow
	Version     string                        `json:"version" validate:"required"`                // Version của workflow (semver)
	Steps       []AIWorkflowStepReferenceInput `json:"steps" validate:"required"`                  // Danh sách steps trong workflow
	RootRefType string                        `json:"rootRefType" validate:"required"`             // Loại root reference: "layer", "stp", "insight", etc.
	TargetLevel string                        `json:"targetLevel,omitempty"`                      // Level mục tiêu: "L1", "L2", ..., "L8" (tùy chọn)
	DefaultPolicy *AIWorkflowStepPolicyInput  `json:"defaultPolicy,omitempty"`                    // Policy mặc định cho tất cả steps
	Status      string                        `json:"status,omitempty"`                           // Trạng thái: "active", "archived", "draft" (mặc định: "active")
	Metadata    map[string]interface{}        `json:"metadata,omitempty"`                         // Metadata bổ sung
}

// AIWorkflowStepReferenceInput input cho step reference trong workflow
type AIWorkflowStepReferenceInput struct {
	StepID string                      `json:"stepId" validate:"required"` // ID của step definition
	Order  int                         `json:"order" validate:"required"`   // Thứ tự thực thi (0-based)
	Policy *AIWorkflowStepPolicyInput `json:"policy,omitempty"`           // Policy override cho step này
}

// AIWorkflowStepPolicyInput input cho step policy
type AIWorkflowStepPolicyInput struct {
	RetryCount int    `json:"retryCount"`    // Số lần retry khi step fail
	Timeout     int    `json:"timeout"`      // Timeout (seconds)
	OnFailure   string `json:"onFailure"`    // Hành động khi fail: "stop", "continue", "rollback"
	OnSuccess   string `json:"onSuccess"`   // Hành động khi success: "continue", "stop"
	Parallel    bool   `json:"parallel"`    // Có thể chạy parallel với steps khác không
	Condition   string `json:"condition,omitempty"` // Điều kiện để chạy step (expression)
}

// AIWorkflowUpdateInput dữ liệu đầu vào khi cập nhật AI workflow
type AIWorkflowUpdateInput struct {
	Name        string                        `json:"name,omitempty"`                             // Tên workflow
	Description string                        `json:"description,omitempty"`                      // Mô tả workflow
	Version     string                        `json:"version,omitempty"`                           // Version của workflow
	Steps       []AIWorkflowStepReferenceInput `json:"steps,omitempty"`                           // Danh sách steps trong workflow
	RootRefType string                        `json:"rootRefType,omitempty"`                       // Loại root reference
	TargetLevel string                        `json:"targetLevel,omitempty"`                      // Level mục tiêu
	DefaultPolicy *AIWorkflowStepPolicyInput  `json:"defaultPolicy,omitempty"`                   // Policy mặc định
	Status      string                        `json:"status,omitempty"`                           // Trạng thái
	Metadata    map[string]interface{}        `json:"metadata,omitempty"`                          // Metadata bổ sung
}
