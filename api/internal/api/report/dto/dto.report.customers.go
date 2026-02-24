// Package reportdto - DTO cho Customer Intelligence (Tab 4 Dashboard).
// Đo lường chất lượng tài sản khách hàng: KPI, phân bố tier, lifecycle, bảng khách, VIP inactive panel.
package reportdto

// CustomersQueryParams query params cho GET /dashboard/customers.
type CustomersQueryParams struct {
	From              string `query:"from"`              // dd-mm-yyyy (cho period=custom)
	To                string `query:"to"`                // dd-mm-yyyy (cho period=custom)
	Period            string `query:"period"`            // day|week|month|60d|90d|year|custom
	Filter            string `query:"filter"`            // vip_inactive|inactive|cooling|active|tier_*|all
	Limit             int    `query:"limit"`            // Số dòng customer table (mặc định 500)
	Offset            int    `query:"offset"`           // Phân trang
	Sort              string `query:"sort"`              // days_since_desc|total_spend_desc|last_order_desc|name_asc
	VipInactiveLimit   int    `query:"vipInactiveLimit"` // Số khách VIP inactive trong panel (mặc định 15)
	ActiveDays        int    `query:"activeDays"`        // Ngưỡng Active: days ≤ 30
	CoolingDays       int    `query:"coolingDays"`       // Ngưỡng Cooling: 30 < days ≤ 60
	InactiveDays      int    `query:"inactiveDays"`      // Ngưỡng Inactive: 60 < days ≤ 90
}

// CustomerSummary 6 KPI cho Row 1 Tab 4.
type CustomerSummary struct {
	TotalCustomers       int64   `json:"totalCustomers"`       // Tổng khách hàng
	NewCustomersInPeriod int64   `json:"newCustomersInPeriod"` // Khách mới trong period (đơn đầu trong khoảng thời gian)
	RepeatRate           float64 `json:"repeatRate"`          // Khách ≥2 đơn / Tổng có đơn
	VipInactiveCount     int64   `json:"vipInactiveCount"`   // Số VIP inactive (Gold/Platinum, days > 90)
	ReactivationValue   int64   `json:"reactivationValue"`   // Tổng trị giá tái KHO = SUM(purchased_amount) VIP inactive
	ActiveTodayCount     int64   `json:"activeTodayCount"`    // Khách có đơn trong 24h
}

// CustomerItem 1 dòng trong bảng Customer Row 4.
type CustomerItem struct {
	CustomerID        string   `json:"customerId"`
	Name             string   `json:"name"`
	Phone            string   `json:"phone"`
	Tier             string   `json:"tier"`             // new|silver|gold|platinum
	TotalSpend       float64  `json:"totalSpend"`
	OrderCount       int64    `json:"orderCount"`
	LastOrderAt      string   `json:"lastOrderAt"`      // ISO format
	DaysSinceLast    int64    `json:"daysSinceLast"`    // -1 nếu chưa mua
	Lifecycle        string   `json:"lifecycle"`        // active|cooling|inactive|vip_inactive|never_purchased
	AssignedSale     string   `json:"assignedSale"`
	Tags             []string `json:"tags"`
}

// VipInactiveItem 1 dòng trong panel VIP Inactive Customers (right panel).
type VipInactiveItem struct {
	CustomerID     string  `json:"customerId"`
	Name           string  `json:"name"`
	TotalSpend     float64 `json:"totalSpend"`
	DaysSinceLast  int64   `json:"daysSinceLast"`
	AssignedSale   string  `json:"assignedSale"`
}

// TierDistribution phân bố theo tier (New, Silver, Gold, Platinum).
type TierDistribution struct {
	New      int64 `json:"new"`
	Silver   int64 `json:"silver"`
	Gold     int64 `json:"gold"`
	Platinum int64 `json:"platinum"`
}

// LifecycleDistribution phân bố theo lifecycle.
type LifecycleDistribution struct {
	Active         int64 `json:"active"`
	Cooling        int64 `json:"cooling"`
	Inactive       int64 `json:"inactive"`
	VipInactive    int64 `json:"vip_inactive"`
	NeverPurchased int64 `json:"never_purchased"`
}

// CustomersSnapshotResult kết quả trả về cho GET /dashboard/customers.
type CustomersSnapshotResult struct {
	Summary               CustomerSummary          `json:"summary"`
	SummaryStatuses       map[string]string        `json:"summaryStatuses,omitempty"` // green|yellow|red từng KPI
	TierDistribution      TierDistribution         `json:"tierDistribution"`
	LifecycleDistribution LifecycleDistribution    `json:"lifecycleDistribution"`
	Customers             []CustomerItem           `json:"customers"`
	VipInactiveCustomers  []VipInactiveItem        `json:"vipInactiveCustomers"`
}
