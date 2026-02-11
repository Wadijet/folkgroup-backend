package notifdto

// NotificationChannelCreateInput dùng cho tạo notification channel (tầng transport)
type NotificationChannelCreateInput struct {
	ChannelType string   `json:"channelType" validate:"required"`
	Name        string   `json:"name" validate:"required"`
	IsActive    bool     `json:"isActive"`
	SenderIDs   []string `json:"senderIds,omitempty"`
	Recipients  []string `json:"recipients,omitempty"`
	ChatIDs     []string `json:"chatIds,omitempty"`
	WebhookURL  string   `json:"webhookUrl,omitempty"`
	WebhookHeaders map[string]string `json:"webhookHeaders,omitempty"`
}

// NotificationChannelUpdateInput dùng cho cập nhật notification channel (tầng transport)
type NotificationChannelUpdateInput struct {
	ChannelType    string            `json:"channelType"`
	Name           string            `json:"name"`
	IsActive       *bool             `json:"isActive"`
	SenderIDs      []string          `json:"senderIds,omitempty"`
	Recipients     []string          `json:"recipients,omitempty"`
	ChatIDs        []string          `json:"chatIds,omitempty"`
	WebhookURL     string            `json:"webhookUrl,omitempty"`
	WebhookHeaders map[string]string `json:"webhookHeaders,omitempty"`
}
