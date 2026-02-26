// Package dto - DTO cho domain CRM (customer).
package dto

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmCustomerProfileResponse trả về profile đầy đủ của khách (metrics + classification).
type CrmCustomerProfileResponse struct {
	UnifiedId                 string             `json:"unifiedId"`
	Name                      string             `json:"name"`
	PhoneNumbers              []string           `json:"phoneNumbers"`
	Emails                    []string           `json:"emails"`
	Birthday                  string             `json:"birthday,omitempty"`
	Gender                    string             `json:"gender,omitempty"`
	LivesIn                   string             `json:"livesIn,omitempty"`
	Addresses                 []interface{}      `json:"addresses,omitempty"`
	ReferralCode              string             `json:"referralCode,omitempty"`
	HasConversation           bool               `json:"hasConversation"`
	TotalSpent                float64            `json:"totalSpent"`
	OrderCount                int                `json:"orderCount"`
	OrderCountOnline          int                `json:"orderCountOnline"`
	OrderCountOffline         int                `json:"orderCountOffline"`
	FirstOrderChannel         string             `json:"firstOrderChannel,omitempty"`
	LastOrderChannel          string             `json:"lastOrderChannel,omitempty"`
	IsOmnichannel             bool               `json:"isOmnichannel"`
	LastOrderAt               int64              `json:"lastOrderAt,omitempty"`
	AvgOrderValue             float64            `json:"avgOrderValue,omitempty"`
	CancelledOrderCount       int                `json:"cancelledOrderCount,omitempty"`
	OrdersLast30d             int                `json:"ordersLast30d,omitempty"`
	OrdersLast90d             int                `json:"ordersLast90d,omitempty"`
	OrdersFromAds             int                `json:"ordersFromAds,omitempty"`
	OrdersFromOrganic         int                `json:"ordersFromOrganic,omitempty"`
	OrdersFromDirect          int                `json:"ordersFromDirect,omitempty"`
	OwnedSkuQuantities        map[string]int     `json:"ownedSkuQuantities,omitempty"`
	ConversationCount         int                `json:"conversationCount,omitempty"`
	ConversationCountByInbox  int                `json:"conversationCountByInbox,omitempty"`
	ConversationCountByComment int               `json:"conversationCountByComment,omitempty"`
	LastConversationAt        int64              `json:"lastConversationAt,omitempty"`
	FirstConversationAt       int64              `json:"firstConversationAt,omitempty"`
	TotalMessages             int                `json:"totalMessages,omitempty"`
	LastMessageFromCustomer   bool               `json:"lastMessageFromCustomer,omitempty"`
	ConversationFromAds       bool               `json:"conversationFromAds,omitempty"`
	ConversationTags          []string           `json:"conversationTags,omitempty"`
	ValueTier                 string             `json:"valueTier,omitempty"`
	LifecycleStage            string             `json:"lifecycleStage,omitempty"`
	JourneyStage              string             `json:"journeyStage,omitempty"`
	Channel                   string             `json:"channel,omitempty"` // online | offline | omnichannel (rỗng nếu chưa mua)
	LoyaltyStage              string             `json:"loyaltyStage,omitempty"`
	MomentumStage             string             `json:"momentumStage,omitempty"`
	SourceIds                 map[string]string  `json:"sourceIds,omitempty"`
	OwnerOrganizationId      primitive.ObjectID `json:"ownerOrganizationId,omitempty"`
}

// CrmCustomerFullProfileResponse trả về toàn bộ thông tin khách: profile + orders + conversations + notes + lịch sử hoạt động.
// currentMetrics: số liệu hiện tại (cùng cấu trúc với metadata.metricsSnapshot) — góc nhìn now song song với lịch sử.
type CrmCustomerFullProfileResponse struct {
	Profile         CrmCustomerProfileResponse  `json:"profile"`
	CurrentMetrics map[string]interface{}       `json:"currentMetrics,omitempty"` // Số liệu hiện tại — so sánh với metricsSnapshot (lịch sử) trong activityHistory
	RecentOrders   []CrmOrderSummary           `json:"recentOrders"`
	Conversations  []CrmConversationSummary    `json:"conversations"`
	Notes          []CrmNoteSummary            `json:"notes"`
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

// ActivityChangeItem mô tả một thay đổi (field, oldValue, newValue).
type ActivityChangeItem struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"oldValue,omitempty"`
	NewValue interface{} `json:"newValue,omitempty"`
}

// CrmActivitySummary tóm tắt hoạt động (lịch sử).
type CrmActivitySummary struct {
	ActivityType   string                 `json:"activityType"`
	Domain         string                 `json:"domain"`
	ActivityAt     int64                  `json:"activityAt"`
	Source         string                 `json:"source"`
	SourceRef      map[string]interface{} `json:"sourceRef,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	DisplayLabel   string                 `json:"displayLabel,omitempty"`
	DisplayIcon    string                 `json:"displayIcon,omitempty"`
	DisplaySubtext string                 `json:"displaySubtext,omitempty"`
	ActorId        string                 `json:"actorId,omitempty"`
	ActorName      string                 `json:"actorName,omitempty"`
	Changes        []ActivityChangeItem   `json:"changes,omitempty"`
	Reason         string                 `json:"reason,omitempty"`
	ClientIp       string                 `json:"clientIp,omitempty"`
	UserAgent      string                 `json:"userAgent,omitempty"`
	Status         string                 `json:"status,omitempty"`
}

// CrmBackfillActivityInput input cho backfill activity (job bên ngoài gọi).
type CrmBackfillActivityInput struct {
	OwnerOrganizationId string `json:"ownerOrganizationId"` // Bắt buộc
	Limit                int    `json:"limit,omitempty"`    // Giới hạn mỗi loại, mặc định 10000
}

// CrmBackfillActivityResult kết quả backfill.
type CrmBackfillActivityResult struct {
	OrdersProcessed                int   `json:"ordersProcessed"`
	ConversationsProcessed          int   `json:"conversationsProcessed"`
	ConversationsLogged             int   `json:"conversationsLogged"`             // Số conversation đã ghi activity thành công
	ConversationsSkippedNoResolve   int   `json:"conversationsSkippedNoResolve"`   // Số conversation không resolve được (thiếu fb_customers)
	NotesProcessed                  int   `json:"notesProcessed"`
	Diagnostic                      *struct {
		TotalConversations       int64   `json:"totalConversations"`       // Tổng fb_conversations
		ConversationsWithOrg     int64   `json:"conversationsWithOrg"`     // fb_conversations có ownerOrganizationId khớp
		TotalOrders              int64   `json:"totalOrders"`              // Tổng pc_pos_orders
		OrdersWithOrg            int64   `json:"ordersWithOrg"`            // pc_pos_orders có ownerOrganizationId khớp
		SampleOrgIdsConversations []string `json:"sampleOrgIdsConversations"` // Mẫu ownerOrganizationId trong fb_conversations (nếu có)
	} `json:"diagnostic,omitempty"` // Chỉ có khi tất cả = 0 để hỗ trợ chẩn đoán
}

// CrmRebuildResult kết quả rebuild (sync + backfill).
type CrmRebuildResult struct {
	Sync    CrmSyncResult               `json:"sync"`    // Kết quả sync profile
	Backfill CrmBackfillActivityResult `json:"backfill"` // Kết quả backfill activity
}

// CrmSyncResult kết quả sync.
type CrmSyncResult struct {
	PosProcessed int `json:"posProcessed"`
	FbProcessed  int `json:"fbProcessed"`
}
