// Package reportsvc - API trend cho Tab 4 Customer: GET /dashboard/customers/trend, transition-matrix, group-changes.
// Dùng metricsSnapshot từ crm_activity_history (hệ CRM).
package reportsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	crmvc "meta_commerce/internal/api/crm/service"
	reportdto "meta_commerce/internal/api/report/dto"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// customerStateAtPeriod CRM trạng thái khách tại cuối chu kỳ (valueTier, lifecycleStage, journeyStage, ...).
type customerStateAtPeriod struct {
	ValueTier     string
	LifecycleStage string
	JourneyStage  string
	Channel       string
	LoyaltyStage  string
	MomentumStage string
	CeoGroup      string
}

// GetCustomersTrendWithComparison trả về snapshot hiện tại + trend data + comparison (KPI % change vs kỳ trước).
// CurrentSnapshot: KPI + distributions từ report_snapshots; Customers và VipInactiveCustomers để trống (Handler sẽ fill từ CrmCustomerService).
func (s *ReportService) GetCustomersTrendWithComparison(ctx context.Context, ownerOrgID primitive.ObjectID, params *reportdto.CustomersQueryParams) (*reportdto.CustomersTrendResult, error) {
	if params == nil {
		params = &reportdto.CustomersQueryParams{}
	}
	applyCustomersDefaults(params)

	// 1. Snapshot: phát sinh (ceoGroupIn/Out) từ report_snapshots; Summary, CeoGroupDistribution từ CRM/API (Handler merge)
	snapData, snapPeriodKey, snapComputedAt, _ := s.GetSnapshotForCustomersDashboard(ctx, ownerOrgID, params)
	currentSnapshot := &reportdto.CustomersSnapshotResult{
		Customers: nil, VipInactiveCustomers: nil, TotalCount: 0,
		SnapshotSource: "realtime", SnapshotPeriodKey: snapPeriodKey, SnapshotComputedAt: snapComputedAt,
	}
	if snapData != nil {
		currentSnapshot.CeoGroupIn = snapData.CeoGroupIn
		currentSnapshot.CeoGroupOut = snapData.CeoGroupOut
		currentSnapshot.SnapshotSource = "report_snapshots"
	}
	if endMs, err := s.GetEndMsForCustomersParams(params); err == nil {
		startMs, _ := s.GetStartMsForCustomersParams(params)
		if balance, err := s.GetPeriodEndBalance(ctx, ownerOrgID, endMs, startMs); err == nil {
			currentSnapshot.CeoGroupDistribution = CeoGroupDistributionFromBalance(balance)
		}
	}

	// 2. Xác định reportKey và from/to cho trend
	reportKey, fromStr, toStr, err := paramsToTrendRange(params)
	if err != nil {
		return nil, err
	}

	// 3. Lấy trend data từ report_snapshots
	snapshots, err := s.FindSnapshotsForTrend(ctx, reportKey, ownerOrgID, fromStr, toStr)
	if err != nil {
		return nil, fmt.Errorf("find snapshots: %w", err)
	}

	trendData := make([]reportdto.CustomersTrendDataItem, 0, len(snapshots))
	for _, snap := range snapshots {
		trendData = append(trendData, reportdto.CustomersTrendDataItem{
			PeriodKey:  snap.PeriodKey,
			PeriodType: snap.PeriodType,
			Metrics:    snap.Metrics,
			ComputedAt: snap.ComputedAt,
		})
	}

	// 4. Comparison: kỳ hiện tại (cuối) vs kỳ trước
	comparison := make(map[string]reportdto.ComparisonItem)
	if len(snapshots) >= 2 {
		curr := snapshots[len(snapshots)-1].Metrics
		prev := snapshots[len(snapshots)-2].Metrics
		comparison = buildComparison(curr, prev)
	}

	return &reportdto.CustomersTrendResult{
		CurrentSnapshot: currentSnapshot,
		TrendData:       trendData,
		Comparison:     comparison,
	}, nil
}

