package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/utility/identity"
)

// FbMessage đại diện cho metadata của conversation (không lưu messages[])
// Messages được lưu riêng trong collection fb_message_items
type FbMessage struct {
	ID             primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                                // ID của document
	// ===== IDENTITY 4 LỚP (enrich trong FbMessageService.UpsertMessages) =====
	Uid             string                       `json:"uid" bson:"uid" index:"single:1"`
	SourceIds       map[string]string            `json:"sourceIds,omitempty" bson:"sourceIds,omitempty"`
	SourceIdsFb     string                       `json:"-" bson:"sourceIds.facebook,omitempty" index:"single:1,sparse"`
	Links           map[string]identity.LinkItem `json:"links,omitempty" bson:"links,omitempty"`
	LinksCustomerUid   string                    `json:"-" bson:"links.customer.uid,omitempty" index:"single:1,sparse"`
	LinksConversationUid string                  `json:"-" bson:"links.conversation.uid,omitempty" index:"single:1,sparse"`
	PageId         string                 `json:"pageId" bson:"pageId" index:"text"`                                                                // ID của trang
	PageUsername   string                 `json:"pageUsername" bson:"pageUsername" index:"text"`                                                    // Tên người dùng của trang
	ConversationId string                 `json:"conversationId" bson:"conversationId" index:"unique;text" extract:"PanCakeData\\.conversation_id"` // ID của cuộc hội thoại (extract từ PanCakeData["conversation_id"])
	CustomerId     string                 `json:"customerId" bson:"customerId" index:"text"`                                                        // ID của khách hàng
	PanCakeData    map[string]interface{} `json:"panCakeData" bson:"panCakeData"`                                                                   // Dữ liệu API (KHÔNG có messages[], messages được lưu riêng trong fb_message_items)
	LastSyncedAt   int64                  `json:"lastSyncedAt" bson:"lastSyncedAt"`                                                                 // Thời gian sync cuối cùng
	TotalMessages  int64                  `json:"totalMessages" bson:"totalMessages"`                                                               // Tổng số messages trong fb_message_items
	HasMore        bool                   `json:"hasMore" bson:"hasMore"`                                                                           // Còn messages để sync không

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo document
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật document
}
