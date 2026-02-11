// Package models - Permission thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Permission quyền trong hệ thống.
type Permission struct {
	_Relationships struct{}          `relationship:"collection:role_permissions,field:permissionId,message:Không thể xóa permission vì có %d role đang sử dụng permission này. Vui lòng gỡ permission khỏi các role trước."`
	ID             primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name           string             `json:"name" bson:"name" index:"unique"`
	Describe       string             `json:"describe" bson:"describe"`
	Category       string             `json:"category" bson:"category"`
	Group          string             `json:"group" bson:"group"`
	IsSystem       bool               `json:"-" bson:"isSystem" index:"single:1"`
	CreatedAt      int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt      int64              `json:"updatedAt" bson:"updatedAt"`
}
