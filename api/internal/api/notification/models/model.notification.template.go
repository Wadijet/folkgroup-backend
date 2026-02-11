// Package models - NotificationTemplate thuộc domain Notification.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationTemplate - Template thông báo
type NotificationTemplate struct {
	ID                  primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID *primitive.ObjectID `json:"ownerOrganizationId,omitempty" bson:"ownerOrganizationId,omitempty" index:"single:1"`
	EventType           string              `json:"eventType" bson:"eventType" index:"single:1"`
	ChannelType         string              `json:"channelType" bson:"channelType" index:"single:1"`
	Description         string              `json:"description,omitempty" bson:"description,omitempty"`
	Subject             string              `json:"subject,omitempty" bson:"subject,omitempty"`
	Content             string              `json:"content" bson:"content"`
	Variables           []string            `json:"variables" bson:"variables"`
	CTACodes            []string            `json:"ctaCodes,omitempty" bson:"ctaCodes,omitempty"`
	CTAs                []NotificationCTA   `json:"ctas,omitempty" bson:"ctas,omitempty"`
	IsActive            bool                `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem            bool                `json:"-" bson:"isSystem" index:"single:1"`
	CreatedAt           int64               `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64               `json:"updatedAt" bson:"updatedAt"`
}

// NotificationCTA - CTA button (deprecated, dùng CTACodes thay thế)
type NotificationCTA struct {
	Label  string `json:"label" bson:"label"`
	Action string `json:"action" bson:"action"`
	Style  string `json:"style,omitempty" bson:"style,omitempty"`
}
