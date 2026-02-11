// Package models - NotificationRoutingRule thuộc domain Notification.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationRoutingRule - Routing rule: Event nào → Teams nào → Channels nào
type NotificationRoutingRule struct {
	ID                  primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:eventType_ownerOrg_unique"`
	EventType           string               `json:"eventType" bson:"eventType" index:"single:1,compound:eventType_ownerOrg_unique"`
	Domain              *string              `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1,sparse"`
	Description         string               `json:"description,omitempty" bson:"description,omitempty"`
	OrganizationIDs     []primitive.ObjectID `json:"organizationIds" bson:"organizationIds"`
	ChannelTypes        []string             `json:"channelTypes,omitempty" bson:"channelTypes,omitempty"`
	Severities          []string             `json:"severities,omitempty" bson:"severities,omitempty"`
	IsActive            bool                 `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem            bool                 `json:"-" bson:"isSystem" index:"single:1"`
	CreatedAt           int64                `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64                `json:"updatedAt" bson:"updatedAt"`
}
