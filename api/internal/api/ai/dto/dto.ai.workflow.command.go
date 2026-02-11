package aidto

// AIWorkflowCommandCreateInput dữ liệu đầu vào khi tạo AI workflow command
type AIWorkflowCommandCreateInput struct {
	CommandType string                 `json:"commandType" validate:"required,oneof=START_WORKFLOW EXECUTE_STEP"`
	WorkflowID  string                 `json:"workflowId,omitempty" transform:"str_objectid_ptr,optional"`
	StepID      string                 `json:"stepId,omitempty" transform:"str_objectid_ptr,optional"`
	RootRefID   string                 `json:"rootRefId,omitempty" transform:"str_objectid_ptr,optional"`
	RootRefType string                 `json:"rootRefType,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AIWorkflowCommandUpdateInput dữ liệu đầu vào khi cập nhật AI workflow command
type AIWorkflowCommandUpdateInput struct {
	Status        string                 `json:"status,omitempty"`
	AgentID       string                 `json:"agentId,omitempty"`
	WorkflowRunID string                 `json:"workflowRunId,omitempty" transform:"str_objectid_ptr,optional"`
	StepRunID     string                 `json:"stepRunId,omitempty" transform:"str_objectid_ptr,optional"`
	Result        map[string]interface{} `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AIWorkflowCommandClaimInput dữ liệu đầu vào khi claim pending commands
type AIWorkflowCommandClaimInput struct {
	AgentID string `json:"agentId" validate:"required"`
	Limit   int    `json:"limit,omitempty" validate:"omitempty,min=1,max=100" transform:"int,default=1"`
}

// AIWorkflowCommandHeartbeatInput dữ liệu đầu vào khi update heartbeat/progress
type AIWorkflowCommandHeartbeatInput struct {
	CommandID string                 `json:"commandId,omitempty" transform:"str_objectid_ptr,optional"`
	Progress  map[string]interface{} `json:"progress,omitempty"`
}
