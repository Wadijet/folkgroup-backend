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
	PanCakeUpdatedAt int64                  `json:"panCakeUpdatedAt" bson:"panCakeUpdatedAt" extract:"PanCakeData\\.updated_at,converter=time,format=2006-01-02T15:04:05.000000,optional"` // Thời gian cập nhật dữ liệu API (extract từ PanCakeData["updated_at"])

	// ===== SYNC FLAGS =====
	NeedsPrioritySync bool `json:"needsPrioritySync" bson:"needsPrioritySync" index:"single:1"` // Đánh dấu hội thoại này cần ưu tiên đồng bộ lại ngay

	// ===== ORGANIZATION =====
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền)

	CreatedAt int64 `json:"createdAt" bson:"createdAt"` // Thời gian tạo quyền
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"` // Thời gian cập nhật quyền
}
