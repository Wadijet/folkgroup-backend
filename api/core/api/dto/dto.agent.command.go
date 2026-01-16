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
	Limit   int    `json:"limit,omitempty" validate:"omitempty,min=1,max=100" transform:"int,default=1"` // Số lượng commands tối đa muốn claim (mặc định: 1, tối đa: 100)
}

// AgentCommandHeartbeatInput dữ liệu đầu vào khi update heartbeat/progress
type AgentCommandHeartbeatInput struct {
	CommandID string                 `json:"commandId,omitempty" transform:"str_objectid_ptr,optional"` // ID của command (dạng string ObjectID) - có thể từ URL params hoặc body
	Progress  map[string]interface{} `json:"progress,omitempty"`                                           // Tiến độ chi tiết (ví dụ: {"step": "stopping", "percentage": 50, "message": "Đang dừng bot..."})
}

// UpdateHeartbeatParams params từ URL khi update heartbeat (nếu có commandId trong URL)
type UpdateHeartbeatParams struct {
	CommandID string `uri:"commandId,omitempty" validate:"omitempty" transform:"str_objectid,optional"` // Command ID từ URL params (tùy chọn) - tự động validate và convert sang ObjectID
}

// ReleaseStuckCommandsQuery query params khi release stuck commands
type ReleaseStuckCommandsQuery struct {
	TimeoutSeconds int64 `query:"timeoutSeconds" validate:"omitempty,min=60"` // Timeout seconds (tối thiểu 60, mặc định 300 - xử lý trong handler)
}
