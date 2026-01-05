package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationRoutingRule - Routing rule định nghĩa: Event nào → Gửi cho teams nào → Qua channels nào
// Có thể routing theo EventType (cụ thể) hoặc Domain (tổng quát)
// Có thể filter theo Severity để tránh spam
// Lưu ý: Mỗi organization chỉ có thể có 1 rule cho mỗi eventType hoặc domain (unique constraint)
type NotificationRoutingRule struct {
	ID                  primitive.ObjectID   `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID primitive.ObjectID   `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:eventType_ownerOrg_unique,compound:domain_ownerOrg_unique"` // Tổ chức sở hữu rule (phân quyền dữ liệu)
	EventType           *string              `json:"eventType,omitempty" bson:"eventType,omitempty" index:"single:1,compound:eventType_ownerOrg_unique"`                               // Optional: routing theo eventType cụ thể (null = dùng Domain)
	Domain              *string              `json:"domain,omitempty" bson:"domain,omitempty" index:"single:1,compound:domain_ownerOrg_unique"`                                          // Optional: routing theo domain (null = dùng EventType)
	Description         string               `json:"description,omitempty" bson:"description,omitempty"`                                                                                // Mô tả về routing rule để người dùng hiểu được mục đích sử dụng
	OrganizationIDs     []primitive.ObjectID `json:"organizationIds" bson:"organizationIds"`                                                                                             // Teams nào nhận (có thể nhiều)
	ChannelTypes        []string             `json:"channelTypes,omitempty" bson:"channelTypes,omitempty"`                                                                            // Filter channels theo type (optional: email, telegram, webhook)
	Severities          []string             `json:"severities,omitempty" bson:"severities,omitempty"`                                                                                  // Filter theo severity (optional: ["critical", "high"] - chỉ nhận các severity này)
	IsActive            bool                 `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem            bool                 `json:"-" bson:"isSystem" index:"single:1"` // true = dữ liệu hệ thống, không thể xóa (chỉ dùng nội bộ, không expose ra API)
	CreatedAt           int64                `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64                `json:"updatedAt" bson:"updatedAt"`
}

