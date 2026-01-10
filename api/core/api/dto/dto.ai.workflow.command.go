package dto

// AIWorkflowCommandCreateInput dữ liệu đầu vào khi tạo AI workflow command
type AIWorkflowCommandCreateInput struct {
	CommandType string                 `json:"commandType" validate:"required"` // Loại command: START_WORKFLOW
	WorkflowID  string                 `json:"workflowId" validate:"required"`  // ID của workflow definition (dạng string ObjectID)
	RootRefID   string                 `json:"rootRefId,omitempty"`            // ID của root content (dạng string ObjectID)
	RootRefType string                 `json:"rootRefType,omitempty"`         // Loại root reference: "layer", "stp", etc.
	Params      map[string]interface{} `json:"params,omitempty"`                 // Tham số bổ sung cho command
	Metadata    map[string]interface{} `json:"metadata,omitempty"`             // Metadata bổ sung
}

// AIWorkflowCommandUpdateInput dữ liệu đầu vào khi cập nhật AI workflow command
type AIWorkflowCommandUpdateInput struct {
	Status        string                 `json:"status,omitempty"`                 // Trạng thái: pending, executing, completed, failed, cancelled
	AgentID       string                 `json:"agentId,omitempty"`                // ID của agent đang xử lý command
	WorkflowRunID string                 `json:"workflowRunId,omitempty"`          // ID của workflow run được tạo (dạng string ObjectID)
	Result        map[string]interface{} `json:"result,omitempty"`                 // Kết quả thực thi command
	Error         string                 `json:"error,omitempty"`                  // Lỗi nếu có
	Metadata      map[string]interface{} `json:"metadata,omitempty"`               // Metadata bổ sung
}
