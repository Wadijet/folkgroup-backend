// Package models - UserRole thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UserRole vai trò người dùng trong hệ thống.
type UserRole struct {
	ID        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	UserID    primitive.ObjectID `json:"userId" bson:"userId" index:"single:1"`
	RoleID    primitive.ObjectID `json:"roleId" bson:"roleId" index:"single:1"`
	CreatedAt int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64              `json:"updatedAt" bson:"updatedAt"`
}
