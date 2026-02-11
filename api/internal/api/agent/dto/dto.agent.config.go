package agentdto

// AgentConfigCreateInput là input để tạo agent config
type AgentConfigCreateInput struct {
	AgentID     string                 `json:"agentId" validate:"required"`
	ConfigData  map[string]interface{} `json:"configData" validate:"required"`
	Description string                 `json:"description,omitempty"`
	ChangeLog   string                 `json:"changeLog,omitempty"`
}

// AgentConfigUpdateInput là input để cập nhật agent config
type AgentConfigUpdateInput struct {
	Description *string `json:"description,omitempty"`
	ChangeLog   *string `json:"changeLog,omitempty"`
	IsActive    *bool   `json:"isActive,omitempty"`
}
