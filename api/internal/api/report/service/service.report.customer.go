// Package reportsvc - Compute engine cho báo cáo khách hàng theo chu kỳ (customer_daily, customer_weekly, customer_monthly, customer_yearly).
// Snapshot chỉ lưu số phát sinh (in/out) cho toàn bộ cấu trúc metrics. Không lưu số cuối kỳ. Số dư lấy từ API GetPeriodEndBalance.
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

	metrics, err := s.computeCustomerPhatSinh(ctx, ownerOrganizationID, startMs, endMs)
	if err != nil {
		return fmt.Errorf("compute customer metrics từ activity history: %w", err)
	}

	return s.upsertSnapshot(ctx, reportKey, periodKey, def.PeriodType, ownerOrganizationID, metrics)
}

// computeCustomerPhatSinh tính số phát sinh (in/out) cho toàn bộ cấu trúc metrics. Snapshot chỉ lưu phát sinh, không lưu số cuối kỳ.
func (s *ReportService) computeCustomerPhatSinh(ctx context.Context, ownerOrgID primitive.ObjectID, startMs, endMs int64) (map[string]interface{}, error) {
	actSvc, err := crmvc.NewCrmActivityService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmActivityService: %w", err)
	}
	return computeAllPhatSinh(ctx, actSvc, ownerOrgID, startMs, endMs)
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

// periodKeyToEndMs chuyển reportKey + periodKey thành endMs (cuối kỳ).
func periodKeyToEndMs(reportKey, periodKey string) (int64, error) {
	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return 0, err
	}
	var endSec int64
	switch reportKey {
	case "customer_daily":
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return 0, err
		}
		endSec = t.AddDate(0, 0, 1).Unix() - 1
	case "customer_weekly":
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return 0, err
		}
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		endSec = monday.AddDate(0, 0, 7).Unix() - 1
	case "customer_monthly":
		t, err := time.ParseInLocation("2006-01", periodKey, loc)
		if err != nil {
			return 0, err
		}
		endSec = t.AddDate(0, 1, 0).Add(-time.Second).Unix()
	case "customer_yearly":
		t, err := time.ParseInLocation("2006", periodKey, loc)
		if err != nil {
			return 0, err
		}
		endSec = t.AddDate(1, 0, 0).Add(-time.Second).Unix()
	default:
		t, _ := time.ParseInLocation("2006-01-02", periodKey, loc)
		endSec = t.AddDate(0, 0, 1).Unix() - 1
	}
	return endSec*1000 + 999, nil
}

// periodKeyToStartMs chuyển reportKey + periodKey thành startMs (đầu kỳ).
func periodKeyToStartMs(reportKey, periodKey string) (int64, error) {
	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return 0, err
	}
	var startSec int64
	switch reportKey {
	case "customer_daily":
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return 0, err
		}
		startSec = t.Unix()
	case "customer_weekly":
		t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
		if err != nil {
			return 0, err
		}
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := t.AddDate(0, 0, -(weekday - 1))
		startSec = monday.Unix()
	case "customer_monthly":
		t, err := time.ParseInLocation("2006-01", periodKey, loc)
		if err != nil {
			return 0, err
		}
		startSec = t.Unix()
	case "customer_yearly":
		t, err := time.ParseInLocation("2006", periodKey, loc)
		if err != nil {
			return 0, err
		}
		startSec = t.Unix()
	default:
		t, _ := time.ParseInLocation("2006-01-02", periodKey, loc)
		startSec = t.Unix()
	}
	return startSec * 1000, nil
}

// GetEndMsForCustomersParams chuyển params thành endMs (cuối kỳ "to") để query số dư.
func (s *ReportService) GetEndMsForCustomersParams(params *reportdto.CustomersQueryParams) (int64, error) {
	if params == nil {
		params = &reportdto.CustomersQueryParams{}
	}
	applyCustomersDefaults(params)
	reportKey, _, periodKey, err := paramsToTrendRange(params)
	if err != nil {
		return 0, err
	}
	return periodKeyToEndMs(reportKey, periodKey)
}

// GetStartMsForCustomersParams chuyển params thành startMs (đầu kỳ của period "to") để query activeInPeriod.
func (s *ReportService) GetStartMsForCustomersParams(params *reportdto.CustomersQueryParams) (int64, error) {
	if params == nil {
		params = &reportdto.CustomersQueryParams{}
	}
	applyCustomersDefaults(params)
	reportKey, _, toStr, err := paramsToTrendRange(params)
	if err != nil {
		return 0, err
	}
	return periodKeyToStartMs(reportKey, toStr)
}

