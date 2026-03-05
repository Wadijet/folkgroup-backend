// Package reportsvc - Số dư cuối kỳ tính từ report_snapshots (phát sinh).
//
// LƯU Ý: Snapshots lưu SỐ PHÁT SINH (in/out) mỗi kỳ, không lưu số dư.
// Số cộng dồn từ snapshots = tổng phát sinh. Số đầu kỳ = 0.
// Ưu tiên chu kỳ dài: thay 3 chu kỳ ngày bằng 1 chu kỳ tháng để ít snapshot nhất.
package reportsvc

import (
	"context"
	"fmt"
	"time"

	reportdto "meta_commerce/internal/api/report/dto"
	reportmodels "meta_commerce/internal/api/report/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// reportKeyOrderCustomer thứ tự ưu tiên cho customer: dài trước, ngắn sau (ít snapshot nhất).
var reportKeyOrderCustomer = []string{"customer_yearly", "customer_monthly", "customer_weekly", "customer_daily"}

// reportKeyOrderOrder thứ tự ưu tiên cho order: dài trước, ngắn sau.
var reportKeyOrderOrder = []string{"order_yearly", "order_monthly", "order_weekly", "order_daily"}

// reportKeyOrderAds thứ tự ưu tiên cho ads (ads_daily — chu kỳ ngày).
var reportKeyOrderAds = []string{"ads_daily"}

// GetReportKeyOrderForDomain trả về thứ tự reportKey theo domain (customer|order|ads).
func GetReportKeyOrderForDomain(domain string) []string {
	if domain == "order" {
		return reportKeyOrderOrder
	}
	if domain == "ads" {
		return reportKeyOrderAds
	}
	return reportKeyOrderCustomer
}

// periodRangeCandidate một ứng viên (reportKey, fromStr, toStr) cho query snapshots.
type periodRangeCandidate struct {
	reportKey string
	fromStr   string
	toStr     string
}

// getCandidateReportKeysAndRanges trả về danh sách ứng viên theo thứ tự ưu tiên: chu kỳ dài trước (ít snapshot).
// Chỉ thêm chu kỳ dài khi [startMs, endMs] KHỚP ranh giới chu kỳ — tránh thừa/thiếu phát sinh.
// Dùng để thử lần lượt: nếu chu kỳ dài không có snapshot thì thử chu kỳ ngắn hơn.
// reportKeyOrder: thứ tự ưu tiên (vd: reportKeyOrderCustomer hoặc reportKeyOrderOrder).
func getCandidateReportKeysAndRanges(startMs, endMs int64, reportKeyOrder []string) []periodRangeCandidate {
	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return nil
	}
	startT := time.UnixMilli(startMs).In(loc)
	endT := time.UnixMilli(endMs).In(loc)

	days := int(endT.Sub(startT).Hours()/24) + 1
	if days < 1 {
		days = 1
	}

	// Thu thập tất cả ứng viên hợp lệ (chu kỳ dài → ngắn).
	type item struct {
		reportKey string
		fromStr   string
		toStr     string
		count     int
	}
	var items []item
	dailyKey := "customer_daily"
	if len(reportKeyOrder) > 0 && len(reportKeyOrder[len(reportKeyOrder)-1]) > 0 {
		dailyKey = reportKeyOrder[len(reportKeyOrder)-1]
	}
	for _, rk := range reportKeyOrder {
		from, to, cnt := periodRangeForReportKey(rk, startT, endT, loc)
		if cnt > 0 && cnt < 999999 {
			items = append(items, item{rk, from, to, cnt})
		}
	}
	// Đảm bảo có daily (fallback cuối nếu chưa có).
	if len(items) == 0 || items[len(items)-1].reportKey != dailyKey {
		items = append(items, item{dailyKey, startT.Format("2006-01-02"), endT.Format("2006-01-02"), days})
	}

	// Sắp xếp theo count tăng dần (ưu tiên chu kỳ dài = ít snapshot).
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].count < items[i].count {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Loại bỏ trùng (cùng reportKey).
	seen := make(map[string]bool)
	var out []periodRangeCandidate
	for _, it := range items {
		if seen[it.reportKey] {
			continue
		}
		seen[it.reportKey] = true
		out = append(out, periodRangeCandidate{it.reportKey, it.fromStr, it.toStr})
	}
	return out
}

