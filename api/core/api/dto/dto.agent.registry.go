package dto

// AgentRegistryCreateInput là input để tạo agent registry
// Lưu ý: AgentRegistry thường được tạo tự động khi bot check-in lần đầu
type AgentRegistryCreateInput struct {
	AgentID     string `json:"agentId" validate:"required"`     // ID của agent (từ ENV AGENT_ID)
	Name        string `json:"name,omitempty"`                 // Tên agent (optional)
	Description string `json:"description,omitempty"`          // Mô tả
	BotVersion  string `json:"botVersion,omitempty"`           // Version của bot code
}

// AgentRegistryUpdateInput là input để cập nhật agent registry
// Lưu ý: Đã bao gồm các fields từ AgentStatus sau khi ghép collections
type AgentRegistryUpdateInput struct {
	// Thông tin cơ bản
	Name        *string `json:"name,omitempty"`        // Tên agent
	Description *string `json:"description,omitempty"` // Mô tả
	BotVersion  *string `json:"botVersion,omitempty"`  // Version của bot code

	// Status fields (từ agent_status đã ghép)
	Status       *string                `json:"status,omitempty"`       // "online", "offline", "error", "maintenance"
	HealthStatus *string                `json:"healthStatus,omitempty"` // "healthy", "degraded", "unhealthy"
	SystemInfo   map[string]interface{} `json:"systemInfo,omitempty"`   // OS, Arch, GoVersion, Uptime, CPU, Memory, Disk
	Metrics      map[string]interface{} `json:"metrics,omitempty"`      // Bot-level metrics
	JobStatus    []map[string]interface{} `json:"jobStatus,omitempty"`  // Job statuses
	ConfigVersion *int64 `json:"configVersion,omitempty"`             // Version của config đang dùng (Unix timestamp)
	ConfigHash    *string `json:"configHash,omitempty"`                // Hash của config đang dùng
}
