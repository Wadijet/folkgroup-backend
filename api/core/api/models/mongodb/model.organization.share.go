package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationShare đại diện cho việc share dữ liệu giữa các organizations
// Organization A có thể share tất cả data của mình với Organization B hoặc nhiều organizations
// Nếu ToOrgIDs rỗng hoặc null → share với tất cả organizations
type OrganizationShare struct {
	ID                  primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền) - Organization share data với ToOrgIDs
	ToOrgIDs            []primitive.ObjectID `json:"toOrgIds,omitempty" bson:"toOrgIds"`                     // Organizations nhận data ([] hoặc null = share với tất cả organizations) - Bỏ omitempty để luôn lưu field (kể cả empty array)
	PermissionNames     []string             `json:"permissionNames,omitempty" bson:"permissionNames,omitempty"`      // [] hoặc nil = tất cả permissions, ["Order.Read", "Order.Create"] = chỉ share với permissions cụ thể
	Description         string               `json:"description,omitempty" bson:"description,omitempty"`                // Mô tả về lệnh share để người dùng hiểu được mục đích
	CreatedAt           int64                `json:"createdAt" bson:"createdAt"`
	CreatedBy           primitive.ObjectID   `json:"createdBy" bson:"createdBy"`
}
