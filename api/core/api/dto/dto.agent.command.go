package dto

// AgentCommandCreateInput là input để tạo agent command
type AgentCommandCreateInput struct {
	AgentID string                 `json:"agentId" validate:"required"` // AgentID (string) - id chung giữa các collection, tương ứng với AgentRegistry.AgentID
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

// AgentCommandClaimInput dữ liệu đầu vào khi claim pending commands
type AgentCommandClaimInput struct {
	AgentID string `json:"agentId" validate:"required"` // ID của agent đang claim commands (bắt buộc)
	Limit   int    `json:"limit,omitempty"`             // Số lượng commands tối đa muốn claim (mặc định: 1, tối đa: 100)
}

// AgentCommandHeartbeatInput dữ liệu đầu vào khi update heartbeat/progress
type AgentCommandHeartbeatInput struct {
	CommandID string                 `json:"commandId" validate:"required" transform:"str_objectid_ptr"` // ID của command (dạng string ObjectID)
	Progress  map[string]interface{} `json:"progress,omitempty"`                                           // Tiến độ chi tiết (ví dụ: {"step": "stopping", "percentage": 50, "message": "Đang dừng bot..."})
}
