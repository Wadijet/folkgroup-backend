// Package dto - DTO cho domain CRM (customer).
package dto

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerProfileResponse trả về profile đầy đủ của khách (metrics + classification).
type CrmCustomerProfileResponse struct {
	UnifiedId           string                 `json:"unifiedId"`
	Name                string                 `json:"name"`
	PhoneNumbers        []string               `json:"phoneNumbers"`
	Emails              []string               `json:"emails"`
	HasConversation     bool                   `json:"hasConversation"`
	TotalSpent          float64                `json:"totalSpent"`
	OrderCount          int                    `json:"orderCount"`
	OrderCountOnline    int                    `json:"orderCountOnline"`
	OrderCountOffline   int                    `json:"orderCountOffline"`
	FirstOrderChannel   string                 `json:"firstOrderChannel,omitempty"`
	LastOrderChannel    string                 `json:"lastOrderChannel,omitempty"`
	IsOmnichannel       bool                   `json:"isOmnichannel"`
	LastOrderAt         int64                  `json:"lastOrderAt,omitempty"`
	ValueTier           string                 `json:"valueTier,omitempty"`
	LifecycleStage      string                 `json:"lifecycleStage,omitempty"`
	JourneyStage        string                 `json:"journeyStage,omitempty"`
	Channel             string                 `json:"channel,omitempty"` // online | offline | omnichannel (rỗng nếu chưa mua)
	LoyaltyStage        string                 `json:"loyaltyStage,omitempty"`
	MomentumStage       string                 `json:"momentumStage,omitempty"`
	SourceIds           map[string]string      `json:"sourceIds,omitempty"`
	OwnerOrganizationId primitive.ObjectID     `json:"ownerOrganizationId,omitempty"`
}

// CrmCustomerFullProfileResponse trả về toàn bộ thông tin khách: profile + orders + conversations + notes + lịch sử hoạt động.
type CrmCustomerFullProfileResponse struct {
	Profile         CrmCustomerProfileResponse  `json:"profile"`
	RecentOrders    []CrmOrderSummary           `json:"recentOrders"`
	Conversations   []CrmConversationSummary    `json:"conversations"`
	Notes           []CrmNoteSummary           `json:"notes"`
	ActivityHistory []CrmActivitySummary       `json:"activityHistory"`
}

// CrmOrderSummary tóm tắt đơn hàng.
type CrmOrderSummary struct {
	OrderId     int64   `json:"orderId"`
	TotalAmount float64 `json:"totalAmount"`
	Status      int     `json:"status"`
	Channel     string  `json:"channel"` // online | offline
	CreatedAt   int64   `json:"createdAt"`
}

// CrmConversationSummary tóm tắt hội thoại.
type CrmConversationSummary struct {
	ConversationId   string `json:"conversationId"`
	PageId           string `json:"pageId,omitempty"`
	PanCakeUpdatedAt int64  `json:"panCakeUpdatedAt,omitempty"`
}

// CrmNoteSummary tóm tắt ghi chú.
type CrmNoteSummary struct {
	Id             string `json:"id"`
	NoteText       string `json:"noteText"`
	NextAction     string `json:"nextAction,omitempty"`
	NextActionDate int64  `json:"nextActionDate,omitempty"`
	CreatedBy      string `json:"createdBy,omitempty"`
	CreatedAt      int64  `json:"createdAt"`
}

// CrmActivitySummary tóm tắt hoạt động (lịch sử).
type CrmActivitySummary struct {
	ActivityType string                 `json:"activityType"`
	ActivityAt   int64                  `json:"activityAt"`
	Source       string                 `json:"source"`
	SourceRef    map[string]interface{}  `json:"sourceRef,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// CrmBackfillActivityInput input cho backfill activity (job bên ngoài gọi).
type CrmBackfillActivityInput struct {
	OwnerOrganizationId string `json:"ownerOrganizationId"` // Bắt buộc
	Limit                int    `json:"limit,omitempty"`    // Giới hạn mỗi loại, mặc định 10000
}

// CrmBackfillActivityResult kết quả backfill.
type CrmBackfillActivityResult struct {
	OrdersProcessed       int `json:"ordersProcessed"`
	ConversationsProcessed int `json:"conversationsProcessed"`
	NotesProcessed       int `json:"notesProcessed"`
}
