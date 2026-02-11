package notifdto

// NotificationRoutingRuleCreateInput dùng cho tạo notification routing rule (tầng transport)
type NotificationRoutingRuleCreateInput struct {
	EventType       string   `json:"eventType" validate:"required"`
	Domain          *string  `json:"domain,omitempty"`
	OrganizationIDs []string `json:"organizationIds" validate:"required"`
	ChannelTypes    []string `json:"channelTypes,omitempty"`
	Severities      []string `json:"severities,omitempty"`
	IsActive        bool     `json:"isActive"`
}

// NotificationRoutingRuleUpdateInput dùng cho cập nhật notification routing rule (tầng transport)
type NotificationRoutingRuleUpdateInput struct {
	EventType       string   `json:"eventType,omitempty"`
	Domain          *string  `json:"domain,omitempty"`
	OrganizationIDs []string `json:"organizationIds,omitempty"`
	ChannelTypes    []string `json:"channelTypes,omitempty"`
	Severities      []string `json:"severities,omitempty"`
	IsActive        *bool    `json:"isActive,omitempty"`
}
