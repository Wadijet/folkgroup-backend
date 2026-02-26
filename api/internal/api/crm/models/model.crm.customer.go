// Package models - CrmCustomer thuộc domain CRM (crm_customers).
// Lưu khách hàng đã merge từ FB + POS, dùng làm nguồn chính cho dashboard và phân loại.
package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerSourceIds chứa customerId từ từng nguồn (pos, fb).
type CrmCustomerSourceIds struct {
	Pos string `json:"pos,omitempty" bson:"pos,omitempty"` // UUID từ pc_pos_customers
	Fb  string `json:"fb,omitempty" bson:"fb,omitempty"`   // UUID từ fb_customers
}

// CrmCustomer lưu khách hàng đã merge (crm_customers).
type CrmCustomer struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	// Identity
	UnifiedId      string `json:"unifiedId" bson:"unifiedId" index:"single:1,compound:crm_customer_org_unified_unique"`
	SourceIds      CrmCustomerSourceIds `json:"sourceIds" bson:"sourceIds"`
	PrimarySource  string `json:"primarySource" bson:"primarySource"` // pos | fb
	Name           string   `json:"name" bson:"name"`
	PhoneNumbers   []string `json:"phoneNumbers" bson:"phoneNumbers" index:"single:1,compound:crm_customer_org_phones"`
	Emails         []string `json:"emails" bson:"emails"`

	// Thông tin bổ sung (merge từ POS/FB/conversation/order)
	Birthday   string        `json:"birthday,omitempty" bson:"birthday,omitempty"`       // Ngày sinh (date_of_birth, birthday)
	Gender     string        `json:"gender,omitempty" bson:"gender,omitempty"`           // Giới tính
	LivesIn    string        `json:"livesIn,omitempty" bson:"livesIn,omitempty"`         // Nơi ở (từ FB)
	Addresses  []interface{} `json:"addresses,omitempty" bson:"addresses,omitempty"`     // Địa chỉ (từ POS shop_customer_address)
	ReferralCode string      `json:"referralCode,omitempty" bson:"referralCode,omitempty"` // Mã giới thiệu (từ posData.referral_code)

	// Flags
	HasConversation bool `json:"hasConversation" bson:"hasConversation"`
	HasOrder        bool `json:"hasOrder" bson:"hasOrder"`

	// Channel metrics (online vs offline)
	OrderCountOnline   int    `json:"orderCountOnline" bson:"orderCountOnline"`
	OrderCountOffline  int    `json:"orderCountOffline" bson:"orderCountOffline"`
	FirstOrderChannel  string `json:"firstOrderChannel,omitempty" bson:"firstOrderChannel,omitempty"` // online | offline
	LastOrderChannel   string `json:"lastOrderChannel,omitempty" bson:"lastOrderChannel,omitempty"`
	IsOmnichannel      bool   `json:"isOmnichannel" bson:"isOmnichannel"`

	// Cached metrics (aggregate từ pc_pos_orders, cập nhật qua hooks)
	TotalSpent         float64 `json:"totalSpent" bson:"totalSpent" index:"compound:crm_customer_org_totalspent,order:-1"` // Index cho sort totalSpend desc trong dashboard
	OrderCount         int     `json:"orderCount" bson:"orderCount"`
	LastOrderAt        int64   `json:"lastOrderAt,omitempty" bson:"lastOrderAt,omitempty" index:"single:-1,compound:crm_customer_org_lastorder,order:-1"` // Unix ms — index cho sort dashboard (ownerOrg + lastOrderAt desc)
	SecondLastOrderAt  int64   `json:"secondLastOrderAt,omitempty" bson:"secondLastOrderAt,omitempty"` // Unix ms — đơn thứ 2 gần nhất (để tính REACTIVATED)
	RevenueLast30d     float64 `json:"revenueLast30d" bson:"revenueLast30d"`                       // Doanh thu 30 ngày qua (cho Momentum)
	RevenueLast90d     float64 `json:"revenueLast90d" bson:"revenueLast90d"`                       // Doanh thu 90 ngày qua (cho Momentum)
	AvgOrderValue      float64 `json:"avgOrderValue" bson:"avgOrderValue"`                        // AOV = totalSpent / orderCount
	CancelledOrderCount int    `json:"cancelledOrderCount" bson:"cancelledOrderCount"`            // Số đơn đã hủy (status 6)
	OrdersLast30d      int     `json:"ordersLast30d" bson:"ordersLast30d"`                        // Số đơn 30 ngày qua
	OrdersLast90d      int     `json:"ordersLast90d" bson:"ordersLast90d"`                        // Số đơn 90 ngày qua
	OrdersFromAds      int     `json:"ordersFromAds" bson:"ordersFromAds"`                        // Đơn từ Meta ads (order_sources = -1)
	OrdersFromOrganic  int     `json:"ordersFromOrganic" bson:"ordersFromOrganic"`                // Đơn organic
	OrdersFromDirect   int     `json:"ordersFromDirect" bson:"ordersFromDirect"`                  // Đơn direct
	OwnedSkuQuantities map[string]int `json:"ownedSkuQuantities,omitempty" bson:"ownedSkuQuantities,omitempty"` // SKU (product_display_id) -> số lượng sở hữu

	// Conversation metrics (aggregate từ fb_conversations)
	ConversationCount         int   `json:"conversationCount" bson:"conversationCount"`
	ConversationCountByInbox  int   `json:"conversationCountByInbox" bson:"conversationCountByInbox"`   // type = INBOX
	ConversationCountByComment int  `json:"conversationCountByComment" bson:"conversationCountByComment"` // type = COMMENT
	LastConversationAt        int64 `json:"lastConversationAt,omitempty" bson:"lastConversationAt,omitempty"`
	FirstConversationAt       int64 `json:"firstConversationAt,omitempty" bson:"firstConversationAt,omitempty"`
	TotalMessages             int   `json:"totalMessages" bson:"totalMessages"`
	LastMessageFromCustomer   bool  `json:"lastMessageFromCustomer" bson:"lastMessageFromCustomer"` // Khách chủ động nhắn gần đây
	ConversationFromAds       bool  `json:"conversationFromAds" bson:"conversationFromAds"`         // Có hội thoại từ quảng cáo
	ConversationTags         []string `json:"conversationTags,omitempty" bson:"conversationTags,omitempty"` // Tag từ panCakeData.tags (union)

	// Phân loại hiện tại (cập nhật cùng lúc với metrics qua hooks).
	// Dùng cho filter/sort dashboard; activity history giữ snapshot lịch sử theo từng sự kiện.
	ValueTier      string `json:"valueTier,omitempty" bson:"valueTier,omitempty"`             // vip|high|medium|low|new
	LifecycleStage string `json:"lifecycleStage,omitempty" bson:"lifecycleStage,omitempty"`   // active|cooling|inactive|dead|never_purchased
	JourneyStage   string `json:"journeyStage,omitempty" bson:"journeyStage,omitempty"`       // visitor|engaged|first|repeat|vip|inactive
	Channel        string `json:"channel,omitempty" bson:"channel,omitempty"`                 // online|offline|omnichannel
	LoyaltyStage   string `json:"loyaltyStage,omitempty" bson:"loyaltyStage,omitempty"`       // core|repeat|one_time
	MomentumStage  string `json:"momentumStage,omitempty" bson:"momentumStage,omitempty"`    // rising|stable|declining|lost

	// Merge metadata
	MergeMethod string `json:"mergeMethod" bson:"mergeMethod"` // customer_id | fb_id | phone | single_source
	MergedAt    int64  `json:"mergedAt" bson:"mergedAt"`

	// Phân quyền
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:crm_customer_org_unified_unique,compound:crm_customer_org_lastorder,compound:crm_customer_org_phones,compound:crm_customer_org_totalspent"`

	// Metadata
	CreatedAt int64 `json:"createdAt" bson:"createdAt" index:"single:1"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt" index:"single:1"`
}
