package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationChannelSender - Cấu hình sender (địa chỉ gửi)
type NotificationChannelSender struct {
	ID                 primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID *primitive.ObjectID `json:"ownerOrganizationId,omitempty" bson:"ownerOrganizationId,omitempty" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền) - null = System Organization
	ChannelType    string              `json:"channelType" bson:"channelType" index:"single:1"`                           // email, telegram, webhook
	Name           string              `json:"name" bson:"name" index:"single:1"`
	Description    string              `json:"description,omitempty" bson:"description,omitempty"`                         // Mô tả về sender để người dùng hiểu được mục đích sử dụng
	IsActive       bool                `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem       bool                `json:"-" bson:"isSystem" index:"single:1"` // true = dữ liệu hệ thống, không thể xóa (chỉ dùng nội bộ, không expose ra API)

	// Email sender config
	SMTPHost     string `json:"smtpHost,omitempty" bson:"smtpHost,omitempty"`
	SMTPPort     int    `json:"smtpPort,omitempty" bson:"smtpPort,omitempty"`
	SMTPUsername string `json:"smtpUsername,omitempty" bson:"smtpUsername,omitempty"`
	SMTPPassword string `json:"smtpPassword,omitempty" bson:"smtpPassword,omitempty"`
	FromEmail    string `json:"fromEmail,omitempty" bson:"fromEmail,omitempty"`
	FromName     string `json:"fromName,omitempty" bson:"fromName,omitempty"`

	// Telegram sender config
	BotToken    string `json:"botToken,omitempty" bson:"botToken,omitempty"`
	BotUsername string `json:"botUsername,omitempty" bson:"botUsername,omitempty"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
