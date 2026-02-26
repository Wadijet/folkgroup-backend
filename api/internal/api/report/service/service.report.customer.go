// Package reportsvc - Compute engine cho báo cáo khách hàng theo chu kỳ (customer_daily, customer_weekly, customer_monthly, customer_yearly).
// Snapshot = trạng thái khách tại cuối chu kỳ — lấy từ metricsSnapshot trong crm_activity_history, lưu vào report_snapshots.
// Chỉ dùng hệ CRM (journey, value, lifecycle, channel, loyalty, momentum, ceoGroup).
package reportsvc

import (
	"context"
	"fmt"
	"time"

	crmvc "meta_commerce/internal/api/crm/service"
	reportdto "meta_commerce/internal/api/report/dto"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ComputeCustomerReport tính snapshot khách hàng tại cuối chu kỳ và upsert vào report_snapshots.
// reportKey: customer_daily | customer_weekly | customer_monthly | customer_yearly
func (s *ReportService) ComputeCustomerReport(ctx context.Context, reportKey, periodKey string, ownerOrganizationID primitive.ObjectID) error {
	def, err := s.LoadDefinition(ctx, reportKey)
	if err != nil {
		return fmt.Errorf("load report definition: %w", err)
	}

	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return fmt.Errorf("load timezone %s: %w", ReportTimezone, err)
	}

	var startSec, endSec int64
	switch def.PeriodType {
	case "day":
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return fmt.Errorf("parse periodKey %s: %w", periodKey, err)
		}
		startSec = t.Unix()
		endSec = t.AddDate(0, 0, 1).Unix() - 1
	case "week":
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return fmt.Errorf("parse periodKey %s: %w", periodKey, err)
		}
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		startSec = monday.Unix()
		endSec = monday.AddDate(0, 0, 7).Unix() - 1
	case "month":
		t, err := time.ParseInLocation("2006-01", periodKey, loc)
		if err != nil {
			return fmt.Errorf("parse periodKey %s: %w", periodKey, err)
		}
		startSec = t.Unix()
		endSec = t.AddDate(0, 1, 0).Add(-time.Second).Unix()
	case "year":
		t, err := time.ParseInLocation("2006", periodKey, loc)
		if err != nil {
			return fmt.Errorf("parse periodKey %s: %w", periodKey, err)
		}
		startSec = t.Unix()
		endSec = t.AddDate(1, 0, 0).Add(-time.Second).Unix()
	default:
		return fmt.Errorf("periodType %s chưa hỗ trợ cho customer report", def.PeriodType)
	}

	endMs := endSec*1000 + 999
	startMs := startSec * 1000

	metrics, err := s.computeCustomerMetricsFromActivityHistory(ctx, ownerOrganizationID, startMs, endMs, startSec, endSec)
	if err != nil {
		return fmt.Errorf("compute customer metrics từ activity history: %w", err)
	}

	return s.upsertSnapshot(ctx, reportKey, periodKey, def.PeriodType, ownerOrganizationID, metrics)
}