// paramsToTrendRange chuyển params (period, from, to) thành reportKey và from/to string cho query.
func paramsToTrendRange(params *reportdto.CustomersQueryParams) (reportKey, fromStr, toStr string, err error) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)

	var from, to time.Time
	if params.Period == "custom" && params.From != "" && params.To != "" {
		from, err = time.ParseInLocation(reportdto.ReportDateFormat, params.From, loc)
		if err != nil {
			return "", "", "", fmt.Errorf("from không đúng định dạng dd-mm-yyyy: %w", err)
		}
		to, err = time.ParseInLocation(reportdto.ReportDateFormat, params.To, loc)
		if err != nil {
			return "", "", "", fmt.Errorf("to không đúng định dạng dd-mm-yyyy: %w", err)
		}
		days := int(to.Sub(from).Hours() / 24)
		if days <= 31 {
			reportKey = "customer_daily"
		} else if days <= 93 {
			reportKey = "customer_weekly"
		} else {
			reportKey = "customer_monthly"
		}
	} else {
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		switch params.Period {
		case "day":
			from = today.AddDate(0, 0, -7)
			to = today
			reportKey = "customer_daily"
		case "week":
			from = today.AddDate(0, 0, -28)
			to = today
			reportKey = "customer_weekly"
		case "60d":
			from = today.AddDate(0, 0, -60)
			to = today
			reportKey = "customer_daily"
		case "90d":
			from = today.AddDate(0, 0, -90)
			to = today
			reportKey = "customer_daily"
		case "year":
			from = today.AddDate(0, 0, -365)
			to = today
			reportKey = "customer_monthly"
		default:
			from = today.AddDate(0, 0, -60)
			to = today
			reportKey = "customer_monthly"
		}
	}

	switch reportKey {
	case "customer_daily":
		fromStr = from.Format("2006-01-02")
		toStr = to.Format("2006-01-02")
	case "customer_weekly":
		fromStr = getMondayOfWeek(from).Format("2006-01-02")
		toStr = getMondayOfWeek(to).Format("2006-01-02")
	case "customer_monthly":
		fromStr = from.Format("2006-01")
		toStr = to.Format("2006-01")
	case "customer_yearly":
		fromStr = from.Format("2006")
		toStr = to.Format("2006")
	default:
		fromStr = from.Format("2006-01-02")
		toStr = to.Format("2006-01-02")
	}
	return reportKey, fromStr, toStr, nil
}

func getMondayOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

