package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FbMessageItem đại diện cho một message riêng lẻ trong collection fb_message_items
// Mỗi message là 1 document riêng để tránh document quá lớn
type FbMessageItem struct {
	ID             primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                                                                       // ID của document
	ConversationId string                 `json:"conversationId" bson:"conversationId" index:"text"`                                                                                       // ID của cuộc hội thoại (không unique, nhiều messages cùng conversationId)
	MessageId      string                 `json:"messageId" bson:"messageId" index:"unique;text" extract:"MessageData\\.id"`                                                               // ID của message từ Pancake (unique, extract từ MessageData["id"])
	MessageData    map[string]interface{} `json:"messageData" bson:"messageData"`                                                                                                          // Toàn bộ dữ liệu của message
	InsertedAt     int64                  `json:"insertedAt" bson:"insertedAt" index:"text" extract:"MessageData\\.inserted_at,converter=time,format=2006-01-02T15:04:05.000000,optional"` // Thời gian insert message (extract từ MessageData["inserted_at"])

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo document
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật document
}
