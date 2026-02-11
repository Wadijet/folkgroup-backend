// Package deliverydto chứa DTO cho domain Delivery (send, tracking).
// File: dto.delivery.send.go - giữ tên cấu trúc cũ (dto.<domain>.<entity>.go).
package deliverydto

// DeliverySendRequest là request để gửi notification trực tiếp
type DeliverySendRequest struct {
	ChannelType string                 `json:"channelType" validate:"required"`
	Recipient   string                 `json:"recipient" validate:"required"`
	Subject     string                 `json:"subject,omitempty"`
	Content     string                 `json:"content" validate:"required"`
	CTAs        []DeliverySendCTA      `json:"ctas,omitempty"`
	EventType   string                 `json:"eventType,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DeliverySendCTA là CTA đã render
type DeliverySendCTA struct {
	Label       string `json:"label"`
	Action      string `json:"action"`      // URL (có thể đã có tracking URL)
	OriginalURL string `json:"originalUrl"` // Original URL (nếu có)
	Style       string `json:"style,omitempty"`
}

// DeliverySendResponse là response sau khi gửi
type DeliverySendResponse struct {
	MessageID string `json:"messageId"` // History ID
	Status    string `json:"status"`    // queued
	QueuedAt  int64  `json:"queuedAt"`
}
