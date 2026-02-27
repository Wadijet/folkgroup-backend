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

// CustomerSummary KPI cho Row 1 Tab 4 (Customer Asset Dashboard).
type CustomerSummary struct {
	TotalCustomers       int64   `json:"totalCustomers"`       // Tổng khách hàng
	CustomersWithOrder    int64   `json:"customersWithOrder"`   // Số khách có ≥1 đơn
	CustomersRepeat       int64   `json:"customersRepeat"`      // Số khách có ≥2 đơn
	NewCustomersInPeriod int64   `json:"newCustomersInPeriod"` // Khách mới trong period (đơn đầu trong khoảng thời gian)
	RepeatRate           float64 `json:"repeatRate"`          // Khách ≥2 đơn / Tổng có đơn
	VipInactiveCount     int64   `json:"vipInactiveCount"`   // Số VIP inactive (Value=VIP, Lifecycle=inactive/dead)
	ReactivationValue    int64   `json:"reactivationValue"`   // Tổng trị giá tái KHO = SUM(purchased_amount) VIP inactive
	ActiveTodayCount     int64   `json:"activeTodayCount"`    // Khách có đơn trong period
	TotalLTV             float64 `json:"totalLTV"`             // Tổng giá trị tài sản = SUM(totalSpent)
	AvgLTV               float64 `json:"avgLTV"`              // Giá trị trung bình = totalLTV / totalCustomers
	VipLTV               float64 `json:"vipLTV"`              // Tổng giá trị khách VIP = SUM(totalSpent) where valueTier=vip
}

// FirstMetrics metrics dành riêng cho stage First Purchase (mua lần đầu).
// Dùng để tối ưu First → Repeat, maximize LTV.
type FirstMetrics struct {
	PurchaseQuality        string `json:"purchaseQuality"`        // high_aov|entry|medium|discount|gift_buyer — chất lượng đơn đầu
	ExperienceQuality      string `json:"experienceQuality"`      // smooth|risk|complaint — trải nghiệm (risk = có hủy/trả)
	EngagementAfterPurchase string `json:"engagementAfterPurchase"` // post_purchase_engaged|silent|negative — tương tác sau mua
	ReorderTiming          string `json:"reorderTiming"`          // within_expected|overdue|too_early — so với kỳ vọng mua lại
	RepeatProbability      string `json:"repeatProbability"`      // high|medium|low — xác suất quay lại
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
	// First Purchase Intelligence — nested theo stage (chỉ có khi journeyStage=first)
	First *FirstMetrics `json:"first,omitempty"`
	// Repeat Intelligence — nested theo stage (chỉ có khi journeyStage=repeat)
	Repeat *RepeatMetrics `json:"repeat,omitempty"`
	// VIP Intelligence — nested theo stage (chỉ có khi journeyStage=vip)
	Vip *VipMetrics `json:"vip,omitempty"`
	// Inactive Intelligence — nested khi lifecycle cooling|inactive|dead (tài sản đang chết)
	Inactive *InactiveMetrics `json:"inactive,omitempty"`
}

// RepeatMetrics metrics dành riêng cho stage Repeat (mua lại).
// Tài sản đang sinh lời — quyết định doanh thu ổn định, dòng tiền.
type RepeatMetrics struct {
	RepeatDepth        string `json:"repeatDepth"`        // R1|R2|R3|R4 — độ sâu mua lại (2, 3-4, 5-7, 8+ đơn)
	RepeatFrequency    string `json:"repeatFrequency"`    // on_track|early|delayed|overdue — nhịp độ mua
	SpendMomentum      string `json:"spendMomentum"`      // upscaling|stable|downscaling — xu hướng AOV
	ProductExpansion   string `json:"productExpansion"`   // single_category|multi_category — đa dạng sản phẩm
	EmotionalEngagement string `json:"emotionalEngagement"` // engaged_repeat|silent_repeat|transactional_repeat
	UpgradePotential   string `json:"upgradePotential"`   // high|medium|low — tiềm năng lên VIP
}

