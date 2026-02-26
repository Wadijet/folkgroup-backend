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
	Source    string `query:"source"`    // Chỉ dùng crm (legacy đã bỏ)
}

// CustomerSummary 6 KPI cho Row 1 Tab 4.
type CustomerSummary struct {
	TotalCustomers       int64   `json:"totalCustomers"`       // Tổng khách hàng
	NewCustomersInPeriod int64   `json:"newCustomersInPeriod"` // Khách mới trong period (đơn đầu trong khoảng thời gian)
	RepeatRate           float64 `json:"repeatRate"`          // Khách ≥2 đơn / Tổng có đơn
	VipInactiveCount     int64   `json:"vipInactiveCount"`   // Số VIP inactive (Value=VIP, Lifecycle=inactive/dead)
	ReactivationValue   int64   `json:"reactivationValue"`   // Tổng trị giá tái KHO = SUM(purchased_amount) VIP inactive
	ActiveTodayCount     int64   `json:"activeTodayCount"`    // Khách có đơn trong 24h
}

// CustomerItem 1 dòng trong bảng Customer Row 4.
type CustomerItem struct {
	CustomerID        string   `json:"customerId"`
	Name              string   `json:"name"`
	Phone             string   `json:"phone"`
	Tier              string   `json:"tier,omitempty"`             // deprecated — dùng valueTier
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

// ValueDistribution phân bố theo Value (CRM): vip, high, medium, low, new.
type ValueDistribution struct {
	Vip    int64 `json:"vip"`
	High   int64 `json:"high"`
	Medium int64 `json:"medium"`
	Low    int64 `json:"low"`
	New    int64 `json:"new"`
}

// JourneyDistribution phân bố theo Journey (CRM): visitor, engaged, first, repeat, vip, inactive.
type JourneyDistribution struct {
	Visitor  int64 `json:"visitor"`
	Engaged  int64 `json:"engaged"`
	First    int64 `json:"first"`
	Repeat   int64 `json:"repeat"`
	Vip      int64 `json:"vip"`
	Inactive int64 `json:"inactive"`
}

// LifecycleDistribution phân bố theo Lifecycle (CRM): active, cooling, inactive, dead, never_purchased.
type LifecycleDistribution struct {
	Active         int64 `json:"active"`
	Cooling        int64 `json:"cooling"`
	Inactive       int64 `json:"inactive"`
	Dead           int64 `json:"dead"`
	NeverPurchased int64 `json:"never_purchased"`
}

// ChannelDistribution phân bố theo Channel (CRM): online, offline, omnichannel.
type ChannelDistribution struct {
	Online       int64 `json:"online"`
	Offline      int64 `json:"offline"`
	Omnichannel  int64 `json:"omnichannel"`
	Unspecified  int64 `json:"unspecified"` // Chưa mua
}

// LoyaltyDistribution phân bố theo Loyalty (CRM): core, repeat, one_time.
type LoyaltyDistribution struct {
	Core     int64 `json:"core"`
	Repeat   int64 `json:"repeat"`
	OneTime  int64 `json:"one_time"`
	Unspecified int64 `json:"unspecified"`
}

// MomentumDistribution phân bố theo Momentum (CRM): rising, stable, declining, lost.
type MomentumDistribution struct {
	Rising    int64 `json:"rising"`
	Stable    int64 `json:"stable"`
	Declining int64 `json:"declining"`
	Lost      int64 `json:"lost"`
	Unspecified int64 `json:"unspecified"`
}

// CeoGroupDistribution phân bố 6 nhóm CEO.
type CeoGroupDistribution struct {
	VipActive    int64 `json:"vip_active"`
	VipInactive  int64 `json:"vip_inactive"`
	Rising       int64 `json:"rising"`
	New          int64 `json:"new"`
	OneTime      int64 `json:"one_time"`
	Dead         int64 `json:"dead"`
}

// CustomersDashboardSnapshotData dữ liệu KPI + phân bố từ report_snapshots (CRM).
// Dùng cho GetSnapshotForCustomersDashboard.
type CustomersDashboardSnapshotData struct {
	Summary               CustomerSummary
	ValueDistribution     ValueDistribution
	JourneyDistribution   JourneyDistribution
	LifecycleDistribution LifecycleDistribution
	ChannelDistribution   ChannelDistribution
	LoyaltyDistribution   LoyaltyDistribution
	MomentumDistribution  MomentumDistribution
	CeoGroupDistribution  CeoGroupDistribution
}

// CustomersSnapshotResult kết quả trả về cho GET /dashboard/customers.
// Chỉ dùng hệ CRM (journey, value, lifecycle, channel, loyalty, momentum, ceoGroup).
type CustomersSnapshotResult struct {
	Summary                CustomerSummary        `json:"summary"`
	SummaryStatuses        map[string]string      `json:"summaryStatuses,omitempty"`
	ValueDistribution      ValueDistribution     `json:"valueDistribution"`
	JourneyDistribution    JourneyDistribution    `json:"journeyDistribution"`
	LifecycleDistribution  LifecycleDistribution  `json:"lifecycleDistribution"`
	ChannelDistribution    ChannelDistribution    `json:"channelDistribution,omitempty"`
	LoyaltyDistribution   LoyaltyDistribution    `json:"loyaltyDistribution,omitempty"`
	MomentumDistribution   MomentumDistribution   `json:"momentumDistribution,omitempty"`
	CeoGroupDistribution   CeoGroupDistribution   `json:"ceoGroupDistribution,omitempty"`
	Customers              []CustomerItem         `json:"customers"`
	VipInactiveCustomers   []VipInactiveItem      `json:"vipInactiveCustomers"`
	TotalCount             int                    `json:"totalCount,omitempty"`
	SnapshotSource         string                 `json:"snapshotSource,omitempty"`
	SnapshotPeriodKey      string                 `json:"snapshotPeriodKey,omitempty"`
	SnapshotComputedAt     int64                  `json:"snapshotComputedAt,omitempty"`
}

// CustomersTrendResult kết quả GET /dashboard/customers/trend — snapshot hiện tại + trend + comparison.
type CustomersTrendResult struct {
	CurrentSnapshot *CustomersSnapshotResult       `json:"currentSnapshot"`
	TrendData       []CustomersTrendDataItem      `json:"trendData"`
	Comparison      map[string]ComparisonItem     `json:"comparison"`
}

// CustomersTrendDataItem một mục trong trendData (snapshot theo chu kỳ).
type CustomersTrendDataItem struct {
	PeriodKey  string                 `json:"periodKey"`
	PeriodType string                 `json:"periodType"`
	Metrics    map[string]interface{} `json:"metrics"`
	ComputedAt int64                  `json:"computedAt,omitempty"`
}

// ComparisonItem so sánh KPI kỳ hiện tại vs kỳ trước.
type ComparisonItem struct {
	Current   interface{} `json:"current"`
	Previous  interface{} `json:"previous"`
	ChangePct float64     `json:"changePct"`
}

// TransitionMatrixResult kết quả GET /dashboard/customers/trend/transition-matrix.
type TransitionMatrixResult struct {
	FromPeriod      string                        `json:"fromPeriod"`
	ToPeriod        string                        `json:"toPeriod"`
	Dimension       string                        `json:"dimension"` // journey|channel|value|lifecycle|loyalty|momentum|ceoGroup
	Matrix          map[string]map[string]int64   `json:"matrix"`
	ConversionRates map[string]float64            `json:"conversionRates"`
	SankeyData      *SankeyData                   `json:"sankeyData,omitempty"`
}

// SankeyData format cho Sankey diagram (nodes + links).
type SankeyData struct {
	Nodes []SankeyNode `json:"nodes"`
	Links []SankeyLink `json:"links"`
}

// SankeyNode một node trong Sankey.
type SankeyNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// SankeyLink một link (luồng) trong Sankey.
type SankeyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int64  `json:"value"`
}

// GroupChangesResult kết quả GET /dashboard/customers/trend/group-changes.
type GroupChangesResult struct {
	FromPeriod string              `json:"fromPeriod"`
	ToPeriod   string              `json:"toPeriod"`
	Dimension  string              `json:"dimension"`
	Upgraded   []GroupChangeItem   `json:"upgraded"`
	Downgraded []GroupChangeItem   `json:"downgraded"`
	Unchanged  []GroupChangeItem   `json:"unchanged"`
}

// GroupChangeItem một nhóm chuyển đổi (fromGroup → toGroup).
type GroupChangeItem struct {
	FromGroup    string   `json:"fromGroup"`
	ToGroup      string   `json:"toGroup"`
	Count        int64    `json:"count"`
	CustomerIDs  []string `json:"customerIds,omitempty"`
}
