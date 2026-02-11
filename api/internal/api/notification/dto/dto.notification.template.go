package notifdto

// NotificationTemplateCreateInput dùng cho tạo notification template (tầng transport)
type NotificationTemplateCreateInput struct {
	EventType   string                      `json:"eventType" validate:"required"`
	ChannelType string                      `json:"channelType" validate:"required"`
	Subject     string                      `json:"subject,omitempty"`
	Content     string                      `json:"content" validate:"required"`
	Variables   []string                    `json:"variables,omitempty"`
	CTAs        []NotificationCTACreateInput `json:"ctas,omitempty"`
	IsActive    bool                        `json:"isActive"`
}

// NotificationTemplateUpdateInput dùng cho cập nhật notification template (tầng transport)
type NotificationTemplateUpdateInput struct {
	EventType   string                      `json:"eventType"`
	ChannelType string                      `json:"channelType"`
	Subject     string                      `json:"subject,omitempty"`
	Content     string                      `json:"content"`
	Variables   []string                    `json:"variables,omitempty"`
	CTAs        []NotificationCTACreateInput `json:"ctas,omitempty"`
	IsActive    *bool                       `json:"isActive"`
}

// NotificationCTACreateInput dùng cho CTA button trong template
type NotificationCTACreateInput struct {
	Label  string `json:"label" validate:"required"`
	Action string `json:"action" validate:"required"`
	Style  string `json:"style,omitempty"`
}
