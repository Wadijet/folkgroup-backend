// Package reportdto - DTO cho Customer Intelligence (Tab 4 Dashboard).
// Đo lường chất lượng tài sản khách hàng: KPI, phân bố tier, lifecycle, bảng khách, VIP inactive panel.
package reportdto

// ParseCustomerSortParams parse sort theo chuẩn CRUD (sortField + sortOrder).
// Trả về (field, order): field=daysSinceLast|totalSpend|lastOrderAt|name, order=1(asc)|-1(desc).
// Chuẩn CRUD: options.sort dùng {"field": 1} hoặc {"field": -1}.
func ParseCustomerSortParams(sortField string, sortOrder int) (field string, order int) {
	allowed := map[string]bool{"daysSinceLast": true, "totalSpend": true, "lastOrderAt": true, "name": true}
	if sortField != "" && allowed[sortField] {
		if sortOrder == 1 || sortOrder == -1 {
			return sortField, sortOrder
		}
		return sortField, -1
	}
	return "daysSinceLast", -1
}

// CustomersQueryParams query params cho GET /dashboard/customers.
type CustomersQueryParams struct {
	From              string `query:"from"`              // dd-mm-yyyy (cho period=custom)
	To                string `query:"to"`                // dd-mm-yyyy (cho period=custom)
	Period            string `query:"period"`            // day|week|month|60d|90d|year|custom
	Filter            string `query:"filter"`            // vip_inactive|inactive|cooling|active|tier_*|all
	Limit             int    `query:"limit"`             // Số dòng customer table (mặc định 20)
	Offset            int    `query:"offset"`            // Phân trang
	SortField         string `query:"sortField"`         // Chuẩn CRUD: daysSinceLast|totalSpend|lastOrderAt|name (mặc định daysSinceLast)
	SortOrder         int    `query:"sortOrder"`         // Chuẩn CRUD: 1=tăng dần, -1=giảm dần (mặc định -1)
	VipInactiveLimit  int    `query:"vipInactiveLimit"`  // Số khách VIP inactive trong panel (mặc định 15)
	ActiveDays        int    `query:"activeDays"`        // Ngưỡng Active: days ≤ 30
	CoolingDays       int    `query:"coolingDays"`       // Ngưỡng Cooling: 30 < days ≤ 60
	InactiveDays      int    `query:"inactiveDays"`      // Ngưỡng Inactive: 60 < days ≤ 90
	// Query params mới theo CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN (dùng crm_customers)
	// Mỗi tiêu chí có thể nhận nhiều giá trị (comma-separated), ví dụ: journey=vip,engaged,first
	Journey   string `query:"journey"`   // visitor|engaged|first|repeat|vip|inactive
	Channel   string `query:"channel"`  // online|offline|omnichannel
	ValueTier string `query:"valueTier"` // vip|high|medium|low|new
	Lifecycle string `query:"lifecycle"` // active|cooling|inactive|dead|never_purchased
	Loyalty   string `query:"loyalty"`  // core|repeat|one_time
	Momentum  string `query:"momentum"`  // rising|stable|declining|lost
	CeoGroup  string `query:"ceoGroup"`  // vip_active|vip_inactive|rising|new|one_time|dead
	Source    string `query:"source"`    // crm (mặc định) | legacy (pc_pos_customers)
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
	Name              string   `json:"name"`
	Phone             string   `json:"phone"`
	Tier              string   `json:"tier,omitempty"`             // legacy: new|silver|gold|platinum
	TotalSpend        float64  `json:"totalSpend"`
	OrderCount        int64    `json:"orderCount"`
	LastOrderAt       string   `json:"lastOrderAt"`
	DaysSinceLast     int64    `json:"daysSinceLast"`
	Lifecycle         string   `json:"lifecycle"`
	AssignedSale      string   `json:"assignedSale"`
	Tags              []string `json:"tags"`
	// Trường mới theo CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN (khi source=crm)
	JourneyStage        string   `json:"journeyStage,omitempty"`
	Channel             string   `json:"channel,omitempty"` // online|offline|omnichannel
	ValueTier           string   `json:"valueTier,omitempty"`
	LifecycleStage      string   `json:"lifecycleStage,omitempty"`
	LoyaltyStage        string   `json:"loyaltyStage,omitempty"`
	MomentumStage       string   `json:"momentumStage,omitempty"`
	RevenueLast30d      float64  `json:"revenueLast30d,omitempty"`
	RevenueLast90d      float64  `json:"revenueLast90d,omitempty"`
	AvgOrderValue       float64  `json:"avgOrderValue,omitempty"`
	Sources             []string `json:"sources,omitempty"`
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
	Summary               CustomerSummary       `json:"summary"`
	SummaryStatuses       map[string]string     `json:"summaryStatuses,omitempty"`
	TierDistribution      TierDistribution      `json:"tierDistribution"`
	LifecycleDistribution LifecycleDistribution `json:"lifecycleDistribution"`
	Customers             []CustomerItem        `json:"customers"`
	VipInactiveCustomers  []VipInactiveItem     `json:"vipInactiveCustomers"`
	TotalCount            int                   `json:"totalCount,omitempty"` // Tổng số khách (cho phân trang, khi source=crm)
}
