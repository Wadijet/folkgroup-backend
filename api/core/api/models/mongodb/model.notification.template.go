package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotificationTemplate - Template thông báo
type NotificationTemplate struct {
	ID                 primitive.ObjectID  `json:"id,omitempty" bson:"_id,omitempty"`
	OwnerOrganizationID *primitive.ObjectID `json:"ownerOrganizationId,omitempty" bson:"ownerOrganizationId,omitempty" index:"single:1"` // Tổ chức sở hữu dữ liệu (phân quyền) - null = System Organization
	EventType      string              `json:"eventType" bson:"eventType" index:"single:1"`                                // conversation_unreplied, order_created, ...
	ChannelType    string              `json:"channelType" bson:"channelType" index:"single:1"`                           // email, telegram, webhook
	Description    string              `json:"description,omitempty" bson:"description,omitempty"`                        // Mô tả về template để người dùng hiểu được mục đích sử dụng
	Subject        string              `json:"subject,omitempty" bson:"subject,omitempty"`                                  // Cho email
	Content        string              `json:"content" bson:"content"`                                                      // Có thể chứa {{variable}}
	Variables      []string            `json:"variables" bson:"variables"`                                                  // ["conversationId", "minutes"]

	// CTA codes (mới) - tham chiếu đến CTALibrary
	CTACodes []string `json:"ctaCodes,omitempty" bson:"ctaCodes,omitempty"` // ["view_detail", "reply", "mark_read"]

	// CTA buttons (cũ - deprecated, giữ lại để backward compatibility)
	CTAs []NotificationCTA `json:"ctas,omitempty" bson:"ctas,omitempty"` // Deprecated: Dùng CTACodes thay thế

	IsActive  bool  `json:"isActive" bson:"isActive" index:"single:1"`
	IsSystem  bool  `json:"-" bson:"isSystem" index:"single:1"` // true = dữ liệu hệ thống, không thể xóa (chỉ dùng nội bộ, không expose ra API)
	CreatedAt int64 `json:"createdAt" bson:"createdAt"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt"`
}

// NotificationCTA - CTA button
type NotificationCTA struct {
	Label  string `json:"label" bson:"label"`         // "Xem chi tiết", "Phản hồi", "Đã xem"
	Action string `json:"action" bson:"action"`       // URL (có thể chứa {{variable}})
	Style  string `json:"style,omitempty" bson:"style,omitempty"` // "primary", "success", "secondary" (chỉ để styling)
}

