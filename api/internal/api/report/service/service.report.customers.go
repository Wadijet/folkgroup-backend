// Package reportsvc - Customer Intelligence (Tab 4): snapshot chất lượng tài sản khách hàng.
// KPI, phân bố tier, lifecycle, bảng khách, panel VIP inactive.
// Data source: pc_pos_customers, pc_pos_orders (aggregate).
package reportsvc

import (
	"context"
	"fmt"
	"sort"
	"time"

	reportdto "meta_commerce/internal/api/report/dto"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetCustomersSnapshot trả về snapshot Tab 4 Customer Intelligence.
// Bao gồm: 6 KPI, tier distribution, lifecycle distribution, bảng customers, VIP inactive panel.
func (s *ReportService) GetCustomersSnapshot(ctx context.Context, ownerOrganizationID primitive.ObjectID, params *reportdto.CustomersQueryParams) (*reportdto.CustomersSnapshotResult, error) {
	if params == nil {
		params = &reportdto.CustomersQueryParams{}
	}
	applyCustomersDefaults(params)

	fromTime, toTime, err := parseCustomersPeriod(params)
	if err != nil {
		return nil, fmt.Errorf("parse period: %w", err)
	}

	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	todayEnd := todayStart.Add(24*time.Hour - time.Second)
	todayStartSec := todayStart.Unix()
	todayEndSec := todayEnd.Unix()
	nowSec := now.Unix()

	// 1. Load customers
	customers, err := s.loadCustomersForIntelligence(ctx, ownerOrganizationID)
	if err != nil {
		return nil, err
	}

	// 2. Aggregate từ orders: first_order_at, last_order_at, purchased_amount, order_count (status 2,3,16)
	orderAgg, err := s.aggregateCustomerOrders(ctx, ownerOrganizationID)
	if err != nil {
		return nil, err
	}

	// 3. Build customer items với tier, lifecycle
	var items []reportdto.CustomerItem
	tierDist := reportdto.TierDistribution{}
	lifecycleDist := reportdto.LifecycleDistribution{}
	var totalCustomers, newInPeriod, customersWithOrder, customersRepeat int64
	var vipInactiveCount int64
	var reactivationValue float64
	var activeToday int64
	var vipInactiveList []reportdto.CustomerItem

	for _, c := range customers {
		agg := orderAgg[c.CustomerId]
		orderCount := c.OrderCount
		purchasedAmount := c.TotalSpent
		lastOrderAt := c.LastOrderAt
		firstOrderAt := int64(0)
		assignedSale := extractCustomerAssignedSale(c.PosData)

		if agg != nil {
			if agg.OrderCount > 0 {
				orderCount = agg.OrderCount
				purchasedAmount = agg.PurchasedAmount
				lastOrderAt = agg.LastOrderAt
				firstOrderAt = agg.FirstOrderAt
			}
			if assignedSale == "" && agg.AssignedSale != "" {
				assignedSale = agg.AssignedSale
			}
		}

		// Tier từ order_count
		tier := computeTier(orderCount)
		switch tier {
		case "new":
			tierDist.New++
		case "silver":
			tierDist.Silver++
		case "gold":
			tierDist.Gold++
		case "platinum":
			tierDist.Platinum++
		}

		// Days since last
		daysSince := int64(-1)
		if lastOrderAt > 0 {
			daysSince = (nowSec - lastOrderAt) / 86400
		}

		// Lifecycle
		lifecycle := computeLifecycle(tier, daysSince, params.ActiveDays, params.CoolingDays, params.InactiveDays)
		switch lifecycle {
		case "active":
			lifecycleDist.Active++
		case "cooling":
			lifecycleDist.Cooling++
		case "inactive":
			lifecycleDist.Inactive++
		case "vip_inactive":
			lifecycleDist.VipInactive++
		case "never_purchased":
			lifecycleDist.NeverPurchased++
		}

		// KPI counts
		totalCustomers++
		if orderCount >= 1 {
			customersWithOrder++
		}
		if orderCount >= 2 {
			customersRepeat++
		}
		if firstOrderAt > 0 && firstOrderAt >= fromTime.Unix() && firstOrderAt <= toTime.Unix() {
			newInPeriod++
		}
		if lifecycle == "vip_inactive" {
			vipInactiveCount++
			reactivationValue += purchasedAmount
			vipInactiveList = append(vipInactiveList, reportdto.CustomerItem{
				CustomerID:       c.CustomerId,
				Name:             c.Name,
				Phone:            getCustomerPhone(c),
				Tier:             tier,
				TotalSpend:       purchasedAmount,
				OrderCount:       orderCount,
				LastOrderAt:      formatUnixToISO(lastOrderAt),
				DaysSinceLast:    daysSince,
				Lifecycle:        lifecycle,
				AssignedSale:     assignedSale,
				Tags:             extractCustomerTags(c.PosData),
			})
		}
		if lastOrderAt >= todayStartSec && lastOrderAt <= todayEndSec {
			activeToday++
		}

		items = append(items, reportdto.CustomerItem{
			CustomerID:    c.CustomerId,
			Name:          c.Name,
			Phone:         getCustomerPhone(c),
			Tier:          tier,
			TotalSpend:   purchasedAmount,
			OrderCount:    orderCount,
			LastOrderAt:   formatUnixToISO(lastOrderAt),
			DaysSinceLast: daysSince,
			Lifecycle:     lifecycle,
			AssignedSale:  assignedSale,
			Tags:          extractCustomerTags(c.PosData),
		})
	}

	// Repeat rate
	repeatRate := float64(0)
	if customersWithOrder > 0 {
		repeatRate = float64(customersRepeat) / float64(customersWithOrder)
	}

	// VIP inactive panel - sort by totalSpend desc, limit
	sort.Slice(vipInactiveList, func(i, j int) bool {
		return vipInactiveList[i].TotalSpend > vipInactiveList[j].TotalSpend
	})
	vipLimit := params.VipInactiveLimit
	if vipLimit <= 0 {
		vipLimit = 15
	}
	if vipLimit > 20 {
		vipLimit = 20
	}
	var vipInactiveCustomers []reportdto.VipInactiveItem
	for i, it := range vipInactiveList {
		if i >= vipLimit {
			break
		}
		vipInactiveCustomers = append(vipInactiveCustomers, reportdto.VipInactiveItem{
			CustomerID:    it.CustomerID,
			Name:          it.Name,
			TotalSpend:    it.TotalSpend,
			DaysSinceLast: it.DaysSinceLast,
			AssignedSale:  it.AssignedSale,
		})
	}

	// Filter items
	items = filterCustomerItems(items, params.Filter)
	// Sort theo chuẩn CRUD (sortField + sortOrder: 1=asc, -1=desc)
	sortField, sortOrder := reportdto.ParseCustomerSortParams(params.SortField, params.SortOrder)
	sortCustomerItems(items, sortField, sortOrder)
	// Paginate
	items = paginateCustomerItems(items, params.Offset, params.Limit)

	summaryStatuses := computeCustomerSummaryStatuses(totalCustomers, newInPeriod, repeatRate, vipInactiveCount, int64(reactivationValue), activeToday)

	return &reportdto.CustomersSnapshotResult{
		Summary: reportdto.CustomerSummary{
			TotalCustomers:       totalCustomers,
			NewCustomersInPeriod: newInPeriod,
			RepeatRate:           repeatRate,
			VipInactiveCount:     vipInactiveCount,
			ReactivationValue:    int64(reactivationValue),
			ActiveTodayCount:     activeToday,
		},
		SummaryStatuses:       summaryStatuses,
		TierDistribution:      tierDist,
		LifecycleDistribution: lifecycleDist,
		Customers:             items,
		VipInactiveCustomers:  vipInactiveCustomers,
	}, nil
}

// customerIntelligenceData dữ liệu customer đã parse.
type customerIntelligenceData struct {
	CustomerId   string
	Name         string
	PhoneNumbers []string
	OrderCount   int64
	TotalSpent   float64
	LastOrderAt  int64
	PosData      map[string]interface{}
}

// customerOrderAgg kết quả aggregate từ orders theo customerId.
type customerOrderAgg struct {
	FirstOrderAt   int64
	LastOrderAt    int64
	OrderCount     int64
	PurchasedAmount float64
	AssignedSale   string
}

func (s *ReportService) loadCustomersForIntelligence(ctx context.Context, ownerOrgID primitive.ObjectID) ([]customerIntelligenceData, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCustomers)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosCustomers, common.ErrNotFound)
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	opts := options.Find().SetProjection(bson.M{
		"customerId": 1, "name": 1, "phoneNumbers": 1, "totalSpent": 1, "totalOrder": 1, "succeedOrderCount": 1,
		"lastOrderAt": 1, "posData": 1,
	})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var result []customerIntelligenceData
	for cursor.Next(ctx) {
		var doc struct {
			CustomerId        string                 `bson:"customerId"`
			Name              string                 `bson:"name"`
			PhoneNumbers      []string               `bson:"phoneNumbers"`
			TotalSpent        float64                `bson:"totalSpent"`
			TotalOrder        int64                  `bson:"totalOrder"`
			SucceedOrderCount int64                  `bson:"succeedOrderCount"`
			LastOrderAt       int64                  `bson:"lastOrderAt"`
			PosData           map[string]interface{} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		orderCount := doc.SucceedOrderCount
		if orderCount <= 0 {
			orderCount = doc.TotalOrder
		}
		if orderCount <= 0 {
			if oc := getInt64FromMap(doc.PosData, "order_count", "succeed_order_count"); oc != nil {
				orderCount = *oc
			}
		}
		lastOrderAt := doc.LastOrderAt
		if lastOrderAt <= 0 {
			if lo := getInt64FromMap(doc.PosData, "last_order_at"); lo != nil {
				lastOrderAt = *lo
			}
		}
		totalSpent := doc.TotalSpent
		if totalSpent <= 0 {
			totalSpent = getFloatFromMap(doc.PosData, "purchased_amount")
		}
		result = append(result, customerIntelligenceData{
			CustomerId:   doc.CustomerId,
			Name:         doc.Name,
			PhoneNumbers: doc.PhoneNumbers,
			OrderCount:   orderCount,
			TotalSpent:   totalSpent,
			LastOrderAt:  lastOrderAt,
			PosData:      doc.PosData,
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	if result == nil {
		result = []customerIntelligenceData{}
	}
	return result, nil
}

func (s *ReportService) aggregateCustomerOrders(ctx context.Context, ownerOrgID primitive.ObjectID) (map[string]*customerOrderAgg, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return make(map[string]*customerOrderAgg), nil
	}

	// Chỉ đơn hoàn thành: status 2, 3, 16
	completedStatuses := []int{2, 3, 16}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"$or": []bson.M{
				{"posData.status": bson.M{"$in": completedStatuses}},
				{"status": bson.M{"$in": completedStatuses}},
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{"$ifNull": bson.A{"$customerId", bson.M{"$ifNull": []interface{}{"$posData.customer.id", ""}}}},
			"firstOrderAt":   bson.M{"$min": bson.M{"$ifNull": []interface{}{"$insertedAt", "$posCreatedAt"}}},
			"lastOrderAt":    bson.M{"$max": bson.M{"$ifNull": []interface{}{"$insertedAt", "$posCreatedAt"}}},
			"orderCount":     bson.M{"$sum": 1},
			"purchasedAmount": bson.M{"$sum": bson.M{"$ifNull": []interface{}{"$posData.total_price_after_sub_discount", 0}}},
			"assignedSale":   bson.M{"$last": "$posData.assigning_seller.name"},
		}}},
	}
	cursor, err := coll.Aggregate(ctx, pipe)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	result := make(map[string]*customerOrderAgg)
	for cursor.Next(ctx) {
		var doc struct {
			ID              interface{} `bson:"_id"`
			FirstOrderAt    int64       `bson:"firstOrderAt"`
			LastOrderAt     int64       `bson:"lastOrderAt"`
			OrderCount      int64       `bson:"orderCount"`
			PurchasedAmount float64     `bson:"purchasedAmount"`
			AssignedSale    string      `bson:"assignedSale"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		customerId := normalizeCustomerID(doc.ID)
		if customerId == "" {
			continue
		}
		result[customerId] = &customerOrderAgg{
			FirstOrderAt:    doc.FirstOrderAt,
			LastOrderAt:     doc.LastOrderAt,
			OrderCount:      doc.OrderCount,
			PurchasedAmount: doc.PurchasedAmount,
			AssignedSale:    doc.AssignedSale,
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return result, nil
}

// normalizeCustomerID chuẩn hóa customer ID từ aggregation _id (có thể string hoặc số).
func normalizeCustomerID(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case float64:
		return fmt.Sprintf("%.0f", x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func computeTier(orderCount int64) string {
	if orderCount <= 0 {
		return "new"
	}
	if orderCount == 1 {
		return "new"
	}
	if orderCount <= 4 {
		return "silver"
	}
	if orderCount <= 9 {
		return "gold"
	}
	return "platinum"
}

func computeLifecycle(tier string, daysSince int64, activeDays, coolingDays, inactiveDays int) string {
	if daysSince < 0 {
		return "never_purchased"
	}
	// VIP inactive: Gold hoặc Platinum và days > inactiveDays
	if (tier == "gold" || tier == "platinum") && daysSince > int64(inactiveDays) {
		return "vip_inactive"
	}
	if daysSince <= int64(activeDays) {
		return "active"
	}
	if daysSince <= int64(coolingDays) {
		return "cooling"
	}
	// Inactive: 60–90 ngày; với non-VIP > 90 cũng coi là inactive
	return "inactive"
}

func getCustomerPhone(c customerIntelligenceData) string {
	if len(c.PhoneNumbers) > 0 && c.PhoneNumbers[0] != "" {
		return c.PhoneNumbers[0]
	}
	if c.PosData != nil {
		if arr, ok := c.PosData["phone_numbers"].([]interface{}); ok && len(arr) > 0 {
			if s, ok := arr[0].(string); ok {
				return s
			}
		}
	}
	return ""
}

func extractCustomerAssignedSale(posData map[string]interface{}) string {
	if posData == nil {
		return ""
	}
	if seller, ok := posData["assigning_seller"].(map[string]interface{}); ok {
		if n, ok := seller["name"].(string); ok && n != "" {
			return n
		}
	}
	return ""
}

func extractCustomerTags(posData map[string]interface{}) []string {
	if posData == nil {
		return nil
	}
	arr, ok := posData["tags"].([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, t := range arr {
		if m, ok := t.(map[string]interface{}); ok {
			if txt, ok := m["text"].(string); ok && txt != "" {
				out = append(out, txt)
			}
		}
	}
	return out
}

func formatUnixToISO(sec int64) string {
	if sec <= 0 {
		return ""
	}
	return time.Unix(sec, 0).Format("2006-01-02T15:04:05")
}

func filterCustomerItems(items []reportdto.CustomerItem, filter string) []reportdto.CustomerItem {
	if filter == "" || filter == "all" {
		return items
	}
	var out []reportdto.CustomerItem
	for _, it := range items {
		if filter == "vip_inactive" && it.Lifecycle == "vip_inactive" {
			out = append(out, it)
		} else if filter == "inactive" && it.Lifecycle == "inactive" {
			out = append(out, it)
		} else if filter == "cooling" && it.Lifecycle == "cooling" {
			out = append(out, it)
		} else if filter == "active" && it.Lifecycle == "active" {
			out = append(out, it)
		} else if filter == "tier_new" && it.Tier == "new" {
			out = append(out, it)
		} else if filter == "tier_silver" && it.Tier == "silver" {
			out = append(out, it)
		} else if filter == "tier_gold" && it.Tier == "gold" {
			out = append(out, it)
		} else if filter == "tier_platinum" && it.Tier == "platinum" {
			out = append(out, it)
		}
	}
	return out
}

// sortCustomerItems sắp xếp theo field và order (chuẩn CRUD: 1=asc, -1=desc).
func sortCustomerItems(items []reportdto.CustomerItem, field string, order int) {
	if order == 0 {
		order = -1
	}
	asc := order == 1

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
		sort.Slice(items, cmpDaysSince)
	}
}

func paginateCustomerItems(items []reportdto.CustomerItem, offset, limit int) []reportdto.CustomerItem {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []reportdto.CustomerItem{}
	}
	toIdx := offset + limit
	if toIdx > len(items) {
		toIdx = len(items)
	}
	return items[offset:toIdx]
}

func applyCustomersDefaults(p *reportdto.CustomersQueryParams) {
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.Limit > 2000 {
		p.Limit = 2000
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	if p.Period == "" {
		p.Period = "month"
	}
	if p.Filter == "" {
		p.Filter = "all"
	}
	if p.SortField == "" {
		p.SortField = "daysSinceLast"
	}
	if p.SortOrder != 1 && p.SortOrder != -1 {
		p.SortOrder = -1
	}
	if p.VipInactiveLimit <= 0 {
		p.VipInactiveLimit = 15
	}
	if p.VipInactiveLimit > 20 {
		p.VipInactiveLimit = 20
	}
	if p.ActiveDays <= 0 {
		p.ActiveDays = 30
	}
	if p.CoolingDays <= 0 {
		p.CoolingDays = 60
	}
	if p.InactiveDays <= 0 {
		p.InactiveDays = 90
	}
}

func parseCustomersPeriod(p *reportdto.CustomersQueryParams) (from, to time.Time, err error) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	if p.Period == "custom" && p.From != "" && p.To != "" {
		from, err = time.ParseInLocation(reportdto.ReportDateFormat, p.From, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("from không đúng định dạng dd-mm-yyyy: %w", err)
		}
		to, err = time.ParseInLocation(reportdto.ReportDateFormat, p.To, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("to không đúng định dạng dd-mm-yyyy: %w", err)
		}
		if from.After(to) {
			return time.Time{}, time.Time{}, fmt.Errorf("from phải nhỏ hơn hoặc bằng to")
		}
		return from, to, nil
	}

	switch p.Period {
	case "day":
		from = today
		to = today.Add(24*time.Hour - time.Second)
	case "week":
		from = today.AddDate(0, 0, -7)
		to = today
	case "60d":
		from = today.AddDate(0, 0, -60)
		to = today
	case "90d":
		from = today.AddDate(0, 0, -90)
		to = today
	case "year":
		from = today.AddDate(0, 0, -365)
		to = today
	default:
		from = today.AddDate(0, 0, -30)
		to = today
	}
	return from, to, nil
}

func computeCustomerSummaryStatuses(totalCustomers, newInPeriod int64, repeatRate float64, vipInactiveCount, reactivationValue, activeToday int64) map[string]string {
	st := make(map[string]string)
	if totalCustomers > 0 {
		st["totalCustomers"] = "green"
	} else {
		st["totalCustomers"] = "yellow"
	}
	st["newCustomersInPeriod"] = "green"
	if repeatRate >= 0.3 {
		st["repeatRate"] = "green"
	} else if repeatRate >= 0.15 {
		st["repeatRate"] = "yellow"
	} else {
		st["repeatRate"] = "red"
	}
	if vipInactiveCount == 0 {
		st["vipInactiveCount"] = "green"
	} else if vipInactiveCount <= 10 {
		st["vipInactiveCount"] = "yellow"
	} else {
		st["vipInactiveCount"] = "red"
	}
	st["reactivationValue"] = "green"
	st["activeTodayCount"] = "green"
	return st
}
