package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationQueueItem - Queue item để xử lý
// Delivery Service chỉ cần: sender, recipient, content đã render
// Option C (Hybrid): SenderID (fallback) + SenderConfig (optional, encrypted) - fast path
type NotificationQueueItem struct {
	ID                  primitive.ObjectID     `json:"id,omitempty" bson:"_id,omitempty"`
	EventType           string                 `json:"eventType" bson:"eventType" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID     `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	SenderID            primitive.ObjectID     `json:"senderId" bson:"senderId" index:"single:1"`            // Sender ID để fallback query nếu không có SenderConfig
	SenderConfig        string                 `json:"senderConfig,omitempty" bson:"senderConfig,omitempty"` // Sender config đã encrypt (optional, fast path)
	ChannelType         string                 `json:"channelType" bson:"channelType" index:"single:1"`
	Recipient           string                 `json:"recipient" bson:"recipient"`
	Subject             string                 `json:"subject,omitempty" bson:"subject,omitempty"`
	Content             string                 `json:"content,omitempty" bson:"content,omitempty"`
	CTAs                []string               `json:"ctas,omitempty" bson:"ctas,omitempty"` // CTAs đã render sẵn (có tracking URLs)
	Payload             map[string]interface{} `json:"payload" bson:"payload"`

	Status      string `json:"status" bson:"status" index:"single:1"` // pending, processing, completed, failed
	RetryCount  int    `json:"retryCount" bson:"retryCount"`
	MaxRetries  int    `json:"maxRetries" bson:"maxRetries"` // Mặc định: 3
	NextRetryAt *int64 `json:"nextRetryAt,omitempty" bson:"nextRetryAt,omitempty" index:"single:1"`

	Error     string `json:"error,omitempty" bson:"error,omitempty"`
	CreatedAt int64  `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64  `json:"updatedAt" bson:"updatedAt"`
}