// computeCustomerMetricsFromActivityHistory tính KPI và phân bố từ metricsSnapshot trong crm_activity_history.
// Lấy snapshot cuối của mỗi khách trước endMs, đếm theo valueTier, lifecycleStage, journeyStage, ...
func (s *ReportService) computeCustomerMetricsFromActivityHistory(ctx context.Context, ownerOrgID primitive.ObjectID, startMs, endMs, startSec, endSec int64) (map[string]interface{}, error) {
	actSvc, err := crmvc.NewCrmActivityService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmActivityService: %w", err)
	}

	snapshotMap, err := actSvc.GetLastSnapshotPerCustomerBeforeEndMs(ctx, ownerOrgID, endMs)
	if err != nil {
		return nil, err
	}

	// Đếm theo CRM dimensions
	valueDist := make(map[string]int64)
	journeyDist := make(map[string]int64)
	lifecycleDist := make(map[string]int64)
	channelDist := make(map[string]int64)
	loyaltyDist := make(map[string]int64)
	momentumDist := make(map[string]int64)
	ceoDist := make(map[string]int64)

	totalCustomers := int64(0)
	customersWithOrder := int64(0)
	customersRepeat := int64(0)
	newInPeriod := int64(0)
	vipInactiveCount := int64(0)
	reactivationValue := 0.0
	activeInPeriod := int64(0)

	for _, m := range snapshotMap {
		valueTier := getStrFromSnapshotMap(m, "valueTier")
		lifecycleStage := getStrFromSnapshotMap(m, "lifecycleStage")
		journeyStage := getStrFromSnapshotMap(m, "journeyStage")
		channel := getStrFromSnapshotMap(m, "channel")
		loyaltyStage := getStrFromSnapshotMap(m, "loyaltyStage")
		momentumStage := getStrFromSnapshotMap(m, "momentumStage")
		orderCount := getIntFromSnapshotMap(m, "orderCount")
		totalSpent := getFloatFromSnapshotMap(m, "totalSpent")
		lastOrderAt := getInt64FromSnapshotMap(m, "lastOrderAt")

		// Chuẩn hóa rỗng
		if valueTier == "" {
			valueTier = "new"
		}
		if lifecycleStage == "" {
			lifecycleStage = "never_purchased"
		}
		if journeyStage == "" {
			journeyStage = "visitor"
		}
		if channel == "" {
			channel = "_unspecified"
		}
		if loyaltyStage == "" {
			loyaltyStage = "_unspecified"
		}
		if momentumStage == "" {
			momentumStage = "_unspecified"
		}

		valueDist[valueTier]++
		journeyDist[journeyStage]++
		lifecycleDist[lifecycleStage]++
		channelDist[channel]++
		loyaltyDist[loyaltyStage]++
		momentumDist[momentumStage]++

		// CEO groups
		if valueTier == "vip" && lifecycleStage == "active" {
			ceoDist["vip_active"]++
		}
		if valueTier == "vip" && (lifecycleStage == "inactive" || lifecycleStage == "dead") {
			ceoDist["vip_inactive"]++
			vipInactiveCount++
			reactivationValue += totalSpent
		}
		if momentumStage == "rising" {
			ceoDist["rising"]++
		}
		if journeyStage == "first" || valueTier == "new" {
			ceoDist["new"]++
		}
		if loyaltyStage == "one_time" {
			ceoDist["one_time"]++
		}
		if lifecycleStage == "dead" {
			ceoDist["dead"]++
		}

		totalCustomers++
		if orderCount >= 1 {
			customersWithOrder++
		}
		if orderCount >= 2 {
			customersRepeat++
		}

		// newInPeriod: orderCount=1 và lastOrderAt trong period (gần đúng cho khách mới)
		if orderCount == 1 && lastOrderAt > 0 {
			lastSec := lastOrderAt
			if lastSec > 1e12 {
				lastSec = lastSec / 1000
			}
			if lastSec >= startSec && lastSec <= endSec {
				newInPeriod++
			}
		}

		// activeInPeriod: có đơn trong period (lastOrderAt trong khoảng)
		if lastOrderAt > 0 {
			lastSec := lastOrderAt
			if lastSec > 1e12 {
				lastSec = lastSec / 1000
			}
			if lastSec >= startSec && lastSec <= endSec {
				activeInPeriod++
			}
		}
	}

	repeatRate := 0.0
	if customersWithOrder > 0 {
		repeatRate = float64(customersRepeat) / float64(customersWithOrder)
	}

	// Cấu trúc nested — dễ đọc, dễ mở rộng
	metrics := map[string]interface{}{
		"summary": map[string]interface{}{
			"totalCustomers":       totalCustomers,
			"newCustomersInPeriod": newInPeriod,
			"repeatRate":           repeatRate,
			"vipInactiveCount":     vipInactiveCount,
			"reactivationValue":    int64(reactivationValue),
			"activeInPeriod":       activeInPeriod,
		},
		"valueDistribution": map[string]interface{}{
			"vip": valueDist["vip"], "high": valueDist["high"], "medium": valueDist["medium"],
			"low": valueDist["low"], "new": valueDist["new"],
		},
		"journeyDistribution": map[string]interface{}{
			"visitor": journeyDist["visitor"], "engaged": journeyDist["engaged"], "first": journeyDist["first"],
			"repeat": journeyDist["repeat"], "vip": journeyDist["vip"], "inactive": journeyDist["inactive"],
		},
		"lifecycleDistribution": map[string]interface{}{
			"active": lifecycleDist["active"], "cooling": lifecycleDist["cooling"], "inactive": lifecycleDist["inactive"],
			"dead": lifecycleDist["dead"], "never_purchased": lifecycleDist["never_purchased"],
		},
		"channelDistribution": map[string]interface{}{
			"online": channelDist["online"], "offline": channelDist["offline"],
			"omnichannel": channelDist["omnichannel"], "unspecified": channelDist["_unspecified"],
		},
		"loyaltyDistribution": map[string]interface{}{
			"core": loyaltyDist["core"], "repeat": loyaltyDist["repeat"],
			"one_time": loyaltyDist["one_time"], "unspecified": loyaltyDist["_unspecified"],
		},
		"momentumDistribution": map[string]interface{}{
			"rising": momentumDist["rising"], "stable": momentumDist["stable"], "declining": momentumDist["declining"],
			"lost": momentumDist["lost"], "unspecified": momentumDist["_unspecified"],
		},
		"ceoGroupDistribution": map[string]interface{}{
			"vip_active": ceoDist["vip_active"], "vip_inactive": ceoDist["vip_inactive"], "rising": ceoDist["rising"],
			"new": ceoDist["new"], "one_time": ceoDist["one_time"], "dead": ceoDist["dead"],
		},
	}
	return metrics, nil
}

