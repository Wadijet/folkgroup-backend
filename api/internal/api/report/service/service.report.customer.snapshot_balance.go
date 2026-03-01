// Package reportsvc - Số dư cuối kỳ tính từ report_snapshots (phát sinh).
// Số dư đầu kỳ = 0, cộng tất cả phát sinh (in - out) trong kỳ. Tối ưu: dùng snapshot dài trước (yearly > monthly > weekly > daily).
package reportsvc

import (
	"context"
	"fmt"
	"time"

	reportdto "meta_commerce/internal/api/report/dto"
	reportmodels "meta_commerce/internal/api/report/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// reportKeyOrder thứ tự ưu tiên: dài trước, ngắn sau (ít snapshot nhất).
var reportKeyOrder = []string{"customer_yearly", "customer_monthly", "customer_weekly", "customer_daily"}

// periodRangeCandidate một ứng viên (reportKey, fromStr, toStr) cho query snapshots.
type periodRangeCandidate struct {
	reportKey string
	fromStr   string
	toStr     string
}

// getCandidateReportKeysAndRanges trả về danh sách ứng viên theo thứ tự ưu tiên: chu kỳ dài trước (ít snapshot).
// Dùng để thử lần lượt: nếu chu kỳ dài không có snapshot thì thử chu kỳ ngắn hơn.
func getCandidateReportKeysAndRanges(startMs, endMs int64) []periodRangeCandidate {
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
	for _, rk := range reportKeyOrder {
		from, to, cnt := periodRangeForReportKey(rk, startT, endT, loc)
		if cnt > 0 && cnt < 999999 {
			items = append(items, item{rk, from, to, cnt})
		}
	}
	// Đảm bảo có daily (fallback cuối nếu chưa có).
	if len(items) == 0 || items[len(items)-1].reportKey != "customer_daily" {
		items = append(items, item{"customer_daily", startT.Format("2006-01-02"), endT.Format("2006-01-02"), days})
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
// Report align: yearly = đầu năm (1/1), monthly = đầu tháng (1), weekly = đầu tuần (thứ 2).
func periodRangeForReportKey(reportKey string, startT, endT time.Time, loc *time.Location) (fromStr, toStr string, count int) {
	switch reportKey {
	case "customer_yearly":
		// Chỉ dùng yearly khi range trùng ranh giới năm: bắt đầu 1/1, kết thúc 31/12.
		// Tránh dùng yearly cho range partial (vd: 15/1–20/3) vì sẽ thừa phát sinh.
		if startT.Day() != 1 || startT.Month() != 1 {
			return "", "", 999999
		}
		if endT.Day() != 31 || endT.Month() != 12 {
			return "", "", 999999
		}
		yFrom := startT.Year()
		yTo := endT.Year()
		return fmt.Sprintf("%d", yFrom), fmt.Sprintf("%d", yTo), yTo - yFrom + 1

	case "customer_monthly":
		// Đầu tháng: periodKey = "YYYY-MM" (ngày 1).
		from := time.Date(startT.Year(), startT.Month(), 1, 0, 0, 0, 0, loc)
		to := time.Date(endT.Year(), endT.Month(), 1, 0, 0, 0, 0, loc)
		months := 0
		for d := from; !d.After(to); d = d.AddDate(0, 1, 0) {
			months++
		}
		return from.Format("2006-01"), to.Format("2006-01"), months

	case "customer_weekly":
		// Đầu tuần (thứ 2): periodKey = "YYYY-MM-DD" (ngày thứ 2).
		mondayStart := getMondayOfWeek(startT)
		mondayEnd := getMondayOfWeek(endT)
		weeks := 1
		if mondayEnd.After(mondayStart) {
			weeks = int(mondayEnd.Sub(mondayStart).Hours()/24)/7 + 1
		}
		return mondayStart.Format("2006-01-02"), mondayEnd.Format("2006-01-02"), weeks

	case "customer_daily":
		days := int(endT.Sub(startT).Hours()/24) + 1
		if days < 1 {
			days = 1
		}
		return startT.Format("2006-01-02"), endT.Format("2006-01-02"), days

	default:
		return startT.Format("2006-01-02"), endT.Format("2006-01-02"), 1
	}
}

// GetPeriodEndBalanceFromSnapshots lấy số dư cuối kỳ bằng cách cộng phát sinh từ report_snapshots.
// Số dư đầu kỳ = 0. Tối ưu: dùng snapshot dài trước (yearly > monthly > weekly > daily) để ít snapshot nhất.
func (s *ReportService) GetPeriodEndBalanceFromSnapshots(ctx context.Context, ownerOrgID primitive.ObjectID, params *reportdto.CustomersQueryParams) (map[string]interface{}, error) {
	if params == nil {
		params = &reportdto.CustomersQueryParams{}
	}
	applyCustomersDefaults(params)

	startMs, endMs, err := getStartEndMsFromParams(params)
	if err != nil {
		return nil, err
	}

	// Thử lần lượt chu kỳ dài → ngắn: dùng chu kỳ đầu tiên có snapshot trong DB.
	candidates := getCandidateReportKeysAndRanges(startMs, endMs)
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
