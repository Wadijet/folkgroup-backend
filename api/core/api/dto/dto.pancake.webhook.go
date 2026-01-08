package dto

// PancakeWebhookPayload là payload nhận được từ Pancake webhook
// Pancake có thể gửi webhook về các events như: conversation_updated, message_received, order_created, etc.
type PancakeWebhookPayload struct {
	EventType string                 `json:"eventType"` // Loại event: conversation_updated, message_received, order_created, etc.
	PageID    string                 `json:"pageId"`    // ID của page
	Data      map[string]interface{} `json:"data"`      // Dữ liệu chi tiết của event
	Timestamp int64                  `json:"timestamp"` // Thời gian event xảy ra (Unix timestamp)
}

// PancakeWebhookRequest là request body từ Pancake webhook
type PancakeWebhookRequest struct {
	Payload PancakeWebhookPayload `json:"payload" validate:"required"`
	// Có thể có thêm signature hoặc token để verify
	Signature string `json:"signature,omitempty"` // Signature để verify webhook (nếu có)
}