// GetPeriodEndBalance lấy số dư cuối kỳ theo cấu trúc raw/layer1/layer2/layer3 (giống metricsSnapshot).
// Query từ crm_activity_history. startMs dùng cho activeInPeriod; 0 = bỏ qua.
func (s *ReportService) GetPeriodEndBalance(ctx context.Context, ownerOrgID primitive.ObjectID, endMs, startMs int64) (map[string]interface{}, error) {
	actSvc, err := crmvc.NewCrmActivityService()
	if err != nil {
		return nil, fmt.Errorf("tạo CrmActivityService: %w", err)
	}
	snapshotMap, err := actSvc.GetLastSnapshotPerCustomerBeforeEndMs(ctx, ownerOrgID, endMs)
	if err != nil {
		return nil, err
	}
	return buildPeriodEndBalance(snapshotMap, endMs, startMs), nil
}

// CeoGroupDistributionFromBalance trích CeoGroupDistribution từ kết quả GetPeriodEndBalance (layer2.ceoGroup).
func CeoGroupDistributionFromBalance(balance map[string]interface{}) reportdto.CeoGroupDistribution {
	if balance == nil {
		return reportdto.CeoGroupDistribution{}
	}
	layer2, ok := balance["layer2"].(map[string]interface{})
	if !ok {
		return reportdto.CeoGroupDistribution{}
	}
	ceoMap, ok := layer2["ceoGroup"].(map[string]interface{})
	if !ok {
		return reportdto.CeoGroupDistribution{}
	}
	return reportdto.CeoGroupDistribution{
		VipActive:   snapshotMetricToInt64(ceoMap["vip_active"]),
		VipInactive: snapshotMetricToInt64(ceoMap["vip_inactive"]),
		Rising:      snapshotMetricToInt64(ceoMap["rising"]),
		New:         snapshotMetricToInt64(ceoMap["new"]),
		OneTime:     snapshotMetricToInt64(ceoMap["one_time"]),
		Dead:        snapshotMetricToInt64(ceoMap["dead"]),
	}
}

// GetSnapshotForCustomersDashboard lấy phát sinh (ceoGroupIn, ceoGroupOut) từ report_snapshots cho period cuối.
// Snapshot chỉ lưu in/out; Summary, CeoGroupDistribution, LTV... lấy từ realtime hoặc API số dư.
func (s *ReportService) GetSnapshotForCustomersDashboard(ctx context.Context, ownerOrgID primitive.ObjectID, params *reportdto.CustomersQueryParams) (*reportdto.CustomersDashboardSnapshotData, string, int64, error) {
	if params == nil {
		params = &reportdto.CustomersQueryParams{}
	}
	applyCustomersDefaults(params)

	reportKey, _, periodKey, err := paramsToTrendRange(params)
	if err != nil {
		return nil, "", 0, err
	}

	snap, err := s.GetReportSnapshot(ctx, reportKey, periodKey, ownerOrgID)
	if err != nil || snap == nil || snap.Metrics == nil {
		return nil, "", 0, nil
	}

	m := snap.Metrics
	// Cấu trúc giống metricsSnapshot: raw, layer1, layer2, layer3. CeoGroup nằm trong layer2.in/out.ceoGroup.
	ceoIn := metricAtPhatSinhFromLayer2(m, "ceoGroup", "in")
	ceoOut := metricAtPhatSinhFromLayer2(m, "ceoGroup", "out")
	return &reportdto.CustomersDashboardSnapshotData{
		CeoGroupIn:  ceoIn,
		CeoGroupOut: ceoOut,
	}, periodKey, snap.ComputedAt, nil
}

// metricAtPhatSinhFromLayer2 đọc CeoGroupPhatSinh từ metrics phát sinh (cấu trúc raw/layer1/layer2/layer3).
// Cấu trúc mới: layer2.dimKey.group = { in, out } — đọc in hoặc out theo inOut.
func metricAtPhatSinhFromLayer2(m map[string]interface{}, dimKey, inOut string) reportdto.CeoGroupPhatSinh {
	layer2, ok := m["layer2"].(map[string]interface{})
	if !ok {
		return reportdto.CeoGroupPhatSinh{}
	}
	sub, ok := layer2[dimKey].(map[string]interface{})
	if !ok {
		return reportdto.CeoGroupPhatSinh{}
	}
	read := func(group string) int64 {
		if g, ok := sub[group].(map[string]interface{}); ok {
			if v, ok := g[inOut]; ok {
				return snapshotMetricToInt64(v)
			}
		}
		return 0
	}
	return reportdto.CeoGroupPhatSinh{
		VipActive:   read("vip_active"),
		VipInactive: read("vip_inactive"),
		Rising:      read("rising"),
		New:         read("new"),
		OneTime:     read("one_time"),
		Dead:        read("dead"),
		Other:       read("_other"),
	}
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
