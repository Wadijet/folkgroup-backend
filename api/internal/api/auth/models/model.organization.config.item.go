// Package models - OrganizationConfigItem thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationConfigItem lưu một key config của một tổ chức (1 document per key).
type OrganizationConfigItem struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:owner_key_unique"`
	Key                 string             `json:"key" bson:"key" index:"single:1,compound:owner_key_unique"`
	Value               interface{}        `json:"value" bson:"value"`
	Name                string             `json:"name" bson:"name"`
	Description         string             `json:"description" bson:"description"`
	DataType            string             `json:"dataType" bson:"dataType"`
	Constraints         string             `json:"constraints,omitempty" bson:"constraints,omitempty"`
	AllowOverride       bool               `json:"allowOverride" bson:"allowOverride"`
	IsSystem            bool               `json:"-" bson:"isSystem" index:"single:1"`
	CreatedAt           int64              `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64              `json:"updatedAt" bson:"updatedAt"`
}
