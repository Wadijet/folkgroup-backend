// Package conversationmodels — Lớp canonical chỉ cho cuộc hội thoại (thread) đa nguồn: 1:1 mỗi bản ghi mirror nguồn → một document.
//
// Tin nhắn không đồng bộ sang collection riêng trong domain này: không có model “message canonical” ở đây.
// Đọc transcript / từng tin trực tiếp từ collection nguồn (vd. fb_message_items) nhờ tham chiếu trên mirror (conversationId, pageId, …) hoặc links/sourceIds.
package conversationmodels

import (
	"go.mongodb.org/mongo-driver/bson/primitive"

	"meta_commerce/internal/utility/identity"
)

// SourceFacebookMessenger — mirror hiện tại: fb_conversations (Messenger / Pancake).
const SourceFacebookMessenger = "facebook_messenger"

// MessagingConversation — một thread chuẩn trong hệ; chỉ đồng bộ metadata hội thoại 1:1 qua sourceRecordMongoId (vd. _id fb_conversations).
// Nội dung tin nhắn không lưu ở document này; tra cứu tin tại collection nguồn tương ứng.
type MessagingConversation struct {
	ID                  primitive.ObjectID           `json:"id,omitempty" bson:"_id,omitempty"`
	Uid                 string                       `json:"uid" bson:"uid" index:"compound:idx_messaging_uid_org"`
	Source              string                       `json:"source" bson:"source" index:"compound:idx_messaging_source_ref,compound:idx_messaging_conv_org_source"`
	SourceIds           map[string]string            `json:"sourceIds,omitempty" bson:"sourceIds,omitempty"`
	Links               map[string]identity.LinkItem `json:"links,omitempty" bson:"links,omitempty"`
	OwnerOrganizationID primitive.ObjectID           `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"compound:idx_messaging_uid_org,compound:idx_messaging_source_ref,compound:idx_messaging_conv_org_source"`

	// SourceRecordMongoID — _id của bản ghi mirror (vd. fb_conversations).
	SourceRecordMongoID primitive.ObjectID `json:"sourceRecordMongoId" bson:"sourceRecordMongoId" index:"compound:idx_messaging_source_ref"`

	// Trường denormalized phục vụ tra cứu / CIX (copy từ FbConversation).
	ConversationId    string                 `json:"conversationId" bson:"conversationId" index:"compound:idx_messaging_conv_org_source"`
	CustomerId        string                 `json:"customerId,omitempty" bson:"customerId,omitempty"`
	PageId            string                 `json:"pageId,omitempty" bson:"pageId,omitempty"`
	PageUsername      string                 `json:"pageUsername,omitempty" bson:"pageUsername,omitempty"`
	PanCakeData       map[string]interface{} `json:"panCakeData,omitempty" bson:"panCakeData,omitempty"`
	PanCakeUpdatedAt  int64                  `json:"panCakeUpdatedAt,omitempty" bson:"panCakeUpdatedAt,omitempty"`
	NeedsPrioritySync bool                   `json:"needsPrioritySync,omitempty" bson:"needsPrioritySync,omitempty"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}