func buildComparison(curr, prev map[string]interface{}) map[string]reportdto.ComparisonItem {
	out := make(map[string]reportdto.ComparisonItem)
	// Snapshot phát sinh: cấu trúc raw/layer1/layer2/layer3 (giống metricsSnapshot trong customer activity).
	// Cấu trúc mới: mỗi metric có in/out trong cùng nhóm — path = raw.totalCustomers.in, layer2.valueTier.vip.in
	pairs := []struct{ nested, flat string }{
		{"raw.totalCustomers.in", "totalCustomersIn"},
		{"raw.totalCustomers.out", "totalCustomersOut"},
		{"raw.newCustomersInPeriod.in", "newCustomersInPeriod"},
		{"raw.activeInPeriod.in", "activeInPeriod"},
		{"raw.reactivationValue.in", "reactivationValueIn"},
		{"raw.reactivationValue.out", "reactivationValueOut"},
		{"layer2.valueTier.vip.in", "value_in_vip"},
		{"layer2.valueTier.high.in", "value_in_high"},
		{"layer2.valueTier.medium.in", "value_in_medium"},
		{"layer2.valueTier.low.in", "value_in_low"},
		{"layer2.valueTier.new.in", "value_in_new"},
		{"layer2.valueTier.vip.out", "value_out_vip"},
		{"layer2.valueTier.high.out", "value_out_high"},
		{"layer1.journeyStage.visitor.in", "journey_in_visitor"},
		{"layer1.journeyStage.engaged.in", "journey_in_engaged"},
		{"layer1.journeyStage.first.in", "journey_in_first"},
		{"layer1.journeyStage.repeat.in", "journey_in_repeat"},
		{"layer1.journeyStage.vip.in", "journey_in_vip"},
		{"layer1.journeyStage.inactive.in", "journey_in_inactive"},
		{"layer2.lifecycleStage.active.in", "lifecycle_in_active"},
		{"layer2.lifecycleStage.cooling.in", "lifecycle_in_cooling"},
		{"layer2.lifecycleStage.inactive.in", "lifecycle_in_inactive"},
		{"layer2.lifecycleStage.dead.in", "lifecycle_in_dead"},
		{"layer2.ceoGroup.vip_active.in", "ceo_in_vip_active"},
		{"layer2.ceoGroup.vip_inactive.in", "ceo_in_vip_inactive"},
		{"layer2.ceoGroup.rising.in", "ceo_in_rising"},
		{"layer2.ceoGroup.new.in", "ceo_in_new"},
		{"layer2.ceoGroup.one_time.in", "ceo_in_one_time"},
		{"layer2.ceoGroup.dead.in", "ceo_in_dead"},
		{"layer2.ceoGroup.vip_active.out", "ceo_out_vip_active"},
		{"layer2.ceoGroup.vip_inactive.out", "ceo_out_vip_inactive"},
		{"layer2.ceoGroup.rising.out", "ceo_out_rising"},
		{"layer2.ceoGroup.new.out", "ceo_out_new"},
		{"layer2.ceoGroup.one_time.out", "ceo_out_one_time"},
		{"layer2.ceoGroup.dead.out", "ceo_out_dead"},
		{"layer2.valueTierLTV.vip.in", "valueLTV_in_vip"},
		{"layer2.valueTierLTV.high.in", "valueLTV_in_high"},
		{"layer2.valueTierLTV.vip.out", "valueLTV_out_vip"},
		{"layer2.ceoGroupLTV.vip_active.in", "ceoGroupLTV_in_vip_active"},
		{"layer2.ceoGroupLTV.vip_inactive.in", "ceoGroupLTV_in_vip_inactive"},
		{"layer2.ceoGroupLTV.vip_active.out", "ceoGroupLTV_out_vip_active"},
		{"layer2.ceoGroupLTV.vip_inactive.out", "ceoGroupLTV_out_vip_inactive"},
	}
	for _, p := range pairs {
		cv := getMetricValue(curr, p.nested, p.flat)
		pv := getMetricValue(prev, p.nested, p.flat)
		changePct := 0.0
		if pv != 0 {
			changePct = (cv - pv) / pv * 100
		} else if cv != 0 {
			changePct = 100
		}
		out[p.flat] = reportdto.ComparisonItem{Current: cv, Previous: pv, ChangePct: changePct}
	}
	return out
}

// getMetricValue lấy float64 từ nested path (vd: "summary.in.totalCustomers") hoặc flat key.
func getMetricValue(m map[string]interface{}, nestedPath, flatKey string) float64 {
	if m == nil {
		return 0
	}
	// Thử nested: "summary.in.totalCustomers" -> m["summary"]["in"]["totalCustomers"]
	parts := strings.Split(nestedPath, ".")
	cur := m
	for i := 0; i < len(parts)-1 && cur != nil; i++ {
		if g, ok := cur[parts[i]].(map[string]interface{}); ok {
			cur = g
		} else {
			return 0
		}
	}
	if len(parts) > 0 && cur != nil {
		return getFloatOrInt64(cur[parts[len(parts)-1]])
	}
	return getFloatOrInt64(m[flatKey])
}

func getFloatOrInt64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float32:
		return float64(x)
	default:
		return 0
	}
}

