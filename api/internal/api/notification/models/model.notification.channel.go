// Package models - NotificationChannel thuộc domain Notification.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationChannel - Cấu hình kênh nhận (recipients) cho team
type NotificationChannel struct {
	_Relationships      struct{}             `relationship:"collection:delivery_queue,field:channelId,message:Không thể xóa channel vì có %d notification đang trong queue. Vui lòng xử lý hoặc xóa các notification trước.|collection:delivery_history,field:channelId,message:Không thể xóa channel vì có %d notification trong lịch sử. Vui lòng xóa lịch sử trước."`
	ID                  primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:channel_name_org_unique"`
	ChannelType         string               `json:"channelType" bson:"channelType" index:"single:1,compound:channel_name_org_unique"`
	Name                string               `json:"name" bson:"name" index:"single:1,compound:channel_name_org_unique"`
	Description         string               `json:"description,omitempty" bson:"description,omitempty"`
	IsActive            bool                 `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem            bool                 `json:"-" bson:"isSystem" index:"single:1"`

	SenderIDs []primitive.ObjectID `json:"senderIds,omitempty" bson:"senderIds,omitempty"`

	Recipients []string `json:"recipients,omitempty" bson:"recipients,omitempty"`
	ChatIDs    []string `json:"chatIds,omitempty" bson:"chatIds,omitempty"`

	WebhookURL     string            `json:"webhookUrl,omitempty" bson:"webhookUrl,omitempty"`
	WebhookHeaders map[string]string `json:"webhookHeaders,omitempty" bson:"webhookHeaders,omitempty"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
