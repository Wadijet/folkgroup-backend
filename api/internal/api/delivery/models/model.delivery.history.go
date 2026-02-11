// Package models - DeliveryHistory thuộc domain Delivery.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// DeliveryHistory - Lịch sử gửi thông báo (thuộc Delivery System)
type DeliveryHistory struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	QueueItemID         primitive.ObjectID `json:"queueItemId" bson:"queueItemId" index:"single:1"`
	EventType           string             `json:"eventType" bson:"eventType" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	Domain              string             `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1"`   // Domain để reporting (optional, có thể infer từ EventType)
	Severity            string             `json:"severity,omitempty" bson:"severity,omitempty" index:"single:1"` // Severity để reporting (optional, có thể infer từ EventType)
	ChannelType         string             `json:"channelType" bson:"channelType" index:"single:1"`
	Recipient           string             `json:"recipient" bson:"recipient"`
	Status              string             `json:"status" bson:"status" index:"single:1"` // sent, failed
	Content             string             `json:"content" bson:"content"`               // Content đã render
	Error               string             `json:"error,omitempty" bson:"error,omitempty"`
	RetryCount          int                `json:"retryCount" bson:"retryCount"`
	SentAt              *int64             `json:"sentAt,omitempty" bson:"sentAt,omitempty"`

	// Open Tracking (Email only)
	OpenedAt  *int64 `json:"openedAt,omitempty" bson:"openedAt,omitempty"` // Thời gian mở email đầu tiên
	OpenCount int    `json:"openCount" bson:"openCount"`                   // Số lần mở email

	// Click Tracking (Tổng)
	ClickedAt   *int64 `json:"clickedAt,omitempty" bson:"clickedAt,omitempty"`     // Thời gian click đầu tiên (bất kỳ CTA nào)
	ClickCount  int    `json:"clickCount" bson:"clickCount"`                       // Tổng số lần click (tất cả CTAs)
	LastClickAt *int64 `json:"lastClickAt,omitempty" bson:"lastClickAt,omitempty"` // Thời gian click cuối cùng

	// CTA Click Tracking (Riêng từng CTA)
	CTAClicks []CTAClick `json:"ctaClicks,omitempty" bson:"ctaClicks,omitempty"` // Tracking clicks cho từng CTA

	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
}

// CTAClick - Tracking click cho từng CTA
type CTAClick struct {
	CTAIndex     int    `json:"ctaIndex" bson:"ctaIndex"`                 // Index của CTA (0, 1, 2, ...)
	Label        string `json:"label" bson:"label"`                       // Label của CTA
	ClickCount   int    `json:"clickCount" bson:"clickCount"`             // Số lần click vào CTA này
	FirstClickAt *int64 `json:"firstClickAt,omitempty" bson:"firstClickAt,omitempty"` // Thời gian click đầu tiên
	LastClickAt  *int64 `json:"lastClickAt,omitempty" bson:"lastClickAt,omitempty"`   // Thời gian click cuối cùng
}