// GetTransitionMatrix tính ma trận chuyển đổi giữa 2 chu kỳ theo dimension CRM.
// dimension: journey|channel|value|lifecycle|loyalty|momentum|ceoGroup
// periodType: day|week|month|year — xác định cách parse periodKey (vd: weekly thì periodKey là thứ Hai, endMs = cuối Chủ nhật).
//
// Cách đếm: Với mỗi unifiedId:
//   1. getCustomerStateMapForPeriod(fromPeriod, periodType) → trạng thái khách tại cuối fromPeriod (snapshot cuối trước endMs)
//   2. getCustomerStateMapForPeriod(toPeriod, periodType) → trạng thái khách tại cuối toPeriod
//   3. Chỉ đếm khách CÓ Ở CẢ HAI period (có trong cả stateFrom và stateTo)
//   4. matrix[fromGroup][toGroup]++ nếu khách chuyển từ fromGroup sang toGroup
// Khách chỉ có ở from (rời bỏ) hoặc chỉ có ở to (mới) không nằm trong matrix.
func (s *ReportService) GetTransitionMatrix(ctx context.Context, ownerOrgID primitive.ObjectID, fromPeriod, toPeriod, dimension, periodType string, includeSankey bool) (*reportdto.TransitionMatrixResult, error) {
	stateFrom, _, err := s.getCustomerStateMapForPeriod(ctx, ownerOrgID, fromPeriod, periodType)
	if err != nil {
		return nil, fmt.Errorf("get state from: %w", err)
	}
	stateTo, _, err := s.getCustomerStateMapForPeriod(ctx, ownerOrgID, toPeriod, periodType)
	if err != nil {
		return nil, fmt.Errorf("get state to: %w", err)
	}

	// Lấy fromGroup, toGroup theo dimension
	getGroup := func(st customerStateAtPeriod) string {
		switch dimension {
		case "value": return st.ValueTier
		case "lifecycle": return st.LifecycleStage
		case "journey": return st.JourneyStage
		case "channel": return st.Channel
		case "loyalty": return st.LoyaltyStage
		case "momentum": return st.MomentumStage
		case "ceoGroup": return st.CeoGroup
		default: return st.ValueTier
		}
	}

	matrix := make(map[string]map[string]int64)

	// Đếm chuyển đổi: khách có ở cả 2 period
	for custID, stFrom := range stateFrom {
		stTo, ok := stateTo[custID]
		if !ok {
			continue
		}
		fromGroup := getGroup(stFrom)
		toGroup := getGroup(stTo)
		if fromGroup == "" { fromGroup = "_unspecified" }
		if toGroup == "" { toGroup = "_unspecified" }
		if matrix[fromGroup] == nil {
			matrix[fromGroup] = make(map[string]int64)
		}
		matrix[fromGroup][toGroup]++
	}

	// Conversion rates: từ fromGroup → toGroup = count / total(fromGroup)
	conversionRates := make(map[string]float64)
	for fromG, row := range matrix {
		total := int64(0)
		for _, c := range row {
			total += c
		}
		if total == 0 {
			continue
		}
		for toG, c := range row {
			if c > 0 {
				key := fromG + "_to_" + toG
				conversionRates[key] = float64(c) / float64(total) * 100
			}
		}
	}

	result := &reportdto.TransitionMatrixResult{
		FromPeriod:      fromPeriod,
		ToPeriod:        toPeriod,
		Dimension:       dimension,
		Matrix:          matrix,
		ConversionRates: conversionRates,
	}

	if includeSankey {
		result.SankeyData = buildSankeyData(matrix, dimension)
	}

	return result, nil
}

