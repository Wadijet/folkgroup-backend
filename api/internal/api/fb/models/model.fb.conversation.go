package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Permission đại diện cho quyền trong hệ thống,
// Các quyền được kết cấu theo các quyền gọi các API trong router.
// Các quyèn này được tạo ra khi khởi tạo hệ thống và không thể thay đổi.
type FbConversation struct {
	ID               primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`                                                                                                     // ID của quyền
	PageId           string                 `json:"pageId" bson:"pageId" index:"text"`                                                                                                     // ID của trang
	PageUsername     string                 `json:"pageUsername" bson:"pageUsername" index:"text"`                                                                                         // Tên người dùng của trang
	ConversationId   string                 `json:"conversationId" bson:"conversationId" index:"unique;text" extract:"PanCakeData\\.id"`                                                   // ID của cuộc hội thoại (extract từ PanCakeData["id"])
	CustomerId       string                 `json:"customerId" bson:"customerId" index:"text" extract:"PanCakeData\\.customer_id,optional"`                                                // ID của khách hàng (extract từ PanCakeData["customer_id"])
	PanCakeData      map[string]interface{} `json:"panCakeData" bson:"panCakeData"`                                                                                                        // Dữ liệu API
	PanCakeUpdatedAt int64                  `json:"panCakeUpdatedAt" bson:"panCakeUpdatedAt" extract:"PanCakeData\\.updated_at,converter=time,format=2006-01-02T15:04:05.000000,optional" index:"compound:idx_backfill_conversations"` // Thời gian cập nhật API (extract từ PanCakeData["updated_at"])

	// ===== SYNC FLAGS =====
	NeedsPrioritySync bool `json:"needsPrioritySync" bson:"needsPrioritySync" index:"single:1"` // Đánh dấu hội thoại này cần ưu tiên đồng bộ lại ngay

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:idx_backfill_conversations"` // Tổ chức sở hữu dữ liệu

	// Index backfill: sort theo thời gian gốc trong panCakeData (dùng cho Find phân trang CRUD)
	panCakeDataInsertedAt int64 `bson:"panCakeData.inserted_at,omitempty" index:"compound:idx_backfill_conversations"`
	panCakeDataUpdatedAt  int64 `bson:"panCakeData.updated_at,omitempty" index:"compound:idx_backfill_conversations"`

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật
}