// periodRangeForReportKey tính fromStr, toStr và count periods cho reportKey trong [startT, endT].
// Chỉ dùng chu kỳ dài khi range [startT, endT] KHỚP ranh giới chu kỳ — tránh thừa/thiếu phát sinh.
// Mỗi chu kỳ phải khớp với số ngày thực tế (count * ngày/chu kỳ = span thực tế).
// Report align: yearly = 1/1–31/12, monthly = đầu tháng–cuối tháng, weekly = T2 00:00–CN 23:59:59.
// Hỗ trợ cả customer_* và order_*.
func periodRangeForReportKey(reportKey string, startT, endT time.Time, loc *time.Location) (fromStr, toStr string, count int) {
	actualDays := int(endT.Sub(startT).Hours()/24) + 1
	if actualDays < 1 {
		actualDays = 1
	}

	switch {
	case reportKey == "customer_yearly" || reportKey == "order_yearly":
		// Chỉ dùng yearly khi range trùng ranh giới năm: bắt đầu 1/1 00:00, kết thúc 31/12 23:59:59.
		if !isStartOfYear(startT) || !isEndOfYear(endT, loc) {
			return "", "", 999999
		}
		yFrom := startT.Year()
		yTo := endT.Year()
		count = yTo - yFrom + 1
		expectedDays := daysInYears(yFrom, yTo, loc)
		if expectedDays != actualDays {
			return "", "", 999999
		}
		return fmt.Sprintf("%d", yFrom), fmt.Sprintf("%d", yTo), count

	case reportKey == "customer_monthly" || reportKey == "order_monthly":
		// Chỉ dùng monthly khi range khớp ranh giới tháng: start = 1st 00:00, end = cuối tháng 23:59:59.
		if !isStartOfMonth(startT) || !isEndOfMonth(endT, loc) {
			return "", "", 999999
		}
		from := time.Date(startT.Year(), startT.Month(), 1, 0, 0, 0, 0, loc)
		to := time.Date(endT.Year(), endT.Month(), 1, 0, 0, 0, 0, loc)
		count = 0
		for d := from; !d.After(to); d = d.AddDate(0, 1, 0) {
			count++
		}
		expectedDays := daysInMonths(from, to, loc)
		if expectedDays != actualDays {
			return "", "", 999999
		}
		return from.Format("2006-01"), to.Format("2006-01"), count

	case reportKey == "customer_weekly" || reportKey == "order_weekly":
		// Chỉ dùng weekly khi range khớp ranh giới tuần: start = T2 00:00, end = CN 23:59:59.
		if !isStartOfWeek(startT) || !isEndOfWeek(endT, loc) {
			return "", "", 999999
		}
		mondayStart := getMondayOfWeek(startT)
		mondayEnd := getMondayOfWeek(endT)
		count = 1
		if mondayEnd.After(mondayStart) {
			count = int(mondayEnd.Sub(mondayStart).Hours()/24)/7 + 1
		}
		expectedDays := count * 7
		if expectedDays != actualDays {
			return "", "", 999999
		}
		return mondayStart.Format("2006-01-02"), mondayEnd.Format("2006-01-02"), count

	case reportKey == "customer_daily" || reportKey == "order_daily" || reportKey == "ads_daily":
		// Daily luôn dùng được; ranh giới tùy startT/endT; count = số ngày thực tế.
		return startT.Format("2006-01-02"), endT.Format("2006-01-02"), actualDays

	default:
		return startT.Format("2006-01-02"), endT.Format("2006-01-02"), 1
	}
}

