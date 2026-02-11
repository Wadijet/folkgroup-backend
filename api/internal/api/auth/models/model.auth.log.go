// Package models - AuthLog thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AuthLog lưu log các hành động trong nhóm chức năng AUTH.
type AuthLog struct {
	ID                    primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID                primitive.ObjectID `json:"userId,omitempty" bson:"userId,omitempty"`
	RoleID                primitive.ObjectID `json:"roleId,omitempty" bson:"roleId,omitempty"`
	OwnerOrganizationID   primitive.ObjectID `json:"ownerOrganizationId,omitempty" bson:"ownerOrganizationId,omitempty"`
	Collection            string             `json:"collection,omitempty" bson:"collection,omitempty"`
	Action                string             `json:"action,omitempty" bson:"action,omitempty"`
	Describe              string             `json:"describe,omitempty" bson:"describe,omitempty"`
	OldData               string             `json:"oldData,omitempty" bson:"oldData,omitempty"`
	NewData               string             `json:"newData,omitempty" bson:"newData,omitempty"`
	CreatedAt             int64              `json:"createdAt,omitempty" bson:"createdAt,omitempty"`
	UpdatedAt             int64              `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
}
