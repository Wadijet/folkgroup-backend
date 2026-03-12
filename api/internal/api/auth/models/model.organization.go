// Package models - Organization thuộc domain auth.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrganizationType các loại tổ chức
const (
	OrganizationTypeSystem     = "system"
	OrganizationTypeGroup      = "group"
	OrganizationTypeCompany    = "company"
	OrganizationTypeDepartment = "department"
	OrganizationTypeDivision   = "division"
	OrganizationTypeTeam       = "team"
)

// TeamCodeDefault các mã team mặc định cho định tuyến thông báo theo domain
const (
	TeamCodeTech       = "TECH_TEAM"       // Tech Team — system, security, user
	TeamCodeMarketing  = "MARKETING_TEAM"  // Marketing — ads, analytics
	TeamCodeSales      = "SALES_TEAM"      // Sales — conversation (chat, tin nhắn)
	TeamCodeOperations = "OPERATIONS_TEAM" // Vận hành — order, payment
)

// Organization đại diện cấu trúc tổ chức hình cây.
type Organization struct {
	_Relationships struct{}           `relationship:"collection:roles,field:organizationId,message:Không thể xóa tổ chức vì có %d role trực thuộc. Vui lòng xóa hoặc di chuyển các role trước."`
	ID             primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	Name           string              `json:"name" bson:"name" index:"single:1"`
	Code           string              `json:"code" bson:"code" index:"unique"`
	Type           string              `json:"type" bson:"type" index:"single:1"`
	ParentID       *primitive.ObjectID `json:"parentId,omitempty" bson:"parentId,omitempty" index:"single:1"`
	Path           string              `json:"path" bson:"path" index:"single:1"`
	Level          int                 `json:"level" bson:"level" index:"single:1"`
	IsActive       bool                `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem       bool                `json:"-" bson:"isSystem" index:"single:1"`
	CreatedAt      int64               `json:"createdAt" bson:"createdAt"`
	UpdatedAt      int64               `json:"updatedAt" bson:"updatedAt"`
}