// daysInYears tổng số ngày từ 1/1 yFrom đến 31/12 yTo.
func daysInYears(yFrom, yTo int, loc *time.Location) int {
	start := time.Date(yFrom, 1, 1, 0, 0, 0, 0, loc)
	end := time.Date(yTo, 12, 31, 23, 59, 59, 0, loc)
	return int(end.Sub(start).Hours()/24) + 1
}

// daysInMonths tổng số ngày từ đầu tháng from đến cuối tháng to.
func daysInMonths(from, to time.Time, loc *time.Location) int {
	endOfLast := time.Date(to.Year(), to.Month()+1, 0, 23, 59, 59, 0, loc)
	return int(endOfLast.Sub(from).Hours()/24) + 1
}

// isStartOfYear true nếu t = 1/1 00:00:00.
func isStartOfYear(t time.Time) bool {
	return t.Day() == 1 && t.Month() == 1 && t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0
}

// isEndOfYear true nếu t = 31/12 và gần cuối ngày (23:59).
func isEndOfYear(t time.Time, _ *time.Location) bool {
	return t.Month() == 12 && t.Day() == 31 && t.Hour() == 23 && t.Minute() >= 59
}

// isStartOfMonth true nếu t = ngày 1 00:00:00.
func isStartOfMonth(t time.Time) bool {
	return t.Day() == 1 && t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0
}

// isEndOfMonth true nếu t = ngày cuối tháng và 23:59.
func isEndOfMonth(t time.Time, loc *time.Location) bool {
	lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, loc)
	return t.Day() == lastDay.Day() && t.Hour() == 23 && t.Minute() >= 59
}

// isStartOfWeek true nếu t = thứ Hai 00:00:00.
func isStartOfWeek(t time.Time) bool {
	return t.Weekday() == time.Monday && t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0
}

// isEndOfWeek true nếu t = Chủ nhật và 23:59 (cuối tuần).
func isEndOfWeek(t time.Time, _ *time.Location) bool {
	return t.Weekday() == time.Sunday && t.Hour() == 23 && t.Minute() >= 59
}

// earliestPeriodKey chu kỳ sớm nhất để bắt đầu tích lũy (tính từ đầu).
var earliestPeriodKey = map[string]string{
	"customer_yearly":  "2000",
	"customer_monthly": "2000-01",
	"customer_weekly":  "2000-01-03", // Thứ Hai đầu tiên của 2000
	"customer_daily":   "2000-01-01",
}

// GetPeriodEndBalanceFromSnapshots lấy số dư cuối kỳ = 0 + tổng phát sinh từ đầu đến endMs.
// Lưu ý: Mỗi snapshot là số phát sinh (in/out) của kỳ đó; cộng dồn = số dư.
// Phải tích lũy TỪ ĐẦU mới khớp GetPeriodEndBalance (CRM). Tối ưu: ưu tiên chu kỳ dài.
func (s *ReportService) GetPeriodEndBalanceFromSnapshots(ctx context.Context, ownerOrgID primitive.ObjectID, params *reportdto.CustomersQueryParams) (map[string]interface{}, error) {
	if params == nil {
		params = &reportdto.CustomersQueryParams{}
	}
	applyCustomersDefaults(params)

	_, endMs, err := getStartEndMsFromParams(params)
	if err != nil {
		return nil, err
	}

	// Số cuối kỳ = 0 + sum(phát sinh từ đầu đến endMs). Bỏ qua startMs từ params.
	endT := time.UnixMilli(endMs).In(mustLoadLoc())
	toStrDaily := endT.Format("2006-01-02")
	toStrMonthly := endT.Format("2006-01")
	toStrYearly := fmt.Sprintf("%d", endT.Year())
	mondayEnd := getMondayOfWeek(endT)
	toStrWeekly := mondayEnd.Format("2006-01-02")

	// Thử chu kỳ dài → ngắn: dùng chu kỳ đầu tiên có snapshot.
	candidates := []periodRangeCandidate{
		{"customer_yearly", earliestPeriodKey["customer_yearly"], toStrYearly},
		{"customer_monthly", earliestPeriodKey["customer_monthly"], toStrMonthly},
		{"customer_weekly", earliestPeriodKey["customer_weekly"], toStrWeekly},
		{"customer_daily", earliestPeriodKey["customer_daily"], toStrDaily},
	}
	var snapshots []reportmodels.ReportSnapshot
	for _, c := range candidates {
		list, err := s.FindSnapshotsForTrend(ctx, c.reportKey, ownerOrgID, c.fromStr, c.toStr)
		if err != nil {
			return nil, fmt.Errorf("truy vấn snapshots: %w", err)
		}
		if len(list) > 0 {
			snapshots = list
			break
		}
	}

	return sumPhatSinhToBalance(snapshots), nil
}

