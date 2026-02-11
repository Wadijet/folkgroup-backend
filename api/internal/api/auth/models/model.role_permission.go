// Package models - RolePermission thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RolePermission quyền vai trò trong hệ thống.
type RolePermission struct {
	ID              primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	RoleID          primitive.ObjectID `json:"roleId" bson:"roleId" index:"single:1"`
	PermissionID    primitive.ObjectID `json:"permissionId" bson:"permissionId" index:"single:1"`
	Scope           byte               `json:"scope" bson:"scope" index:"single:1"`
	CreatedByRoleID primitive.ObjectID `json:"createdByRoleId" bson:"createdByRoleId"`
	CreatedByUserID primitive.ObjectID `json:"createdByUserId" bson:"createdByUserId"`
	CreatedAt       int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt       int64              `json:"updatedAt" bson:"updatedAt"`
}
