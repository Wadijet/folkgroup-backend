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

// CrmCustomerProfile thông tin profile khách — gộp trong 1 field cho gọn, đồng nhất với profileSnapshot trong activity.
type CrmCustomerProfile struct {
	Name        string        `json:"name,omitempty" bson:"name,omitempty"`
	PhoneNumbers []string     `json:"phoneNumbers,omitempty" bson:"phoneNumbers,omitempty"`
	Emails      []string     `json:"emails,omitempty" bson:"emails,omitempty"`
	Birthday    string       `json:"birthday,omitempty" bson:"birthday,omitempty"`
	Gender      string       `json:"gender,omitempty" bson:"gender,omitempty"`
	LivesIn     string       `json:"livesIn,omitempty" bson:"livesIn,omitempty"`
	Addresses   []interface{} `json:"addresses,omitempty" bson:"addresses,omitempty"`
	ReferralCode string      `json:"referralCode,omitempty" bson:"referralCode,omitempty"`
}

// CrmCustomer lưu khách hàng đã merge (crm_customers).
type CrmCustomer struct {
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	// Identity
	UnifiedId     string               `json:"unifiedId" bson:"unifiedId" index:"single:1,compound:crm_customer_org_unified_unique"`
	SourceIds     CrmCustomerSourceIds `json:"sourceIds" bson:"sourceIds"`
	PrimarySource string               `json:"primarySource" bson:"primarySource"` // pos | fb

	// Profile — thông tin cá nhân gộp trong 1 field; đồng nhất với profileSnapshot trong crm_activity_history.
	Profile CrmCustomerProfile `json:"profile" bson:"profile"`

	// Legacy — document cũ có name, phoneNumbers... ở top-level; dùng khi Profile trống (chưa migrate).
	LegacyName         string        `json:"-" bson:"name,omitempty"`
	LegacyPhoneNumbers []string      `json:"-" bson:"phoneNumbers,omitempty"`
	LegacyEmails       []string      `json:"-" bson:"emails,omitempty"`
	LegacyBirthday     string        `json:"-" bson:"birthday,omitempty"`
	LegacyGender       string        `json:"-" bson:"gender,omitempty"`
	LegacyLivesIn      string        `json:"-" bson:"livesIn,omitempty"`
	LegacyAddresses    []interface{} `json:"-" bson:"addresses,omitempty"`
	LegacyReferralCode string        `json:"-" bson:"referralCode,omitempty"`

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

	// CurrentMetrics: trạng thái metrics hiện tại (nested raw/layer1/layer2/layer3).
	// Luôn mới nhất — phục vụ phân tích, thống kê real-time. Cập nhật cùng lúc với scalars qua hooks/merge.
	CurrentMetrics map[string]interface{} `json:"currentMetrics,omitempty" bson:"currentMetrics,omitempty"`

	// Phân loại hiện tại (cập nhật cùng lúc với metrics qua hooks).
	// Dùng cho filter/sort dashboard; activity history giữ snapshot lịch sử theo từng sự kiện.
	ValueTier      string `json:"valueTier,omitempty" bson:"valueTier,omitempty" index:"single:1,compound:crm_customer_org_value"`             // vip|high|medium|low|new
	LifecycleStage string `json:"lifecycleStage,omitempty" bson:"lifecycleStage,omitempty" index:"single:1,compound:crm_customer_org_lifecycle"`   // active|cooling|inactive|dead|never_purchased
	JourneyStage   string `json:"journeyStage,omitempty" bson:"journeyStage,omitempty" index:"single:1,compound:crm_customer_org_journey"`       // visitor|engaged|first|repeat|vip|inactive
	Channel        string `json:"channel,omitempty" bson:"channel,omitempty" index:"single:1,compound:crm_customer_org_channel"`                 // online|offline|omnichannel
	LoyaltyStage   string `json:"loyaltyStage,omitempty" bson:"loyaltyStage,omitempty" index:"single:1,compound:crm_customer_org_loyalty"`       // core|repeat|one_time
	MomentumStage  string `json:"momentumStage,omitempty" bson:"momentumStage,omitempty" index:"single:1,compound:crm_customer_org_momentum"`    // rising|stable|declining|lost

	// Merge metadata
	MergeMethod string `json:"mergeMethod" bson:"mergeMethod"` // customer_id | fb_id | phone | single_source
	MergedAt    int64  `json:"mergedAt" bson:"mergedAt"`

	// Phân quyền
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:crm_customer_org_unified_unique,compound:crm_customer_org_lastorder,compound:crm_customer_org_totalspent,compound:crm_customer_org_value,compound:crm_customer_org_journey,compound:crm_customer_org_lifecycle,compound:crm_customer_org_channel,compound:crm_customer_org_loyalty,compound:crm_customer_org_momentum"`

	// Metadata
	CreatedAt int64 `json:"createdAt" bson:"createdAt" index:"single:1"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt" index:"single:1"`
}
