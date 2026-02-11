// Package models - OrganizationShare thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationShare share dữ liệu giữa các organizations.
type OrganizationShare struct {
	ID                  primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	ToOrgIDs            []primitive.ObjectID `json:"toOrgIds,omitempty" bson:"toOrgIds"`
	PermissionNames     []string             `json:"permissionNames,omitempty" bson:"permissionNames,omitempty"`
	Description         string               `json:"description,omitempty" bson:"description,omitempty"`
	CreatedAt           int64                `json:"createdAt" bson:"createdAt"`
	CreatedBy           primitive.ObjectID   `json:"createdBy" bson:"createdBy"`
}
