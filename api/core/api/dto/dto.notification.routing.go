package dto

// NotificationRoutingRuleCreateInput dùng cho tạo notification routing rule (tầng transport)
// Đây là contract/interface cho Frontend - định nghĩa cấu trúc dữ liệu cần gửi khi tạo routing rule
// Lưu ý: Backend parse trực tiếp vào Model, nhưng DTO này dùng để Frontend biết cấu trúc cần gửi
// Có thể routing theo EventType (cụ thể) hoặc Domain (tổng quát) - ít nhất một trong hai phải có
type NotificationRoutingRuleCreateInput struct {
	EventType       *string  `json:"eventType,omitempty"`       // Loại event cụ thể (optional): conversation_unreplied, order_created, ... - Optional (nếu có Domain thì không cần)
	Domain          *string  `json:"domain,omitempty"`           // Domain tổng quát (optional): system, conversation, order, ... - Optional (nếu có EventType thì không cần)
	OrganizationIDs []string `json:"organizationIds" validate:"required"` // Teams nào nhận (có thể nhiều) - BẮT BUỘC - Array of organization IDs
	ChannelTypes    []string `json:"channelTypes,omitempty"`     // Filter channels theo type (optional): ["email", "telegram", "webhook"] - Optional
	Severities      []string `json:"severities,omitempty"`       // Filter theo severity (optional): ["critical", "high"] - chỉ nhận các severity này - Optional
	IsActive        bool     `json:"isActive"`                   // Rule có đang hoạt động không - Optional (default: true)

	// Lưu ý: KHÔNG cần gửi isSystem - Backend tự động set (chỉ dùng nội bộ)
	// Lưu ý: Phải có ít nhất EventType hoặc Domain (một trong hai)
}

// NotificationRoutingRuleUpdateInput dùng cho cập nhật notification routing rule (tầng transport)
// Đây là contract/interface cho Frontend - định nghĩa cấu trúc dữ liệu cần gửi khi cập nhật routing rule
// Lưu ý: Backend parse trực tiếp vào Model, nhưng DTO này dùng để Frontend biết cấu trúc cần gửi
type NotificationRoutingRuleUpdateInput struct {
	EventType       *string  `json:"eventType,omitempty"`       // Loại event cụ thể - Optional
	Domain          *string  `json:"domain,omitempty"`           // Domain tổng quát - Optional
	OrganizationIDs []string `json:"organizationIds,omitempty"`  // Teams nào nhận - Optional
	ChannelTypes    []string `json:"channelTypes,omitempty"`    // Filter channels theo type - Optional
	Severities      []string `json:"severities,omitempty"`       // Filter theo severity - Optional
	IsActive        *bool    `json:"isActive,omitempty"`         // Rule có đang hoạt động không - Optional

	// Lưu ý: KHÔNG thể update isSystem - Backend sẽ tự động xóa field này nếu có trong request (bảo mật)
}