func getStrFromSnapshotMap(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getIntFromSnapshotMap(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case int32:
		return int(x)
	}
	return 0
}

func getInt64FromSnapshotMap(m map[string]interface{}, key string) int64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func getFloatFromSnapshotMap(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

// GetSnapshotForCustomersDashboard lấy KPI và phân bố từ report_snapshots cho period cuối (hệ CRM).
// Trả về *CustomersDashboardSnapshotData hoặc nil nếu không có snapshot — fallback sang real-time.
func (s *ReportService) GetSnapshotForCustomersDashboard(ctx context.Context, ownerOrgID primitive.ObjectID, params *reportdto.CustomersQueryParams) (*reportdto.CustomersDashboardSnapshotData, string, int64, error) {
	if params == nil {
		params = &reportdto.CustomersQueryParams{}
	}
	applyCustomersDefaults(params)

	reportKey, _, periodKey, err := paramsToTrendRange(params) // periodKey = toStr (chu kỳ cuối)
	if err != nil {
		return nil, "", 0, err
	}

	snap, err := s.GetReportSnapshot(ctx, reportKey, periodKey, ownerOrgID)
	if err != nil {
		return nil, "", 0, err
	}
	if snap == nil || snap.Metrics == nil {
		return nil, "", 0, nil
	}

	m := snap.Metrics
	data := &reportdto.CustomersDashboardSnapshotData{
		Summary: reportdto.CustomerSummary{
			TotalCustomers:       metricAt(m, "summary", "totalCustomers", "totalCustomers"),
			NewCustomersInPeriod: metricAt(m, "summary", "newCustomersInPeriod", "newCustomersInPeriod"),
			RepeatRate:           metricAtFloat(m, "summary", "repeatRate", "repeatRate"),
			VipInactiveCount:     metricAt(m, "summary", "vipInactiveCount", "vipInactiveCount"),
			ReactivationValue:    metricAt(m, "summary", "reactivationValue", "reactivationValue"),
			ActiveTodayCount:     metricAt(m, "summary", "activeInPeriod", "activeInPeriod"),
		},
		ValueDistribution: reportdto.ValueDistribution{
			Vip:    metricAtDist(m, "valueDistribution", "value", "vip"),
			High:   metricAtDist(m, "valueDistribution", "value", "high"),
			Medium: metricAtDist(m, "valueDistribution", "value", "medium"),
			Low:    metricAtDist(m, "valueDistribution", "value", "low"),
			New:    metricAtDist(m, "valueDistribution", "value", "new"),
		},
		JourneyDistribution: reportdto.JourneyDistribution{
			Visitor:  metricAtDist(m, "journeyDistribution", "journey", "visitor"),
			Engaged:  metricAtDist(m, "journeyDistribution", "journey", "engaged"),
			First:    metricAtDist(m, "journeyDistribution", "journey", "first"),
			Repeat:   metricAtDist(m, "journeyDistribution", "journey", "repeat"),
			Vip:      metricAtDist(m, "journeyDistribution", "journey", "vip"),
			Inactive: metricAtDist(m, "journeyDistribution", "journey", "inactive"),
		},
		LifecycleDistribution: reportdto.LifecycleDistribution{
			Active:         metricAtDist(m, "lifecycleDistribution", "lifecycle", "active"),
			Cooling:        metricAtDist(m, "lifecycleDistribution", "lifecycle", "cooling"),
			Inactive:       metricAtDist(m, "lifecycleDistribution", "lifecycle", "inactive"),
			Dead:           metricAtDist(m, "lifecycleDistribution", "lifecycle", "dead"),
			NeverPurchased: metricAtDist(m, "lifecycleDistribution", "lifecycle", "never_purchased"),
		},
		ChannelDistribution: reportdto.ChannelDistribution{
			Online:      metricAtDist(m, "channelDistribution", "channel", "online"),
			Offline:     metricAtDist(m, "channelDistribution", "channel", "offline"),
			Omnichannel: metricAtDist(m, "channelDistribution", "channel", "omnichannel"),
			Unspecified: metricAtDist(m, "channelDistribution", "channel", "unspecified"),
		},
		LoyaltyDistribution: reportdto.LoyaltyDistribution{
			Core:        metricAtDist(m, "loyaltyDistribution", "loyalty", "core"),
			Repeat:     metricAtDist(m, "loyaltyDistribution", "loyalty", "repeat"),
			OneTime:    metricAtDist(m, "loyaltyDistribution", "loyalty", "one_time"),
			Unspecified: metricAtDist(m, "loyaltyDistribution", "loyalty", "unspecified"),
		},
		MomentumDistribution: reportdto.MomentumDistribution{
			Rising:      metricAtDist(m, "momentumDistribution", "momentum", "rising"),
			Stable:     metricAtDist(m, "momentumDistribution", "momentum", "stable"),
			Declining:  metricAtDist(m, "momentumDistribution", "momentum", "declining"),
			Lost:       metricAtDist(m, "momentumDistribution", "momentum", "lost"),
			Unspecified: metricAtDist(m, "momentumDistribution", "momentum", "unspecified"),
		},
		CeoGroupDistribution: reportdto.CeoGroupDistribution{
			VipActive:   metricAtDist(m, "ceoGroupDistribution", "ceo", "vip_active"),
			VipInactive: metricAtDist(m, "ceoGroupDistribution", "ceo", "vip_inactive"),
			Rising:      metricAtDist(m, "ceoGroupDistribution", "ceo", "rising"),
			New:         metricAtDist(m, "ceoGroupDistribution", "ceo", "new"),
			OneTime:     metricAtDist(m, "ceoGroupDistribution", "ceo", "one_time"),
			Dead:        metricAtDist(m, "ceoGroupDistribution", "ceo", "dead"),
		},
	}
	return data, periodKey, snap.ComputedAt, nil
}

// metricAt lấy int64 từ nested (m[group][key]) hoặc flat (m[flatKey]) để backward compat.
func metricAt(m map[string]interface{}, group, key, flatKey string) int64 {
	if g, ok := m[group].(map[string]interface{}); ok {
		return snapshotMetricToInt64(g[key])
	}
	return snapshotMetricToInt64(m[flatKey])
}

// metricAtFloat lấy float64 từ nested hoặc flat.
func metricAtFloat(m map[string]interface{}, group, key, flatKey string) float64 {
	if g, ok := m[group].(map[string]interface{}); ok {
		return snapshotMetricToFloat64(g[key])
	}
	return snapshotMetricToFloat64(m[flatKey])
}

// metricAtDist lấy int64 từ distribution nested (m[distKey][key]) hoặc flat (m[flatPrefix_key]) để backward compat.
func metricAtDist(m map[string]interface{}, distKey, flatPrefix, key string) int64 {
	if g, ok := m[distKey].(map[string]interface{}); ok {
		return snapshotMetricToInt64(g[key])
	}
	return snapshotMetricToInt64(m[flatPrefix+"_"+key])
}

// snapshotMetricToInt64 chuyển giá trị metric từ snapshot sang int64 (an toàn).
func snapshotMetricToInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case float32:
		return int64(x)
	default:
		return 0
	}
}

// snapshotMetricToFloat64 chuyển giá trị metric từ snapshot sang float64 (an toàn).
func snapshotMetricToFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case float32:
		return float64(x)
	default:
		return 0
	}
}