// VipMetrics metrics dành riêng cho stage VIP (tài sản chiến lược) — Lớp 3.
// Bỏ statusHealth: dùng lifecycleStage (Lớp 2) thay thế.
type VipMetrics struct {
	VipDepth         string `json:"vipDepth"`         // silver_vip|gold_vip|platinum_vip|core_patron — độ sâu quan hệ
	SpendTrend       string `json:"spendTrend"`       // upscaling_vip|stable_vip|downscaling_vip — xu hướng chi tiêu
	ProductDiversity string `json:"productDiversity"` // single_line_vip|multi_line_vip|full_portfolio_vip — đa dạng sản phẩm
	EngagementLevel  string `json:"engagementLevel"`  // engaged_vip|silent_vip|transactional_vip — mức độ tương tác
	RiskScore        string `json:"riskScore"`        // low|medium|high|critical — nguy cơ mất khách (dùng lifecycleStage nội bộ)
}

// InactiveMetrics metrics cho khách lifecycle cooling|inactive|dead (tài sản đang chết) — Lớp 3.
// Bỏ valueTier, inactiveDuration, previousBehavior, spendMomentumBefore: dùng valueTier, lifecycleStage, loyaltyStage, momentumStage (Lớp 2).
type InactiveMetrics struct {
	EngagementDrop        string `json:"engagementDrop"`        // had_post_engagement|no_engagement|dropped_engagement — chat/sale follow-up
	ReactivationPotential string `json:"reactivationPotential"` // high|medium|low — tiềm năng cứu lại (dùng L2 nội bộ)
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

// FirstLayer3Distribution phân bố Lớp 3 cho First (chỉ khách journeyStage=first).
type FirstLayer3Distribution struct {
	PurchaseQuality        map[string]int64 `json:"purchaseQuality"`
	ExperienceQuality      map[string]int64 `json:"experienceQuality"`
	EngagementAfterPurchase map[string]int64 `json:"engagementAfterPurchase"`
	ReorderTiming          map[string]int64 `json:"reorderTiming"`
	RepeatProbability      map[string]int64 `json:"repeatProbability"`
}

// RepeatLayer3Distribution phân bố Lớp 3 cho Repeat.
type RepeatLayer3Distribution struct {
	RepeatDepth         map[string]int64 `json:"repeatDepth"`
	RepeatFrequency     map[string]int64 `json:"repeatFrequency"`
	SpendMomentum       map[string]int64 `json:"spendMomentum"`
	ProductExpansion    map[string]int64 `json:"productExpansion"`
	EmotionalEngagement map[string]int64 `json:"emotionalEngagement"`
	UpgradePotential    map[string]int64 `json:"upgradePotential"`
}

// VipLayer3Distribution phân bố Lớp 3 cho VIP.
type VipLayer3Distribution struct {
	VipDepth         map[string]int64 `json:"vipDepth"`
	SpendTrend       map[string]int64 `json:"spendTrend"`
	ProductDiversity map[string]int64 `json:"productDiversity"`
	EngagementLevel  map[string]int64 `json:"engagementLevel"`
	RiskScore        map[string]int64 `json:"riskScore"`
}

// InactiveLayer3Distribution phân bố Lớp 3 cho Inactive.
type InactiveLayer3Distribution struct {
	EngagementDrop        map[string]int64 `json:"engagementDrop"`
	ReactivationPotential map[string]int64 `json:"reactivationPotential"`
}

// EngagedLayer3Distribution phân bố Lớp 3 cho Engaged (có hội thoại, chưa có đơn).
type EngagedLayer3Distribution struct {
	ConversationTemperature map[string]int64 `json:"conversationTemperature"`
	EngagementDepth         map[string]int64 `json:"engagementDepth"`
	SourceType              map[string]int64 `json:"sourceType"`
}

// ValueLTV LTV theo nhóm Value — client derive totalLTV, vipLTV, avgLTV từ đây.
type ValueLTV struct {
	Vip    float64 `json:"vip"`
	High   float64 `json:"high"`
	Medium float64 `json:"medium"`
	Low    float64 `json:"low"`
	New    float64 `json:"new"`
}

// JourneyLTV LTV theo nhóm Journey.
type JourneyLTV struct {
	Visitor  float64 `json:"visitor"`
	Engaged  float64 `json:"engaged"`
	First    float64 `json:"first"`
	Repeat   float64 `json:"repeat"`
	Vip      float64 `json:"vip"`
	Inactive float64 `json:"inactive"`
}

// LifecycleLTV LTV theo nhóm Lifecycle.
type LifecycleLTV struct {
	Active         float64 `json:"active"`
	Cooling        float64 `json:"cooling"`
	Inactive       float64 `json:"inactive"`
	Dead           float64 `json:"dead"`
	NeverPurchased float64 `json:"never_purchased"`
}

// ChannelLTV LTV theo nhóm Channel.
type ChannelLTV struct {
	Online       float64 `json:"online"`
	Offline      float64 `json:"offline"`
	Omnichannel  float64 `json:"omnichannel"`
	Unspecified  float64 `json:"unspecified"`
}

// LoyaltyLTV LTV theo nhóm Loyalty.
type LoyaltyLTV struct {
	Core        float64 `json:"core"`
	Repeat      float64 `json:"repeat"`
	OneTime     float64 `json:"one_time"`
	Unspecified float64 `json:"unspecified"`
}

// MomentumLTV LTV theo nhóm Momentum.
type MomentumLTV struct {
	Rising      float64 `json:"rising"`
	Stable      float64 `json:"stable"`
	Declining   float64 `json:"declining"`
	Lost        float64 `json:"lost"`
	Unspecified float64 `json:"unspecified"`
}

// CeoGroupLTV LTV theo 6 nhóm CEO + other.
type CeoGroupLTV struct {
	VipActive   float64 `json:"vip_active"`
	VipInactive float64 `json:"vip_inactive"`
	Rising      float64 `json:"rising"`
	New         float64 `json:"new"`
	OneTime     float64 `json:"one_time"`
	Dead        float64 `json:"dead"`
	Other       float64 `json:"other"`
}

// CustomersDashboardSnapshotData dữ liệu KPI + phân bố + LTV theo nhóm từ report_snapshots (CRM).
// Dùng cho GetSnapshotForCustomersDashboard. Client derive totalLTV, vipLTV, avgLTV từ valueLTV/journeyLTV.
type CustomersDashboardSnapshotData struct {
	Summary                CustomerSummary
	ValueDistribution      ValueDistribution
	JourneyDistribution     JourneyDistribution
	LifecycleDistribution  LifecycleDistribution
	ChannelDistribution    ChannelDistribution
	LoyaltyDistribution    LoyaltyDistribution
	MomentumDistribution   MomentumDistribution
	CeoGroupDistribution   CeoGroupDistribution
	ValueLTV               ValueLTV
	JourneyLTV             JourneyLTV
	LifecycleLTV           LifecycleLTV
	ChannelLTV             ChannelLTV
	LoyaltyLTV             LoyaltyLTV
	MomentumLTV            MomentumLTV
	CeoGroupLTV            CeoGroupLTV
	FirstLayer3            FirstLayer3Distribution    `json:"firstLayer3,omitempty"`
	RepeatLayer3           RepeatLayer3Distribution  `json:"repeatLayer3,omitempty"`
	VipLayer3              VipLayer3Distribution      `json:"vipLayer3,omitempty"`
	InactiveLayer3         InactiveLayer3Distribution `json:"inactiveLayer3,omitempty"`
	EngagedLayer3          EngagedLayer3Distribution  `json:"engagedLayer3,omitempty"`
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
	ValueLTV               ValueLTV               `json:"valueLTV,omitempty"`
	JourneyLTV             JourneyLTV            `json:"journeyLTV,omitempty"`
	LifecycleLTV            LifecycleLTV          `json:"lifecycleLTV,omitempty"`
	ChannelLTV             ChannelLTV            `json:"channelLTV,omitempty"`
	LoyaltyLTV             LoyaltyLTV            `json:"loyaltyLTV,omitempty"`
	MomentumLTV            MomentumLTV            `json:"momentumLTV,omitempty"`
	CeoGroupLTV            CeoGroupLTV            `json:"ceoGroupLTV,omitempty"`
	FirstLayer3            FirstLayer3Distribution    `json:"firstLayer3,omitempty"`
	RepeatLayer3           RepeatLayer3Distribution  `json:"repeatLayer3,omitempty"`
	VipLayer3              VipLayer3Distribution      `json:"vipLayer3,omitempty"`
	InactiveLayer3         InactiveLayer3Distribution `json:"inactiveLayer3,omitempty"`
	EngagedLayer3          EngagedLayer3Distribution  `json:"engagedLayer3,omitempty"`
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
