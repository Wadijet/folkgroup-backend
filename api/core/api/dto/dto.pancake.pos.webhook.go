package dto

// PancakePosWebhookPayload là payload nhận được từ Pancake POS webhook
// Pancake POS có thể gửi webhook về các events như: order_created, order_updated, product_updated, customer_created, etc.
type PancakePosWebhookPayload struct {
	EventType string                 `json:"eventType"` // Loại event: order_created, order_updated, product_updated, customer_created, etc.
	ShopID    int                    `json:"shopId"`    // ID của shop (integer)
	Data      map[string]interface{} `json:"data"`      // Dữ liệu chi tiết của event
	Timestamp int64                  `json:"timestamp"` // Thời gian event xảy ra (Unix timestamp)
}

// PancakePosWebhookRequest là request body từ Pancake POS webhook
type PancakePosWebhookRequest struct {
	Payload PancakePosWebhookPayload `json:"payload" validate:"required"`
	// Có thể có thêm signature hoặc token để verify
	Signature string `json:"signature,omitempty"` // Signature để verify webhook (nếu có)
}