// getCustomerStateMapForPeriod trả về map[unifiedId]state tại cuối chu kỳ — lấy từ metricsSnapshot trong crm_activity_history.
// periodType: day|week|month|year — khi "week", periodKey YYYY-MM-DD là thứ Hai, endMs = cuối Chủ nhật; khi "day" là cuối ngày đó.
func (s *ReportService) getCustomerStateMapForPeriod(ctx context.Context, ownerOrgID primitive.ObjectID, periodKey, periodType string) (map[string]customerStateAtPeriod, int64, error) {
	loc, _ := time.LoadLocation(ReportTimezone)
	var endSec int64
	if len(periodKey) == 10 {
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return nil, 0, fmt.Errorf("parse periodKey %s: %w", periodKey, err)
		}
		if periodType == "week" {
			// Chuẩn hóa về thứ Hai: periodKey có thể được truyền sai (vd: thứ Ba), cần lấy thứ Hai của tuần đó.
			monday := getMondayOfWeek(t)
			// endMs = cuối Chủ nhật (thứ Hai + 7 ngày - 1 giây)
			endSec = monday.AddDate(0, 0, 7).Unix() - 1
		} else {
			endSec = t.AddDate(0, 0, 1).Unix() - 1
		}
	} else if len(periodKey) == 7 {
		t, err := time.ParseInLocation("2006-01", periodKey, loc)
		if err != nil {
			return nil, 0, fmt.Errorf("parse periodKey %s: %w", periodKey, err)
		}
		endSec = t.AddDate(0, 1, 0).Add(-time.Second).Unix()
	} else if len(periodKey) == 4 {
		t, err := time.ParseInLocation("2006", periodKey, loc)
		if err != nil {
			return nil, 0, fmt.Errorf("parse periodKey %s: %w", periodKey, err)
		}
		endSec = t.AddDate(1, 0, 0).Add(-time.Second).Unix()
	} else {
		return nil, 0, fmt.Errorf("periodKey không hợp lệ: %s", periodKey)
	}
	endMs := endSec*1000 + 999

	actSvc, err := crmvc.NewCrmActivityService()
	if err != nil {
		return nil, 0, fmt.Errorf("tạo CrmActivityService: %w", err)
	}
	snapshotMap, err := actSvc.GetLastSnapshotPerCustomerBeforeEndMs(ctx, ownerOrgID, endMs)
	if err != nil {
		return nil, 0, err
	}

	result := make(map[string]customerStateAtPeriod)
	for unifiedId, m := range snapshotMap {
		valueTier := crmvc.GetStrFromNestedMetrics(m, "valueTier")
		lifecycleStage := crmvc.GetStrFromNestedMetrics(m, "lifecycleStage")
		journeyStage := crmvc.GetStrFromNestedMetrics(m, "journeyStage")
		channel := crmvc.GetStrFromNestedMetrics(m, "channel")
		loyaltyStage := crmvc.GetStrFromNestedMetrics(m, "loyaltyStage")
		momentumStage := crmvc.GetStrFromNestedMetrics(m, "momentumStage")
		// Chuẩn hóa rỗng: dùng _unspecified, không tự gán ý nghĩa nghiệp vụ (đọc từ metricsSnapshot).
		if valueTier == "" { valueTier = "_unspecified" }
		if lifecycleStage == "" { lifecycleStage = "_unspecified" }
		if journeyStage == "" { journeyStage = "_unspecified" }
		if channel == "" { channel = "_unspecified" }
		if loyaltyStage == "" { loyaltyStage = "_unspecified" }
		if momentumStage == "" { momentumStage = "_unspecified" }
		ceoGroup := computeCeoGroup(valueTier, lifecycleStage, journeyStage, loyaltyStage, momentumStage)
		result[unifiedId] = customerStateAtPeriod{
			ValueTier:      valueTier,
			LifecycleStage: lifecycleStage,
			JourneyStage:   journeyStage,
			Channel:        channel,
			LoyaltyStage:   loyaltyStage,
			MomentumStage:  momentumStage,
			CeoGroup:       ceoGroup,
		}
	}
	return result, endMs, nil
}

func computeCeoGroup(valueTier, lifecycleStage, journeyStage, loyaltyStage, momentumStage string) string {
	if valueTier == "vip" && lifecycleStage == "active" { return "vip_active" }
	if valueTier == "vip" && (lifecycleStage == "inactive" || lifecycleStage == "dead") { return "vip_inactive" }
	if momentumStage == "rising" { return "rising" }
	if journeyStage == "first" || valueTier == "new" { return "new" }
	if loyaltyStage == "one_time" { return "one_time" }
	if lifecycleStage == "dead" { return "dead" }
	return "_other"
}

func buildSankeyData(matrix map[string]map[string]int64, dimension string) *reportdto.SankeyData {
	var nodes []reportdto.SankeyNode
	var links []reportdto.SankeyLink
	seen := make(map[string]bool)
	for fromG, row := range matrix {
		fromID := fromG + "_from"
		if !seen[fromID] {
			nodes = append(nodes, reportdto.SankeyNode{ID: fromID, Label: fromG + " (T)"})
			seen[fromID] = true
		}
		for toG, val := range row {
			if val <= 0 {
				continue
			}
			toID := toG + "_to"
			if !seen[toID] {
				nodes = append(nodes, reportdto.SankeyNode{ID: toID, Label: toG + " (T+1)"})
				seen[toID] = true
			}
			links = append(links, reportdto.SankeyLink{Source: fromID, Target: toID, Value: val})
		}
	}
	return &reportdto.SankeyData{Nodes: nodes, Links: links}
}

