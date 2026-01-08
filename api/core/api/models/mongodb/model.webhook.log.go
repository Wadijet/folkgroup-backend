package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WebhookLog lưu log của tất cả webhooks nhận được để debug
type WebhookLog struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"` // ID của log

	// ===== SOURCE INFO =====
	Source      string `json:"source" bson:"source" index:"single:1"`           // Nguồn webhook: "pancake" hoặc "pancake_pos"
	EventType   string `json:"eventType" bson:"eventType" index:"single:1"`     // Loại event: order_created, conversation_updated, etc.
	PageID      string `json:"pageId,omitempty" bson:"pageId,omitempty" index:"text"` // Page ID (cho Pancake)
	ShopID      int64  `json:"shopId,omitempty" bson:"shopId,omitempty" index:"single:1"` // Shop ID (cho Pancake POS)

	// ===== REQUEST INFO =====
	RequestHeaders map[string]string      `json:"requestHeaders,omitempty" bson:"requestHeaders,omitempty"` // Headers của request
	RequestBody     map[string]interface{} `json:"requestBody" bson:"requestBody"`                          // Body của request (toàn bộ payload)
	RawBody         string                 `json:"rawBody,omitempty" bson:"rawBody,omitempty"`              // Raw body string (để debug)

	// ===== PROCESSING INFO =====
	Processed   bool   `json:"processed" bson:"processed" index:"single:1"`     // Đã xử lý thành công chưa
	ProcessError string `json:"processError,omitempty" bson:"processError,omitempty"` // Lỗi nếu có trong quá trình xử lý
	ProcessedAt  int64  `json:"processedAt,omitempty" bson:"processedAt,omitempty"`   // Thời gian xử lý

	// ===== METADATA =====
	IPAddress string `json:"ipAddress,omitempty" bson:"ipAddress,omitempty"` // IP address của request
	UserAgent string `json:"userAgent,omitempty" bson:"userAgent,omitempty"` // User agent của request

	// ===== TIMESTAMPS =====
	ReceivedAt int64 `json:"receivedAt" bson:"receivedAt" index:"single:-1"` // Thời gian nhận webhook (Unix timestamp milliseconds)
	CreatedAt  int64 `json:"createdAt" bson:"createdAt"`                      // Thời gian tạo log
	UpdatedAt  int64 `json:"updatedAt" bson:"updatedAt"`                      // Thời gian cập nhật log
}
