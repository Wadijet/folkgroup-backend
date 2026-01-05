package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationChannel - Cấu hình kênh nhận (recipients) cho team
type NotificationChannel struct {
	_Relationships struct{}            `relationship:"collection:delivery_queue,field:channelId,message:Không thể xóa channel vì có %d notification đang trong queue. Vui lòng xử lý hoặc xóa các notification trước.|collection:delivery_history,field:channelId,message:Không thể xóa channel vì có %d notification trong lịch sử. Vui lòng xóa lịch sử trước."` // Relationship definitions - không export, chỉ dùng cho tag parsing
	ID                 primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:channel_name_org_unique"` // Tổ chức sở hữu dữ liệu (phân quyền) - Team ID
	ChannelType    string               `json:"channelType" bson:"channelType" index:"single:1,compound:channel_name_org_unique"`        // email, telegram, webhook
	Name           string               `json:"name" bson:"name" index:"single:1,compound:channel_name_org_unique"` // Tên channel phải unique trong 1 organization và channelType
	Description    string               `json:"description,omitempty" bson:"description,omitempty"`                  // Mô tả về channel để người dùng hiểu được mục đích sử dụng
	IsActive       bool                 `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem       bool                 `json:"-" bson:"isSystem" index:"single:1"`              // true = dữ liệu hệ thống, không thể xóa (chỉ dùng nội bộ, không expose ra API)

	// Sender configs (dự phòng - thứ tự ưu tiên)
	SenderIDs []primitive.ObjectID `json:"senderIds,omitempty" bson:"senderIds,omitempty"` // Mảng sender IDs (thứ tự ưu tiên), null/empty = dùng inheritance

	// Recipients (kênh nhận)
	// Email recipients
	Recipients []string `json:"recipients,omitempty" bson:"recipients,omitempty"` // Email addresses

	// Telegram recipients
	ChatIDs []string `json:"chatIds,omitempty" bson:"chatIds,omitempty"` // Telegram chat IDs

	// Webhook recipients
	WebhookURL     string            `json:"webhookUrl,omitempty" bson:"webhookUrl,omitempty"`         // Webhook URL (chỉ 1 URL)
	WebhookHeaders map[string]string `json:"webhookHeaders,omitempty" bson:"webhookHeaders,omitempty"` // Webhook headers

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}

