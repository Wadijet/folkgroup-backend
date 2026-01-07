package dto

// AgentCommandCreateInput là input để tạo agent command
type AgentCommandCreateInput struct {
	AgentID string                 `json:"agentId" validate:"required"` // ObjectID của agent registry
	Type    string                 `json:"type" validate:"required"`    // "stop", "start", "restart", "reload_config", "shutdown", "run_job", "pause_job", "resume_job", "disable_job", "enable_job", "update_job_schedule"
	Target  string                 `json:"target" validate:"required"`  // "bot" hoặc job name
	Params  map[string]interface{} `json:"params,omitempty"`            // Tham số cho command
}

// AgentCommandUpdateInput là input để cập nhật agent command
// Lưu ý: Thường chỉ update status, result, error khi bot báo cáo kết quả
type AgentCommandUpdateInput struct {
	Status *string                 `json:"status,omitempty"` // "pending", "executing", "completed", "failed", "cancelled"
	Result *map[string]interface{} `json:"result,omitempty"` // Kết quả thực thi
	Error  *string                 `json:"error,omitempty"`  // Lỗi nếu có
}
