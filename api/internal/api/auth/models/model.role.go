// Package models - Role thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Role vai trò trong hệ thống.
type Role struct {
	_Relationships        struct{}           `relationship:"collection:user_roles,field:roleId,message:Không thể xóa role vì có %d user đang sử dụng role này. Vui lòng gỡ role khỏi các user trước.|collection:role_permissions,field:roleId,message:Không thể xóa role vì có %d permission đang được gán cho role này. Vui lòng gỡ các permission trước."`
	ID                   primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Name                 string             `json:"name" bson:"name" index:"compound:role_org_name_unique"`
	Describe             string             `json:"describe" bson:"describe"`
	OwnerOrganizationID  primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:role_org_name_unique"`
	IsSystem             bool               `json:"-" bson:"isSystem" index:"single:1"`
	CreatedAt            int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt            int64              `json:"updatedAt" bson:"updatedAt"`
}
