package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CTALibrary - CTA Library Template
// Lưu trữ các CTA templates có thể reuse
type CTALibrary struct {
	ID                 primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID  `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"` // Tổ chức sở hữu (required) - System Organization ID = System/Global CTA
	Code               string              `json:"code" bson:"code" index:"single:1"`                                 // Mã CTA (unique trong organization)
	Label              string              `json:"label" bson:"label"`                                                 // Label hiển thị (có thể chứa {{variable}})
	Action             string              `json:"action" bson:"action"`                                                 // URL action (có thể chứa {{variable}})
	Style              string              `json:"style,omitempty" bson:"style,omitempty"`                          // Style: "primary", "success", "secondary", "danger"
	Variables          []string            `json:"variables" bson:"variables"`                                         // Danh sách variables cần render: ["conversationId", "orderId"]
	Description        string              `json:"description,omitempty" bson:"description,omitempty"`              // Mô tả về CTA để người dùng hiểu được mục đích sử dụng
	IsActive           bool                `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem           bool                `json:"-" bson:"isSystem" index:"single:1"` // true = dữ liệu hệ thống, không thể xóa
	CreatedAt          int64               `json:"createdAt" bson:"createdAt"`
	UpdatedAt          int64               `json:"updatedAt" bson:"updatedAt"`
}
