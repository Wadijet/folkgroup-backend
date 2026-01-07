package dto

// AgentActivityLogCreateInput là input để tạo agent activity log
// Lưu ý: AgentActivityLog thường được tạo tự động khi bot log activity
// Không nên tạo thủ công, để bot tự log
type AgentActivityLogCreateInput struct {
	AgentID string `json:"agentId" validate:"required"` // ObjectID của agent registry
}

// AgentActivityLogUpdateInput là input để cập nhật agent activity log
// Lưu ý: Activity log thường không được update sau khi tạo
type AgentActivityLogUpdateInput struct {
	Message *string `json:"message,omitempty"` // Thêm message nếu cần
}
