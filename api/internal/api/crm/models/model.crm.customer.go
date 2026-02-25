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
	TotalSpent         float64 `json:"totalSpent" bson:"totalSpent"`
	OrderCount         int     `json:"orderCount" bson:"orderCount"`
	LastOrderAt        int64   `json:"lastOrderAt,omitempty" bson:"lastOrderAt,omitempty" index:"single:-1,compound:crm_customer_org_lastorder"` // Unix ms — index cho sort dashboard
	SecondLastOrderAt  int64   `json:"secondLastOrderAt,omitempty" bson:"secondLastOrderAt,omitempty"` // Unix ms — đơn thứ 2 gần nhất (để tính REACTIVATED)
	RevenueLast30d     float64 `json:"revenueLast30d" bson:"revenueLast30d"`                       // Doanh thu 30 ngày qua (cho Momentum)
	RevenueLast90d     float64 `json:"revenueLast90d" bson:"revenueLast90d"`                       // Doanh thu 90 ngày qua (cho Momentum)

	// Merge metadata
	MergeMethod string `json:"mergeMethod" bson:"mergeMethod"` // customer_id | fb_id | phone | single_source
	MergedAt    int64  `json:"mergedAt" bson:"mergedAt"`

	// Phân quyền
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1,compound:crm_customer_org_unified_unique,compound:crm_customer_org_lastorder,compound:crm_customer_org_phones"`

	// Metadata
	CreatedAt int64 `json:"createdAt" bson:"createdAt" index:"single:1"`
	UpdatedAt int64 `json:"updatedAt" bson:"updatedAt" index:"single:1"`
}
