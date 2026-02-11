package agentdto

// AgentActivityLogCreateInput là input để tạo agent activity log
type AgentActivityLogCreateInput struct {
	AgentID string `json:"agentId" validate:"required" transform:"str_objectid"`
}

// AgentActivityLogUpdateInput là input để cập nhật agent activity log
type AgentActivityLogUpdateInput struct {
	Message *string `json:"message,omitempty"`
}