func mustLoadLoc() *time.Location {
	loc, _ := time.LoadLocation(ReportTimezone)
	if loc == nil {
		loc = time.UTC
	}
	return loc
}

// getPeriodKeyForEndMs trả về periodKey chứa endMs cho reportKey (dùng cho fallback chu kỳ dài→ngắn).
func getPeriodKeyForEndMs(endMs int64, reportKey string) string {
	loc := mustLoadLoc()
	endT := time.UnixMilli(endMs).In(loc)
	switch reportKey {
	case "customer_yearly":
		return fmt.Sprintf("%d", endT.Year())
	case "customer_monthly":
		return endT.Format("2006-01")
	case "customer_weekly":
		return getMondayOfWeek(endT).Format("2006-01-02")
	case "customer_daily":
		return endT.Format("2006-01-02")
	default:
		return endT.Format("2006-01-02")
	}
}

// getStartEndMsFromParams chuyển params thành startMs, endMs.
func getStartEndMsFromParams(params *reportdto.CustomersQueryParams) (startMs, endMs int64, err error) {
	reportKey, fromStr, toStr, err := paramsToTrendRange(params)
	if err != nil {
		return 0, 0, err
	}
	loc, err := time.LoadLocation(ReportTimezone)
	if err != nil {
		return 0, 0, err
	}

	var startSec, endSec int64
	switch reportKey {
	case "customer_daily":
		tFrom, _ := time.ParseInLocation("2006-01-02", fromStr, loc)
		tTo, _ := time.ParseInLocation("2006-01-02", toStr, loc)
		startSec = tFrom.Unix()
		endSec = tTo.AddDate(0, 0, 1).Unix() - 1
	case "customer_weekly":
		tFrom, _ := time.ParseInLocation("2006-01-02", fromStr, loc)
		tTo, _ := time.ParseInLocation("2006-01-02", toStr, loc)
		// Chuẩn hóa về thứ Hai: đảm bảo from/to align với ranh giới tuần (T2 00:00 → CN 23:59:59)
		tFrom = getMondayOfWeek(tFrom)
		tTo = getMondayOfWeek(tTo)
		startSec = tFrom.Unix()
		endSec = tTo.AddDate(0, 0, 7).Unix() - 1
	case "customer_monthly":
		tFrom, _ := time.ParseInLocation("2006-01", fromStr, loc)
		tTo, _ := time.ParseInLocation("2006-01", toStr, loc)
		startSec = tFrom.Unix()
		endSec = tTo.AddDate(0, 1, 0).Add(-time.Second).Unix()
	case "customer_yearly":
		tFrom, _ := time.ParseInLocation("2006", fromStr, loc)
		tTo, _ := time.ParseInLocation("2006", toStr, loc)
		startSec = tFrom.Unix()
		endSec = tTo.AddDate(1, 0, 0).Add(-time.Second).Unix()
	default:
		tFrom, _ := time.ParseInLocation("2006-01-02", fromStr, loc)
		tTo, _ := time.ParseInLocation("2006-01-02", toStr, loc)
		startSec = tFrom.Unix()
		endSec = tTo.AddDate(0, 0, 1).Unix() - 1
	}

	return startSec * 1000, endSec*1000 + 999, nil
}
