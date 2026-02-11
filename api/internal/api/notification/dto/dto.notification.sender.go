package notifdto

// NotificationChannelSenderCreateInput dùng cho tạo notification sender (tầng transport)
type NotificationChannelSenderCreateInput struct {
	ChannelType   string `json:"channelType" validate:"required"`
	Name          string `json:"name" validate:"required"`
	IsActive      bool   `json:"isActive"`
	SMTPHost      string `json:"smtpHost,omitempty"`
	SMTPPort      int    `json:"smtpPort,omitempty"`
	SMTPUsername  string `json:"smtpUsername,omitempty"`
	SMTPPassword  string `json:"smtpPassword,omitempty"`
	FromEmail     string `json:"fromEmail,omitempty"`
	FromName      string `json:"fromName,omitempty"`
	BotToken      string `json:"botToken,omitempty"`
	BotUsername   string `json:"botUsername,omitempty"`
}

// NotificationChannelSenderUpdateInput dùng cho cập nhật notification sender (tầng transport)
type NotificationChannelSenderUpdateInput struct {
	ChannelType   string `json:"channelType"`
	Name          string `json:"name"`
	IsActive      *bool  `json:"isActive"`
	SMTPHost      string `json:"smtpHost,omitempty"`
	SMTPPort      *int   `json:"smtpPort,omitempty"`
	SMTPUsername  string `json:"smtpUsername,omitempty"`
	SMTPPassword  string `json:"smtpPassword,omitempty"`
	FromEmail     string `json:"fromEmail,omitempty"`
	FromName      string `json:"fromName,omitempty"`
	BotToken      string `json:"botToken,omitempty"`
	BotUsername   string `json:"botUsername,omitempty"`
}
