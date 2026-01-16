package dto

// AIWorkflowCommandCreateInput dữ liệu đầu vào khi tạo AI workflow command
// Lưu ý:
// - Nếu CommandType = "START_WORKFLOW": WorkflowID là bắt buộc, StepID không cần
// - Nếu CommandType = "EXECUTE_STEP": StepID là bắt buộc, WorkflowID không cần
// - RootRefID và RootRefType luôn bắt buộc để xác định parent content node
type AIWorkflowCommandCreateInput struct {
	CommandType string                 `json:"commandType" validate:"required,oneof=START_WORKFLOW EXECUTE_STEP"` // Loại command: START_WORKFLOW, EXECUTE_STEP
	WorkflowID  string                 `json:"workflowId,omitempty" transform:"str_objectid_ptr,optional"` // ID của workflow definition (dạng string ObjectID) - bắt buộc nếu CommandType = START_WORKFLOW
	StepID      string                 `json:"stepId,omitempty" transform:"str_objectid_ptr,optional"` // ID của step definition (dạng string ObjectID) - bắt buộc nếu CommandType = EXECUTE_STEP
	RootRefID   string                 `json:"rootRefId" validate:"required" transform:"str_objectid_ptr"` // ID của root content (dạng string ObjectID) - parent node để tạo level tiếp theo
	RootRefType string                 `json:"rootRefType" validate:"required"` // Loại root reference: "layer", "stp", etc. - phải match với ParentLevel của step
	Params      map[string]interface{} `json:"params,omitempty"`                 // Tham số bổ sung cho command
	Metadata    map[string]interface{} `json:"metadata,omitempty"`             // Metadata bổ sung
}

// AIWorkflowCommandUpdateInput dữ liệu đầu vào khi cập nhật AI workflow command
type AIWorkflowCommandUpdateInput struct {
	Status        string                 `json:"status,omitempty"`                 // Trạng thái: pending, executing, completed, failed, cancelled
	AgentID       string                 `json:"agentId,omitempty"`                // ID của agent đang xử lý command
	WorkflowRunID string                 `json:"workflowRunId,omitempty" transform:"str_objectid_ptr,optional"` // ID của workflow run được tạo (dạng string ObjectID)
	StepRunID     string                 `json:"stepRunId,omitempty" transform:"str_objectid_ptr,optional"`     // ID của step run được tạo (dạng string ObjectID)
	Result        map[string]interface{} `json:"result,omitempty"`                 // Kết quả thực thi command
	Error         string                 `json:"error,omitempty"`                  // Lỗi nếu có
	Metadata      map[string]interface{} `json:"metadata,omitempty"`               // Metadata bổ sung
}

// AIWorkflowCommandClaimInput dữ liệu đầu vào khi claim pending commands
type AIWorkflowCommandClaimInput struct {
	AgentID string `json:"agentId" validate:"required"` // ID của agent đang claim commands (bắt buộc)
	Limit   int    `json:"limit,omitempty" validate:"omitempty,min=1,max=100" transform:"int,default=1"` // Số lượng commands tối đa muốn claim (mặc định: 1, tối đa: 100)
}

// AIWorkflowCommandHeartbeatInput dữ liệu đầu vào khi update heartbeat/progress
type AIWorkflowCommandHeartbeatInput struct {
	CommandID string                 `json:"commandId,omitempty" transform:"str_objectid_ptr,optional"` // ID của command (dạng string ObjectID) - có thể từ URL params hoặc body
	Progress  map[string]interface{} `json:"progress,omitempty"`                                           // Tiến độ chi tiết (ví dụ: {"step": "generating", "percentage": 50, "message": "Đang tạo nội dung..."})
}

// UpdateHeartbeatParams và ReleaseStuckCommandsQuery được định nghĩa trong dto.agent.command.go
// để dùng chung cho cả AIWorkflowCommandHandler và AgentCommandHandler
