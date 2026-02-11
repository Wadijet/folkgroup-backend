// Package webhookdto chứa DTO cho domain Webhook (log).
// File: dto.webhook.log.go
package webhookdto

// WebhookLogCreateInput là DTO cho tạo mới webhook log
type WebhookLogCreateInput struct {
	Source         string                 `json:"source" validate:"required"`         // "pancake" hoặc "pancake_pos"
	EventType      string                 `json:"eventType" validate:"required"`     // Loại event
	PageID         string                 `json:"pageId,omitempty"`                    // Page ID (cho Pancake)
	ShopID         int64                  `json:"shopId,omitempty"`                  // Shop ID (cho Pancake POS)
	RequestHeaders map[string]string     `json:"requestHeaders,omitempty"`            // Headers của request
	RequestBody    map[string]interface{} `json:"requestBody" validate:"required"`   // Body của request
	RawBody        string                 `json:"rawBody,omitempty"`                  // Raw body string
	IPAddress      string                 `json:"ipAddress,omitempty"`              // IP address
	UserAgent      string                 `json:"userAgent,omitempty"`               // User agent
}

// WebhookLogUpdateInput là DTO cho cập nhật webhook log
type WebhookLogUpdateInput struct {
	Processed    *bool   `json:"processed,omitempty"`    // Đã xử lý thành công chưa
	ProcessError *string `json:"processError,omitempty"` // Lỗi nếu có
}
