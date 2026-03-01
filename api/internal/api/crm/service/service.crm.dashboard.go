// Package crmvc - Dashboard aggregates: danh sách khách, CEO groups, Journey funnel, Asset matrix.
// Report module gọi các hàm này để lấy dữ liệu cho Tab 4 Customer Intelligence.
package crmvc

import (
	"context"
	"sort"
	"strings"
	"time"

	crmmodels "meta_commerce/internal/api/crm/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// CrmDashboardCustomerItem 1 dòng khách cho dashboard (format mới theo CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN).
type CrmDashboardCustomerItem struct {
	CustomerID          string   `json:"customerId"`
	Name                string   `json:"name"`
	Phone               string   `json:"phone"`
	JourneyStage        string   `json:"journeyStage"`   // visitor|engaged|first|repeat|vip|inactive
	Channel             string   `json:"channel"`       // online|offline|omnichannel (rỗng nếu chưa mua)
	ValueTier           string   `json:"valueTier"`
	LifecycleStage      string   `json:"lifecycleStage"`
	LoyaltyStage        string   `json:"loyaltyStage"`
	MomentumStage       string   `json:"momentumStage"`
	TotalSpend          float64  `json:"totalSpend"`
	OrderCount          int      `json:"orderCount"`
	RevenueLast30d      float64  `json:"revenueLast30d"`
	RevenueLast90d      float64  `json:"revenueLast90d"`
	AvgOrderValue       float64  `json:"avgOrderValue"`
	LastOrderAt         string   `json:"lastOrderAt"`
	DaysSinceLast       int64    `json:"daysSinceLast"`
	Sources             []string `json:"sources"`
	LastOrderAtMs       int64    `json:"-"` // Timestamp ms — dùng cho First/Repeat metrics
	SecondLastOrderAt   int64    `json:"-"` // Đơn thứ 2 gần nhất — dùng cho Repeat (avg days)
	LastConversationAt  int64    `json:"-"` // Dùng cho First/Repeat (engagement)
	CancelledOrderCount int      `json:"-"` // Dùng cho First (experience quality)
	OrdersLast30d       int      `json:"-"` // Số đơn 30 ngày — dùng cho Repeat (spend momentum)
	OwnedSkuCount       int      `json:"-"` // Số SKU đã mua — dùng cho Repeat (product expansion)
	TotalMessages       int      `json:"-"` // Tổng tin nhắn — dùng cho Engaged (engagement depth)
	ConversationFromAds bool     `json:"-"` // Hội thoại từ ads — dùng cho Engaged (source type)
}

// CrmDashboardFilters filter cho ListCustomersForDashboard.
// Mỗi tiêu chí có thể nhận nhiều giá trị (comma-separated), ví dụ: journey=vip,engaged,first.
type CrmDashboardFilters struct {
	Journey   []string // visitor|engaged|first|repeat|vip|inactive
	Channel   []string // online|offline|omnichannel
	ValueTier []string // vip|high|medium|low|new
	Lifecycle []string // active|cooling|inactive|dead|never_purchased
	Loyalty   []string // core|repeat|one_time
	Momentum  []string // rising|stable|declining|lost
	CeoGroup  []string // vip_active|vip_inactive|rising|new|one_time|dead
	Limit     int
	Offset    int
	// Chuẩn CRUD: sortField (daysSinceLast|totalSpend|lastOrderAt|name) + sortOrder (1=asc, -1=desc)
	SortField string
	SortOrder int // 1=tăng dần, -1=giảm dần
}

// ParseFilterValues tách chuỗi "a,b,c" thành []string{"a","b","c"}, bỏ trống.
func ParseFilterValues(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// JourneyFunnelItem 1 stage trong funnel, kèm breakdown theo channel/value/lifecycle/loyalty/momentum.
type JourneyFunnelItem struct {
	Stage      string                 `json:"stage"`
	Count      int                    `json:"count"`
	Breakdowns map[string]map[string]int `json:"breakdowns,omitempty"` // channel, value, lifecycle, loyalty, momentum
}

// JourneyFunnelResult kết quả GET /dashboard/customers/journey-funnel.
type JourneyFunnelResult struct {
	Funnel []JourneyFunnelItem `json:"funnel"`
}

// aggregationLimit dùng cho matrix/funnel/ceo — lấy toàn bộ khách để tổng = tổng số khách hàng.
const aggregationLimit = 10_000_000

// AssetMatrixResult ma trận Value × Lifecycle (hoặc Journey×L2, L2×L2).
type AssetMatrixResult struct {
	Matrix map[string]map[string]int `json:"matrix"` // row -> col -> count
	Rows   []string                 `json:"rows"`
	Cols   []string                 `json:"cols"`
	Total  int                      `json:"total"` // Tổng số khách (để verify sum(matrix)==total)
}

// ListCustomersForDashboard lấy danh sách khách từ crm_customers với filter và phân trang ở DB.
// Dùng classification đã lưu (valueTier, lifecycleStage, ...) — filter + sort + skip/limit ở MongoDB.
func (s *CrmCustomerService) ListCustomersForDashboard(ctx context.Context, ownerOrgID primitive.ObjectID, filters *CrmDashboardFilters) ([]CrmDashboardCustomerItem, int, error) {
	if filters == nil {
		filters = &CrmDashboardFilters{}
	}
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	filter := buildDashboardMongoFilter(ownerOrgID, filters)
	sortDoc := buildDashboardSortBson(filters.SortField, filters.SortOrder)

	// Tổng số match (cho pagination)
	total, err := s.Collection().CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := mongoopts.Find().
		SetSort(sortDoc).
		SetSkip(int64(filters.Offset)).
		SetLimit(int64(filters.Limit))

	all, err := s.BaseServiceMongoImpl.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	if all == nil {
		all = []crmmodels.CrmCustomer{}
	}

	var items []CrmDashboardCustomerItem
	for i := range all {
		items = append(items, s.toDashboardItem(&all[i]))
	}
	return items, int(total), nil
}

// buildDashboardMongoFilter tạo MongoDB filter từ CrmDashboardFilters (dùng classification đã lưu).
func buildDashboardMongoFilter(ownerOrgID primitive.ObjectID, f *CrmDashboardFilters) bson.M {
	filter := bson.M{"ownerOrganizationId": ownerOrgID}

	if len(f.Journey) > 0 {
		vals := make([]string, 0, len(f.Journey))
		for _, v := range f.Journey {
			if n := normalizeJourneyFilter(v); n != "" {
				vals = append(vals, n)
			}
		}
		if len(vals) > 0 {
			filter["journeyStage"] = bson.M{"$in": vals}
		}
	}
	if len(f.Channel) > 0 {
		filter["channel"] = bson.M{"$in": f.Channel}
	}
	if len(f.ValueTier) > 0 {
		filter["valueTier"] = bson.M{"$in": f.ValueTier}
	}
	if len(f.Lifecycle) > 0 {
		filter["lifecycleStage"] = bson.M{"$in": f.Lifecycle}
	}
	if len(f.Loyalty) > 0 {
		filter["loyaltyStage"] = bson.M{"$in": f.Loyalty}
	}
	if len(f.Momentum) > 0 {
		filter["momentumStage"] = bson.M{"$in": f.Momentum}
	}
	if len(f.CeoGroup) > 0 {
		ceoOr := buildCeoGroupOrConditions(f.CeoGroup)
		if len(ceoOr) > 0 {
			filter["$or"] = ceoOr
		}
	}
	return filter
}

// buildCeoGroupOrConditions tạo $or conditions cho filter CeoGroup.
func buildCeoGroupOrConditions(ceoGroups []string) []bson.M {
	var or []bson.M
	for _, g := range ceoGroups {
		var cond bson.M
		switch g {
		case "vip_active":
			cond = bson.M{"valueTier": "vip", "lifecycleStage": "active"}
		case "vip_inactive":
			cond = bson.M{"valueTier": "vip", "lifecycleStage": bson.M{"$in": []string{"inactive", "dead"}}}
		case "rising":
			cond = bson.M{"momentumStage": "rising"}
		case "new":
			cond = bson.M{"$or": []bson.M{
				{"journeyStage": "first"},
				{"valueTier": "new"},
			}}
		case "one_time":
			cond = bson.M{"loyaltyStage": "one_time"}
		case "dead":
			cond = bson.M{"lifecycleStage": "dead"}
		default:
			continue
		}
		or = append(or, cond)
	}
	return or
}

// buildDashboardSortBson tạo bson.D cho sort theo sortField và sortOrder (1=asc, -1=desc).
func buildDashboardSortBson(sortField string, sortOrder int) bson.D {
	if sortOrder == 0 {
		sortOrder = -1
	}
	dir := -1
	if sortOrder == 1 {
		dir = 1
	}

	switch sortField {
	case "totalSpend":
		return bson.D{{Key: "totalSpent", Value: dir}}
	case "lastOrderAt":
		// Đơn gần nhất trước = lastOrderAt desc
		return bson.D{{Key: "lastOrderAt", Value: -sortOrder}}
	case "name":
		return bson.D{{Key: "profile.name", Value: dir}}
	default:
		// daysSinceLast: lâu không mua trước (desc) = lastOrderAt asc; mua gần đây trước (asc) = lastOrderAt desc
		return bson.D{{Key: "lastOrderAt", Value: -sortOrder}}
	}
}

// toDashboardItem chuyển CrmCustomer sang CrmDashboardCustomerItem.
// Đọc metrics từ currentMetrics khi có; fallback top-level (backward compat).
func (s *CrmCustomerService) toDashboardItem(c *crmmodels.CrmCustomer) CrmDashboardCustomerItem {
	totalSpent := GetTotalSpentFromCustomer(c)
	orderCount := GetOrderCountFromCustomer(c)
	lastOrderAt := GetLastOrderAtFromCustomer(c)
	avgOrderValue := 0.0
	if orderCount > 0 {
		avgOrderValue = totalSpent / float64(orderCount)
	}
	daysSince := int64(-1)
	if lastOrderAt > 0 {
		daysSince = (time.Now().UnixMilli() - lastOrderAt) / (24 * 60 * 60 * 1000)
	}
	lastOrderAtStr := ""
	if lastOrderAt > 0 {
		lastOrderAtStr = time.UnixMilli(lastOrderAt).Format("2006-01-02T15:04:05")
	}

	phones := GetPhoneNumbersFromCustomer(c)
	phone := ""
	if len(phones) > 0 {
		phone = phones[0]
	}
	sources := []string{}
	if c.SourceIds.Pos != "" {
		sources = append(sources, "pos")
	}
	if c.SourceIds.Fb != "" {
		sources = append(sources, "fb")
	}

	return CrmDashboardCustomerItem{
		CustomerID:          c.UnifiedId,
		Name:                GetNameFromCustomer(c),
		Phone:               phone,
		JourneyStage:        c.JourneyStage,
		Channel:             c.Channel,
		ValueTier:           c.ValueTier,
		LifecycleStage:      c.LifecycleStage,
		LoyaltyStage:        c.LoyaltyStage,
		MomentumStage:       c.MomentumStage,
		TotalSpend:          totalSpent,
		OrderCount:          orderCount,
		RevenueLast30d:      GetFloatFromCustomer(c, "revenueLast30d"),
		RevenueLast90d:      GetFloatFromCustomer(c, "revenueLast90d"),
		AvgOrderValue:       avgOrderValue,
		LastOrderAt:         lastOrderAtStr,
		DaysSinceLast:       daysSince,
		Sources:             sources,
		LastOrderAtMs:       lastOrderAt,
		SecondLastOrderAt:   GetInt64FromCustomer(c, "secondLastOrderAt"),
		LastConversationAt:  GetInt64FromCustomer(c, "lastConversationAt"),
		CancelledOrderCount: GetIntFromCustomer(c, "cancelledOrderCount"),
		OrdersLast30d:       GetIntFromCustomer(c, "ordersLast30d"),
		OwnedSkuCount:       len(c.OwnedSkuQuantities),
		TotalMessages:       GetIntFromCustomer(c, "totalMessages"),
		ConversationFromAds: GetBoolFromCustomer(c, "conversationFromAds"),
	}
}

// matchDashboardFilters kiểm tra item có match filter không.
// Mỗi tiêu chí có thể có nhiều giá trị (OR) — item match nếu thuộc 1 trong các giá trị.
// Lưu ý: journey=omni, reactivated (stage cũ) → bỏ qua filter journey (backward compat).
func matchDashboardFilters(item CrmDashboardCustomerItem, f *CrmDashboardFilters) bool {
	if len(f.Journey) > 0 {
		itemNorm := normalizeJourneyFilter(item.JourneyStage)
		matched := false
		for _, v := range f.Journey {
			norm := normalizeJourneyFilter(v)
			if norm != "" && itemNorm == norm {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(f.Channel) > 0 && !contains(f.Channel, item.Channel) {
		return false
	}
	if len(f.ValueTier) > 0 && !contains(f.ValueTier, item.ValueTier) {
		return false
	}
	if len(f.Lifecycle) > 0 && !contains(f.Lifecycle, item.LifecycleStage) {
		return false
	}
	if len(f.Loyalty) > 0 && !contains(f.Loyalty, item.LoyaltyStage) {
		return false
	}
	if len(f.Momentum) > 0 && !contains(f.Momentum, item.MomentumStage) {
		return false
	}
	if len(f.CeoGroup) > 0 {
		matched := false
		for _, g := range f.CeoGroup {
			if matchCeoGroup(item, g) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// matchCeoGroup kiểm tra item thuộc ceoGroup (design 9: Journey=FIRST hoặc Value=New cho nhóm New).
func matchCeoGroup(item CrmDashboardCustomerItem, ceoGroup string) bool {
	switch ceoGroup {
	case "vip_active":
		return item.ValueTier == "vip" && item.LifecycleStage == "active"
	case "vip_inactive":
		return item.ValueTier == "vip" && (item.LifecycleStage == "inactive" || item.LifecycleStage == "dead")
	case "rising":
		return item.MomentumStage == "rising"
	case "new":
		return item.JourneyStage == "first" || item.ValueTier == "new"
	case "one_time":
		return item.LoyaltyStage == "one_time"
	case "dead":
		return item.LifecycleStage == "dead"
	default:
		return false
	}
}

// normalizeJourneyFilter chuẩn hóa giá trị filter (backward compat theo design 14.1).
// engaged_online→engaged, first_online/first_offline→first.
// omni, reactivated: stage cũ đã gộp vào repeat/vip — trả "" để bỏ qua filter journey.
func normalizeJourneyFilter(s string) string {
	switch s {
	case "engaged_online", "engaged":
		return "engaged"
	case "first_online", "first_offline", "first":
		return "first"
	case "visitor", "repeat", "vip", "inactive":
		return s
	case "omni", "reactivated":
		return "" // Stage cũ, filter journey bỏ qua
	default:
		return s
	}
}

// sortDashboardItems sắp xếp theo field và order (chuẩn CRUD: 1=asc, -1=desc).
func sortDashboardItems(items []CrmDashboardCustomerItem, field string, order int) {
	if order == 0 {
		order = -1
	}
	asc := order == 1

	// cmpDaysSince: daysSinceLast — order=-1 (desc) = lâu không mua trước (lớn trước)
	cmpDaysSince := func(i, j int) bool {
		if items[i].DaysSinceLast < 0 && items[j].DaysSinceLast < 0 {
			return (items[i].Name < items[j].Name) == asc
		}
		if items[i].DaysSinceLast < 0 {
			return false
		}
		if items[j].DaysSinceLast < 0 {
			return true
		}
		if asc {
			return items[i].DaysSinceLast < items[j].DaysSinceLast
		}
		return items[i].DaysSinceLast > items[j].DaysSinceLast
	}
	// cmpLastOrder: lastOrderAt — order=-1 (desc) = đơn gần nhất trước = daysSince nhỏ trước
	cmpLastOrder := func(i, j int) bool {
		if items[i].DaysSinceLast < 0 && items[j].DaysSinceLast < 0 {
			return (items[i].Name < items[j].Name) == asc
		}
		if items[i].DaysSinceLast < 0 {
			return false
		}
		if items[j].DaysSinceLast < 0 {
			return true
		}
		if asc {
			return items[i].DaysSinceLast > items[j].DaysSinceLast
		}
		return items[i].DaysSinceLast < items[j].DaysSinceLast
	}

	switch field {
	case "totalSpend":
		if asc {
			sort.Slice(items, func(i, j int) bool { return items[i].TotalSpend < items[j].TotalSpend })
		} else {
			sort.Slice(items, func(i, j int) bool { return items[i].TotalSpend > items[j].TotalSpend })
		}
	case "lastOrderAt":
		sort.Slice(items, cmpLastOrder)
	case "name":
		if asc {
			sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		} else {
			sort.Slice(items, func(i, j int) bool { return items[i].Name > items[j].Name })
		}
	default:
		// daysSinceLast
		sort.Slice(items, cmpDaysSince)
	}
}

// GetJourneyFunnel trả về số lượng từng stage Journey, mỗi stage có breakdown theo value/lifecycle/loyalty/momentum.
func (s *CrmCustomerService) GetJourneyFunnel(ctx context.Context, ownerOrgID primitive.ObjectID) (*JourneyFunnelResult, error) {
	items, _, err := s.ListCustomersForDashboard(ctx, ownerOrgID, &CrmDashboardFilters{Limit: aggregationLimit})
	if err != nil {
		return nil, err
	}

	stageOrder := []string{"visitor", "engaged", "first", "repeat", "vip", "inactive"}

	// Gom khách theo stage
	byStage := make(map[string][]CrmDashboardCustomerItem)
	for _, stage := range stageOrder {
		byStage[stage] = nil
	}
	for _, it := range items {
		byStage[it.JourneyStage] = append(byStage[it.JourneyStage], it)
	}

	funnel := make([]JourneyFunnelItem, 0, len(stageOrder))
	for _, stage := range stageOrder {
		stageItems := byStage[stage]
		item := JourneyFunnelItem{Stage: stage, Count: len(stageItems)}
		if len(stageItems) > 0 {
			item.Breakdowns = buildJourneyStageBreakdowns(stageItems)
		}
		funnel = append(funnel, item)
	}
	return &JourneyFunnelResult{Funnel: funnel}, nil
}

// buildJourneyStageBreakdowns tạo breakdown channel/value/lifecycle/loyalty/momentum cho 1 stage.
func buildJourneyStageBreakdowns(items []CrmDashboardCustomerItem) map[string]map[string]int {
	channelOrder := []string{"online", "offline", "omnichannel"}
	valueOrder := []string{"vip", "high", "medium", "low", "new"}
	lifecycleOrder := []string{"active", "cooling", "inactive", "dead", "never_purchased"}
	loyaltyOrder := []string{"core", "repeat", "one_time"}
	momentumOrder := []string{"rising", "stable", "declining", "lost"}

	channel := make(map[string]int)
	value := make(map[string]int)
	lifecycle := make(map[string]int)
	loyalty := make(map[string]int)
	momentum := make(map[string]int)
	for _, v := range channelOrder {
		channel[v] = 0
	}
	channel[""] = 0 // chưa mua
	for _, v := range valueOrder {
		value[v] = 0
	}
	for _, v := range lifecycleOrder {
		lifecycle[v] = 0
	}
	for _, v := range loyaltyOrder {
		loyalty[v] = 0
	}
	for _, v := range momentumOrder {
		momentum[v] = 0
	}

	for _, it := range items {
		if _, ok := channel[it.Channel]; ok {
			channel[it.Channel]++
		} else {
			channel[it.Channel] = 1
		}
		if _, ok := value[it.ValueTier]; ok {
			value[it.ValueTier]++
		} else {
			value[it.ValueTier] = 1
		}
		if _, ok := lifecycle[it.LifecycleStage]; ok {
			lifecycle[it.LifecycleStage]++
		} else {
			lifecycle[it.LifecycleStage] = 1
		}
		if it.LoyaltyStage != "" {
			if _, ok := loyalty[it.LoyaltyStage]; ok {
				loyalty[it.LoyaltyStage]++
			} else {
				loyalty[it.LoyaltyStage] = 1
			}
		}
		if _, ok := momentum[it.MomentumStage]; ok {
			momentum[it.MomentumStage]++
		} else {
			momentum[it.MomentumStage] = 1
		}
	}

	return map[string]map[string]int{
		"channel":   channel,
		"value":     value,
		"lifecycle": lifecycle,
		"loyalty":   loyalty,
		"momentum":  momentum,
	}
}

// GetAssetMatrix trả về ma trận Value × Lifecycle.
func (s *CrmCustomerService) GetAssetMatrix(ctx context.Context, ownerOrgID primitive.ObjectID) (*AssetMatrixResult, error) {
	items, _, err := s.ListCustomersForDashboard(ctx, ownerOrgID, &CrmDashboardFilters{Limit: aggregationLimit})
	if err != nil {
		return nil, err
	}

	rows := []string{"vip", "high", "medium", "low", "new"}
	cols := []string{"active", "cooling", "inactive", "dead", "never_purchased"}

	matrix := make(map[string]map[string]int)
	for _, r := range rows {
		matrix[r] = make(map[string]int)
		for _, c := range cols {
			matrix[r][c] = 0
		}
	}

	for _, it := range items {
		vt := it.ValueTier
		lc := it.LifecycleStage
		if matrix[vt] == nil {
			matrix[vt] = make(map[string]int)
		}
		matrix[vt][lc]++
	}

	return &AssetMatrixResult{
		Matrix: matrix,
		Rows:   rows,
		Cols:   cols,
		Total:  len(items),
	}, nil
}

// L2 axis orders cho ma trận (channel, value, lifecycle, loyalty, momentum).
var (
	l2ChannelOrder   = []string{"online", "offline", "omnichannel", ""}
	l2ValueOrder     = []string{"vip", "high", "medium", "low", "new"}
	l2LifecycleOrder = []string{"active", "cooling", "inactive", "dead", "never_purchased"}
	l2LoyaltyOrder   = []string{"core", "repeat", "one_time", ""}
	l2MomentumOrder  = []string{"rising", "stable", "declining", "lost", ""}
)

// getL2Cols trả về thứ tự cột cho trục L2.
func getL2Cols(axis string) []string {
	switch axis {
	case "channel":
		return l2ChannelOrder
	case "value":
		return l2ValueOrder
	case "lifecycle":
		return l2LifecycleOrder
	case "loyalty":
		return l2LoyaltyOrder
	case "momentum":
		return l2MomentumOrder
	default:
		return l2ValueOrder
	}
}

// getL2Value lấy giá trị L2 từ item theo axis.
func getL2Value(item CrmDashboardCustomerItem, axis string) string {
	switch axis {
	case "channel":
		return item.Channel
	case "value":
		return item.ValueTier
	case "lifecycle":
		return item.LifecycleStage
	case "loyalty":
		return item.LoyaltyStage
	case "momentum":
		return item.MomentumStage
	default:
		return item.ValueTier
	}
}

// GetMatrixJourneyValue trả về ma trận Journey × L2 (cols=axis: channel|value|lifecycle|loyalty|momentum).
// Tổng matrix = tổng số khách hàng.
func (s *CrmCustomerService) GetMatrixJourneyValue(ctx context.Context, ownerOrgID primitive.ObjectID, colsAxis string) (*AssetMatrixResult, error) {
	items, _, err := s.ListCustomersForDashboard(ctx, ownerOrgID, &CrmDashboardFilters{Limit: aggregationLimit})
	if err != nil {
		return nil, err
	}
	rows := []string{"visitor", "engaged", "first", "repeat", "vip", "inactive"}
	if colsAxis == "" {
		colsAxis = "value"
	}
	cols := getL2Cols(colsAxis)

	matrix := make(map[string]map[string]int)
	for _, r := range rows {
		matrix[r] = make(map[string]int)
		for _, c := range cols {
			matrix[r][c] = 0
		}
	}
	for _, it := range items {
		row := it.JourneyStage
		col := getL2Value(it, colsAxis)
		if matrix[row] == nil {
			matrix[row] = make(map[string]int)
		}
		matrix[row][col]++
	}
	return &AssetMatrixResult{Matrix: matrix, Rows: rows, Cols: cols, Total: len(items)}, nil
}

// GetMatrixValueLoyalty trả về ma trận L2 × L2 (rows=rowAxis, cols=colAxis).
// Tổng matrix = tổng số khách hàng.
func (s *CrmCustomerService) GetMatrixValueLoyalty(ctx context.Context, ownerOrgID primitive.ObjectID, rowAxis, colAxis string) (*AssetMatrixResult, error) {
	items, _, err := s.ListCustomersForDashboard(ctx, ownerOrgID, &CrmDashboardFilters{Limit: aggregationLimit})
	if err != nil {
		return nil, err
	}
	rows := getL2Cols(rowAxis)
	cols := getL2Cols(colAxis)
	if rowAxis == "" {
		rows = l2ValueOrder
	}
	if colAxis == "" {
		cols = l2LoyaltyOrder
	}

	matrix := make(map[string]map[string]int)
	for _, r := range rows {
		matrix[r] = make(map[string]int)
		for _, c := range cols {
			matrix[r][c] = 0
		}
	}
	for _, it := range items {
		row := getL2Value(it, rowAxis)
		col := getL2Value(it, colAxis)
		if matrix[row] == nil {
			matrix[row] = make(map[string]int)
		}
		matrix[row][col]++
	}
	return &AssetMatrixResult{Matrix: matrix, Rows: rows, Cols: cols, Total: len(items)}, nil
}

