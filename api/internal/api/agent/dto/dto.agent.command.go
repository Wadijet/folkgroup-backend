package agentdto

// AgentCommandCreateInput là input để tạo agent command
type AgentCommandCreateInput struct {
	AgentID string                 `json:"agentId" validate:"required"`
	Type    string                 `json:"type" validate:"required"`
	Target  string                 `json:"target" validate:"required"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// AgentCommandUpdateInput là input để cập nhật agent command
type AgentCommandUpdateInput struct {
	Status *string                 `json:"status,omitempty"`
	Result *map[string]interface{} `json:"result,omitempty"`
	Error  *string                 `json:"error,omitempty"`
}

// AgentCommandClaimInput dữ liệu đầu vào khi claim pending commands
type AgentCommandClaimInput struct {
	AgentID string `json:"agentId" validate:"required"`
	Limit   int    `json:"limit,omitempty" validate:"omitempty,min=1,max=100" transform:"int,default=1"`
}

// AgentCommandHeartbeatInput dữ liệu đầu vào khi update heartbeat/progress
type AgentCommandHeartbeatInput struct {
	CommandID string                 `json:"commandId,omitempty" transform:"str_objectid_ptr,optional"`
	Progress  map[string]interface{} `json:"progress,omitempty"`
}

// UpdateHeartbeatParams params từ URL khi update heartbeat
type UpdateHeartbeatParams struct {
	CommandID string `uri:"commandId,omitempty" validate:"omitempty" transform:"str_objectid,optional"`
}

// ReleaseStuckCommandsQuery query params khi release stuck commands
type ReleaseStuckCommandsQuery struct {
	TimeoutSeconds int64 `query:"timeoutSeconds" validate:"omitempty,min=60"`
}
