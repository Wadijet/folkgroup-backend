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
	"meta_commerce/internal/api/report/layer3"

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

	// LTV theo từng nhóm (7 dimensions) — client derive totalLTV, vipLTV, avgLTV từ đây
	valueLTV := make(map[string]float64)
	journeyLTV := make(map[string]float64)
	lifecycleLTV := make(map[string]float64)
	channelLTV := make(map[string]float64)
	loyaltyLTV := make(map[string]float64)
	momentumLTV := make(map[string]float64)
	ceoGroupLTV := make(map[string]float64)

	totalCustomers := int64(0)
	customersWithOrder := int64(0)
	customersRepeat := int64(0)
	newInPeriod := int64(0)
	vipInactiveCount := int64(0)
	reactivationValue := 0.0
	activeInPeriod := int64(0)
	totalLTV := 0.0
	vipLTV := 0.0

	// Phân bố Lớp 3
	firstPQ := make(map[string]int64)
	firstEQ := make(map[string]int64)
	firstEng := make(map[string]int64)
	firstRT := make(map[string]int64)
	firstRP := make(map[string]int64)
	repeatRD := make(map[string]int64)
	repeatRF := make(map[string]int64)
	repeatSM := make(map[string]int64)
	repeatPE := make(map[string]int64)
	repeatEE := make(map[string]int64)
	repeatUP := make(map[string]int64)
	vipVD := make(map[string]int64)
	vipST := make(map[string]int64)
	vipPD := make(map[string]int64)
	vipEL := make(map[string]int64)
	vipRS := make(map[string]int64)
	inactiveED := make(map[string]int64)
	inactiveRP := make(map[string]int64)
	engagedTemp := make(map[string]int64)
	engagedDepth := make(map[string]int64)
	engagedSource := make(map[string]int64)

	inc := func(m map[string]int64, k string) {
		if k == "" {
			return
		}
		m[k]++
	}

	for _, m := range snapshotMap {
		valueTier := crmvc.GetStrFromNestedMetrics(m, "valueTier")
		lifecycleStage := crmvc.GetStrFromNestedMetrics(m, "lifecycleStage")
		journeyStage := crmvc.GetStrFromNestedMetrics(m, "journeyStage")
		channel := crmvc.GetStrFromNestedMetrics(m, "channel")
		loyaltyStage := crmvc.GetStrFromNestedMetrics(m, "loyaltyStage")
		momentumStage := crmvc.GetStrFromNestedMetrics(m, "momentumStage")
		orderCount := crmvc.GetIntFromNestedMetrics(m, "orderCount")
		totalSpent := crmvc.GetFloatFromNestedMetrics(m, "totalSpent")
		lastOrderAt := crmvc.GetInt64FromNestedMetrics(m, "lastOrderAt")

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

		// LTV theo từng nhóm
		valueLTV[valueTier] += totalSpent
		journeyLTV[journeyStage] += totalSpent
		lifecycleLTV[lifecycleStage] += totalSpent
		channelLTV[channel] += totalSpent
		loyaltyLTV[loyaltyStage] += totalSpent
		momentumLTV[momentumStage] += totalSpent
		ceoGroup := computeCeoGroupForLTV(valueTier, lifecycleStage, journeyStage, loyaltyStage, momentumStage)
		ceoGroupLTV[ceoGroup] += totalSpent

		// CEO groups
		if valueTier == "vip" && lifecycleStage == "active" {
			ceoDist["vip_active"]++
		}
		if valueTier == "vip" && (lifecycleStage == "inactive" || lifecycleStage == "dead") {
			ceoDist["vip_inactive"]++
			vipInactiveCount++
			reactivationValue += totalSpent
		}
		// Tổng giá trị tài sản (LTV)
		totalLTV += totalSpent
		if valueTier == "vip" {
			vipLTV += totalSpent
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

		// Lớp 3: derive và aggregate phân bố
		agg := layer3.DeriveFromNested(m, endMs)
		if agg != nil {
			if agg.First != nil {
				inc(firstPQ, agg.First.PurchaseQuality)
				inc(firstEQ, agg.First.ExperienceQuality)
				inc(firstEng, agg.First.EngagementAfterPurchase)
				inc(firstRT, agg.First.ReorderTiming)
				inc(firstRP, agg.First.RepeatProbability)
			}
			if agg.Repeat != nil {
				inc(repeatRD, agg.Repeat.RepeatDepth)
				inc(repeatRF, agg.Repeat.RepeatFrequency)
				inc(repeatSM, agg.Repeat.SpendMomentum)
				inc(repeatPE, agg.Repeat.ProductExpansion)
				inc(repeatEE, agg.Repeat.EmotionalEngagement)
				inc(repeatUP, agg.Repeat.UpgradePotential)
			}
			if agg.Vip != nil {
				inc(vipVD, agg.Vip.VipDepth)
				inc(vipST, agg.Vip.SpendTrend)
				inc(vipPD, agg.Vip.ProductDiversity)
				inc(vipEL, agg.Vip.EngagementLevel)
				inc(vipRS, agg.Vip.RiskScore)
			}
			if agg.Inactive != nil {
				inc(inactiveED, agg.Inactive.EngagementDrop)
				inc(inactiveRP, agg.Inactive.ReactivationPotential)
			}
			if agg.Engaged != nil {
				inc(engagedTemp, agg.Engaged.ConversationTemperature)
				inc(engagedDepth, agg.Engaged.EngagementDepth)
				inc(engagedSource, agg.Engaged.SourceType)
			}
		}
	}

	repeatRate := 0.0
	if customersWithOrder > 0 {
		repeatRate = float64(customersRepeat) / float64(customersWithOrder)
	}

	avgLTV := 0.0
	if totalCustomers > 0 {
		avgLTV = totalLTV / float64(totalCustomers)
	}

	// Cấu trúc nested — dễ đọc, dễ mở rộng
	metrics := map[string]interface{}{
		"summary": map[string]interface{}{
			"totalCustomers":       totalCustomers,
			"customersWithOrder":   customersWithOrder,
			"customersRepeat":      customersRepeat,
			"newCustomersInPeriod": newInPeriod,
			"repeatRate":           repeatRate,
			"vipInactiveCount":     vipInactiveCount,
			"reactivationValue":    int64(reactivationValue),
			"activeInPeriod":       activeInPeriod,
			"totalLTV":             totalLTV,
			"avgLTV":               avgLTV,
			"vipLTV":               vipLTV,
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
		"valueLTV": map[string]interface{}{
			"vip": valueLTV["vip"], "high": valueLTV["high"], "medium": valueLTV["medium"],
			"low": valueLTV["low"], "new": valueLTV["new"],
		},
		"journeyLTV": map[string]interface{}{
			"visitor": journeyLTV["visitor"], "engaged": journeyLTV["engaged"], "first": journeyLTV["first"],
			"repeat": journeyLTV["repeat"], "vip": journeyLTV["vip"], "inactive": journeyLTV["inactive"],
		},
		"lifecycleLTV": map[string]interface{}{
			"active": lifecycleLTV["active"], "cooling": lifecycleLTV["cooling"], "inactive": lifecycleLTV["inactive"],
			"dead": lifecycleLTV["dead"], "never_purchased": lifecycleLTV["never_purchased"],
		},
		"channelLTV": map[string]interface{}{
			"online": channelLTV["online"], "offline": channelLTV["offline"],
			"omnichannel": channelLTV["omnichannel"], "unspecified": channelLTV["_unspecified"],
		},
		"loyaltyLTV": map[string]interface{}{
			"core": loyaltyLTV["core"], "repeat": loyaltyLTV["repeat"],
			"one_time": loyaltyLTV["one_time"], "unspecified": loyaltyLTV["_unspecified"],
		},
		"momentumLTV": map[string]interface{}{
			"rising": momentumLTV["rising"], "stable": momentumLTV["stable"], "declining": momentumLTV["declining"],
			"lost": momentumLTV["lost"], "unspecified": momentumLTV["_unspecified"],
		},
		"ceoGroupLTV": map[string]interface{}{
			"vip_active": ceoGroupLTV["vip_active"], "vip_inactive": ceoGroupLTV["vip_inactive"],
			"rising": ceoGroupLTV["rising"], "new": ceoGroupLTV["new"],
			"one_time": ceoGroupLTV["one_time"], "dead": ceoGroupLTV["dead"], "other": ceoGroupLTV["_other"],
		},
		"firstLayer3": map[string]interface{}{
			"purchaseQuality": firstPQ, "experienceQuality": firstEQ, "engagementAfterPurchase": firstEng,
			"reorderTiming": firstRT, "repeatProbability": firstRP,
		},
		"repeatLayer3": map[string]interface{}{
			"repeatDepth": repeatRD, "repeatFrequency": repeatRF, "spendMomentum": repeatSM,
			"productExpansion": repeatPE, "emotionalEngagement": repeatEE, "upgradePotential": repeatUP,
		},
		"vipLayer3": map[string]interface{}{
			"vipDepth": vipVD, "spendTrend": vipST, "productDiversity": vipPD,
			"engagementLevel": vipEL, "riskScore": vipRS,
		},
		"inactiveLayer3": map[string]interface{}{
			"engagementDrop": inactiveED, "reactivationPotential": inactiveRP,
		},
		"engagedLayer3": map[string]interface{}{
			"conversationTemperature": engagedTemp, "engagementDepth": engagedDepth, "sourceType": engagedSource,
		},
	}
	return metrics, nil
}

// computeCeoGroupForLTV gán mỗi khách vào đúng 1 nhóm CEO (mutually exclusive) để tính LTV.
func computeCeoGroupForLTV(valueTier, lifecycleStage, journeyStage, loyaltyStage, momentumStage string) string {
	if valueTier == "vip" && lifecycleStage == "active" {
		return "vip_active"
	}
	if valueTier == "vip" && (lifecycleStage == "inactive" || lifecycleStage == "dead") {
		return "vip_inactive"
	}
	if momentumStage == "rising" {
		return "rising"
	}
	if journeyStage == "first" || valueTier == "new" {
		return "new"
	}
	if loyaltyStage == "one_time" {
		return "one_time"
	}
	if lifecycleStage == "dead" {
		return "dead"
	}
	return "_other"
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
			CustomersWithOrder:   metricAt(m, "summary", "customersWithOrder", "customersWithOrder"),
			CustomersRepeat:     metricAt(m, "summary", "customersRepeat", "customersRepeat"),
			NewCustomersInPeriod: metricAt(m, "summary", "newCustomersInPeriod", "newCustomersInPeriod"),
			RepeatRate:           metricAtFloat(m, "summary", "repeatRate", "repeatRate"),
			VipInactiveCount:     metricAt(m, "summary", "vipInactiveCount", "vipInactiveCount"),
			ReactivationValue:    metricAt(m, "summary", "reactivationValue", "reactivationValue"),
			ActiveTodayCount:     metricAt(m, "summary", "activeInPeriod", "activeInPeriod"),
			TotalLTV:             metricAtFloat(m, "summary", "totalLTV", "totalLTV"),
			AvgLTV:               metricAtFloat(m, "summary", "avgLTV", "avgLTV"),
			VipLTV:               metricAtFloat(m, "summary", "vipLTV", "vipLTV"),
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
		ValueLTV: reportdto.ValueLTV{
			Vip:    metricAtDistFloat(m, "valueLTV", "vip"),
			High:   metricAtDistFloat(m, "valueLTV", "high"),
			Medium: metricAtDistFloat(m, "valueLTV", "medium"),
			Low:    metricAtDistFloat(m, "valueLTV", "low"),
			New:    metricAtDistFloat(m, "valueLTV", "new"),
		},
		JourneyLTV: reportdto.JourneyLTV{
			Visitor:  metricAtDistFloat(m, "journeyLTV", "visitor"),
			Engaged:  metricAtDistFloat(m, "journeyLTV", "engaged"),
			First:    metricAtDistFloat(m, "journeyLTV", "first"),
			Repeat:   metricAtDistFloat(m, "journeyLTV", "repeat"),
			Vip:      metricAtDistFloat(m, "journeyLTV", "vip"),
			Inactive: metricAtDistFloat(m, "journeyLTV", "inactive"),
		},
		LifecycleLTV: reportdto.LifecycleLTV{
			Active:         metricAtDistFloat(m, "lifecycleLTV", "active"),
			Cooling:        metricAtDistFloat(m, "lifecycleLTV", "cooling"),
			Inactive:       metricAtDistFloat(m, "lifecycleLTV", "inactive"),
			Dead:           metricAtDistFloat(m, "lifecycleLTV", "dead"),
			NeverPurchased: metricAtDistFloat(m, "lifecycleLTV", "never_purchased"),
		},
		ChannelLTV: reportdto.ChannelLTV{
			Online:      metricAtDistFloat(m, "channelLTV", "online"),
			Offline:     metricAtDistFloat(m, "channelLTV", "offline"),
			Omnichannel: metricAtDistFloat(m, "channelLTV", "omnichannel"),
			Unspecified: metricAtDistFloat(m, "channelLTV", "unspecified"),
		},
		LoyaltyLTV: reportdto.LoyaltyLTV{
			Core:        metricAtDistFloat(m, "loyaltyLTV", "core"),
			Repeat:      metricAtDistFloat(m, "loyaltyLTV", "repeat"),
			OneTime:     metricAtDistFloat(m, "loyaltyLTV", "one_time"),
			Unspecified: metricAtDistFloat(m, "loyaltyLTV", "unspecified"),
		},
		MomentumLTV: reportdto.MomentumLTV{
			Rising:      metricAtDistFloat(m, "momentumLTV", "rising"),
			Stable:     metricAtDistFloat(m, "momentumLTV", "stable"),
			Declining:  metricAtDistFloat(m, "momentumLTV", "declining"),
			Lost:       metricAtDistFloat(m, "momentumLTV", "lost"),
			Unspecified: metricAtDistFloat(m, "momentumLTV", "unspecified"),
		},
		CeoGroupLTV: reportdto.CeoGroupLTV{
			VipActive:   metricAtDistFloat(m, "ceoGroupLTV", "vip_active"),
			VipInactive: metricAtDistFloat(m, "ceoGroupLTV", "vip_inactive"),
			Rising:      metricAtDistFloat(m, "ceoGroupLTV", "rising"),
			New:         metricAtDistFloat(m, "ceoGroupLTV", "new"),
			OneTime:     metricAtDistFloat(m, "ceoGroupLTV", "one_time"),
			Dead:        metricAtDistFloat(m, "ceoGroupLTV", "dead"),
			Other:       metricAtDistFloat(m, "ceoGroupLTV", "other"),
		},
		FirstLayer3: reportdto.FirstLayer3Distribution{
			PurchaseQuality:        metricAtLayer3Map(m, "firstLayer3", "purchaseQuality"),
			ExperienceQuality:      metricAtLayer3Map(m, "firstLayer3", "experienceQuality"),
			EngagementAfterPurchase: metricAtLayer3Map(m, "firstLayer3", "engagementAfterPurchase"),
			ReorderTiming:          metricAtLayer3Map(m, "firstLayer3", "reorderTiming"),
			RepeatProbability:      metricAtLayer3Map(m, "firstLayer3", "repeatProbability"),
		},
		RepeatLayer3: reportdto.RepeatLayer3Distribution{
			RepeatDepth:         metricAtLayer3Map(m, "repeatLayer3", "repeatDepth"),
			RepeatFrequency:     metricAtLayer3Map(m, "repeatLayer3", "repeatFrequency"),
			SpendMomentum:       metricAtLayer3Map(m, "repeatLayer3", "spendMomentum"),
			ProductExpansion:    metricAtLayer3Map(m, "repeatLayer3", "productExpansion"),
			EmotionalEngagement: metricAtLayer3Map(m, "repeatLayer3", "emotionalEngagement"),
			UpgradePotential:    metricAtLayer3Map(m, "repeatLayer3", "upgradePotential"),
		},
		VipLayer3: reportdto.VipLayer3Distribution{
			VipDepth:         metricAtLayer3Map(m, "vipLayer3", "vipDepth"),
			SpendTrend:       metricAtLayer3Map(m, "vipLayer3", "spendTrend"),
			ProductDiversity: metricAtLayer3Map(m, "vipLayer3", "productDiversity"),
			EngagementLevel:  metricAtLayer3Map(m, "vipLayer3", "engagementLevel"),
			RiskScore:        metricAtLayer3Map(m, "vipLayer3", "riskScore"),
		},
		InactiveLayer3: reportdto.InactiveLayer3Distribution{
			EngagementDrop:        metricAtLayer3Map(m, "inactiveLayer3", "engagementDrop"),
			ReactivationPotential: metricAtLayer3Map(m, "inactiveLayer3", "reactivationPotential"),
		},
		EngagedLayer3: reportdto.EngagedLayer3Distribution{
			ConversationTemperature: metricAtLayer3Map(m, "engagedLayer3", "conversationTemperature"),
			EngagementDepth:         metricAtLayer3Map(m, "engagedLayer3", "engagementDepth"),
			SourceType:              metricAtLayer3Map(m, "engagedLayer3", "sourceType"),
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

// metricAtDistFloat lấy float64 từ LTV distribution nested (m[LTVKey][key]).
func metricAtDistFloat(m map[string]interface{}, ltvKey, key string) float64 {
	if g, ok := m[ltvKey].(map[string]interface{}); ok {
		return snapshotMetricToFloat64(g[key])
	}
	return 0
}

// metricAtLayer3Map lấy map[string]int64 từ Lớp 3 distribution (m[group][key]).
func metricAtLayer3Map(m map[string]interface{}, group, key string) map[string]int64 {
	groupMap, ok := m[group].(map[string]interface{})
	if !ok || groupMap == nil {
		return nil
	}
	inner, ok := groupMap[key].(map[string]interface{})
	if !ok || inner == nil {
		return nil
	}
	out := make(map[string]int64)
	for k, v := range inner {
		out[k] = snapshotMetricToInt64(v)
	}
	return out
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