// GetGroupChanges trả về chi tiết khách chuyển nhóm (upgraded, downgraded, unchanged).
// dimension: journey|channel|value|lifecycle|loyalty|momentum|ceoGroup
// periodType: day|week|month|year — xác định cách parse periodKey.
func (s *ReportService) GetGroupChanges(ctx context.Context, ownerOrgID primitive.ObjectID, fromPeriod, toPeriod, dimension, periodType string) (*reportdto.GroupChangesResult, error) {
	stateFrom, _, err := s.getCustomerStateMapForPeriod(ctx, ownerOrgID, fromPeriod, periodType)
	if err != nil {
		return nil, err
	}
	stateTo, _, err := s.getCustomerStateMapForPeriod(ctx, ownerOrgID, toPeriod, periodType)
	if err != nil {
		return nil, err
	}

	getGroup := func(st customerStateAtPeriod) string {
		switch dimension {
		case "value": return st.ValueTier
		case "lifecycle": return st.LifecycleStage
		case "journey": return st.JourneyStage
		case "channel": return st.Channel
		case "loyalty": return st.LoyaltyStage
		case "momentum": return st.MomentumStage
		case "ceoGroup": return st.CeoGroup
		default: return st.ValueTier
		}
	}
	// Thứ tự nhóm cho xác định upgrade/downgrade (số càng cao càng tốt)
	valueOrder := map[string]int{"new": 0, "low": 1, "medium": 2, "high": 3, "vip": 4, "_unspecified": -1}
	lifecycleOrder := map[string]int{"never_purchased": 0, "dead": 1, "inactive": 2, "cooling": 3, "active": 4, "_unspecified": -1}
	journeyOrder := map[string]int{"visitor": 0, "engaged": 1, "first": 2, "repeat": 3, "vip": 4, "inactive": 0, "_unspecified": -1}
	ceoOrder := map[string]int{"_other": 0, "dead": 0, "one_time": 1, "new": 2, "rising": 3, "vip_inactive": 2, "vip_active": 4}
	defaultOrder := map[string]int{"_unspecified": -1, "_other": 0}

	var upgraded, downgraded, unchanged []reportdto.GroupChangeItem
	upgradeCounts := make(map[string]map[string]int64)
	downgradeCounts := make(map[string]map[string]int64)
	unchangedCounts := make(map[string]int64)

	var order map[string]int
	switch dimension {
	case "value": order = valueOrder
	case "lifecycle": order = lifecycleOrder
	case "journey": order = journeyOrder
	case "ceoGroup": order = ceoOrder
	default: order = defaultOrder
	}

	for custID, stFrom := range stateFrom {
		stTo, ok := stateTo[custID]
		if !ok {
			continue
		}
		fromGroup := getGroup(stFrom)
		toGroup := getGroup(stTo)
		if fromGroup == "" { fromGroup = "_unspecified" }
		if toGroup == "" { toGroup = "_unspecified" }

		fromOrd, _ := order[fromGroup]
		toOrd, _ := order[toGroup]
		if toOrd > fromOrd {
			if upgradeCounts[fromGroup] == nil {
				upgradeCounts[fromGroup] = make(map[string]int64)
			}
			upgradeCounts[fromGroup][toGroup]++
		} else if toOrd < fromOrd {
			if downgradeCounts[fromGroup] == nil {
				downgradeCounts[fromGroup] = make(map[string]int64)
			}
			downgradeCounts[fromGroup][toGroup]++
		} else {
			unchangedCounts[fromGroup]++
		}
	}

	for fromG, row := range upgradeCounts {
		for toG, cnt := range row {
			upgraded = append(upgraded, reportdto.GroupChangeItem{FromGroup: fromG, ToGroup: toG, Count: cnt})
		}
	}
	for fromG, row := range downgradeCounts {
		for toG, cnt := range row {
			downgraded = append(downgraded, reportdto.GroupChangeItem{FromGroup: fromG, ToGroup: toG, Count: cnt})
		}
	}
	for g, cnt := range unchangedCounts {
		if cnt > 0 {
			unchanged = append(unchanged, reportdto.GroupChangeItem{FromGroup: g, ToGroup: g, Count: cnt})
		}
	}

	return &reportdto.GroupChangesResult{
		FromPeriod: fromPeriod,
		ToPeriod:   toPeriod,
		Dimension:  dimension,
		Upgraded:   upgraded,
		Downgraded: downgraded,
		Unchanged:  unchanged,
	}, nil
}
