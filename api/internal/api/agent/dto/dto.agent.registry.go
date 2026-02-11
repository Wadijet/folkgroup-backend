package agentdto

// AgentRegistryCreateInput là input để tạo agent registry
type AgentRegistryCreateInput struct {
	AgentID     string   `json:"agentId" validate:"required"`
	Name        string   `json:"name,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Description string   `json:"description,omitempty"`
	BotVersion  string   `json:"botVersion,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Color       string   `json:"color,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// AgentRegistryUpdateInput là input để cập nhật agent registry
type AgentRegistryUpdateInput struct {
	Name         *string                  `json:"name,omitempty"`
	DisplayName  *string                  `json:"displayName,omitempty"`
	Description  *string                  `json:"description,omitempty"`
	BotVersion   *string                  `json:"botVersion,omitempty"`
	Icon         *string                  `json:"icon,omitempty"`
	Color        *string                  `json:"color,omitempty"`
	Category     *string                  `json:"category,omitempty"`
	Tags         *[]string                `json:"tags,omitempty"`
	Status       *string                  `json:"status,omitempty"`
	HealthStatus *string                  `json:"healthStatus,omitempty"`
	SystemInfo   map[string]interface{}   `json:"systemInfo,omitempty"`
	Metrics      map[string]interface{}   `json:"metrics,omitempty"`
	JobStatus    []map[string]interface{} `json:"jobStatus,omitempty"`
	ConfigVersion *int64                  `json:"configVersion,omitempty"`
	ConfigHash   *string                  `json:"configHash,omitempty"`
}
