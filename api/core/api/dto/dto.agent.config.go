package dto

// AgentConfigCreateInput là input để tạo agent config
// Lưu ý: AgentConfig thường được tạo tự động khi bot submit config hoặc admin tạo
// Version được server tự động quyết định bằng Unix timestamp, frontend không cần truyền
type AgentConfigCreateInput struct {
	AgentID     string                 `json:"agentId" validate:"required"`   // ObjectID của agent registry
	ConfigData  map[string]interface{} `json:"configData" validate:"required"` // Config data động
	Description string                 `json:"description,omitempty"`          // Mô tả về config
	ChangeLog   string                 `json:"changeLog,omitempty"`            // Log thay đổi
	// Version: Server tự động gán bằng Unix timestamp, không cần frontend truyền
}

// AgentConfigUpdateInput là input để cập nhật agent config
// Lưu ý: Thường chỉ update description, changeLog, hoặc deactivate config cũ
// Để update configData (tạo version mới), dùng endpoint Upsert với AgentConfigCreateInput
type AgentConfigUpdateInput struct {
	Description *string `json:"description,omitempty"` // Mô tả về config
	ChangeLog   *string `json:"changeLog,omitempty"`   // Log thay đổi
	IsActive    *bool   `json:"isActive,omitempty"`    // Activate/deactivate config
}
