package worker

import (
	"context"
	"time"

	reportsvc "meta_commerce/internal/api/report/service"
	"meta_commerce/internal/logger"
)

// ReportDirtyWorker worker xử lý report_dirty_periods: đọc các chu kỳ chưa xử lý (processedAt = null), gọi engine Compute rồi đánh dấu processedAt.
// Chạy định kỳ (mặc định 5 phút), mỗi lần xử lý hết hàng đợi (lấy theo batch batchSize cho đến khi rỗng).
type ReportDirtyWorker struct {
	reportService *reportsvc.ReportService
	interval      time.Duration // Khoảng thời gian giữa các lần chạy
	batchSize     int           // Số bản ghi mỗi lần lấy từ DB (vd: 50); xử lý hết hàng đợi
}

// NewReportDirtyWorker tạo mới ReportDirtyWorker.
// Tham số:
//   - interval: Khoảng thời gian giữa các lần chạy (mặc định: 5 phút)
//   - batchSize: Số bản ghi mỗi lần lấy từ DB (mặc định: 50); worker xử lý hết hàng đợi
func NewReportDirtyWorker(interval time.Duration, batchSize int) (*ReportDirtyWorker, error) {
	reportService, err := reportsvc.NewReportService()
	if err != nil {
		return nil, err
	}
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &ReportDirtyWorker{
		reportService: reportService,
		interval:      interval,
		batchSize:     batchSize,
	}, nil
}

// Start chạy worker trong vòng lặp: mỗi interval xử lý hết hàng đợi dirty (lấy theo batch, xử lý tuần tự đến khi rỗng).
func (w *ReportDirtyWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval":   w.interval.String(),
		"batchSize":  w.batchSize,
	}).Info("📊 [REPORT_DIRTY] Starting Report Dirty Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("📊 [REPORT_DIRTY] Report Dirty Worker stopped")
			return
		case <-ticker.C:
			if ShouldThrottle(PriorityHigh) {
				continue
			}
			if effInterval := GetEffectiveInterval(w.interval, PriorityHigh); effInterval > w.interval {
				time.Sleep(effInterval - w.interval)
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.WithFields(map[string]interface{}{
							"panic": r,
						}).Error("📊 [REPORT_DIRTY] Panic khi xử lý dirty periods, sẽ tiếp tục ở lần chạy tiếp theo")
					}
				}()

				batchCtx := ctx
				totalProcessed := 0
				batchSize := GetEffectiveBatchSize(w.batchSize, PriorityHigh)

				for {
					list, err := w.reportService.GetUnprocessedDirtyPeriods(batchCtx, batchSize)
					if err != nil {
						log.WithError(err).Error("📊 [REPORT_DIRTY] Lỗi lấy danh sách dirty periods")
						return
					}
					if len(list) == 0 {
						break
					}

					for _, d := range list {
						if err := w.reportService.Compute(batchCtx, d.ReportKey, d.PeriodKey, d.OwnerOrganizationID); err != nil {
							log.WithError(err).WithFields(map[string]interface{}{
								"reportKey":  d.ReportKey,
								"periodKey":  d.PeriodKey,
								"orgId":      d.OwnerOrganizationID.Hex(),
							}).Warn("📊 [REPORT_DIRTY] Compute thất bại, bỏ qua và sẽ thử lại lần sau")
							continue
						}
						if err := w.reportService.SetDirtyProcessed(batchCtx, d.ReportKey, d.PeriodKey, d.OwnerOrganizationID); err != nil {
							log.WithError(err).WithFields(map[string]interface{}{
								"reportKey": d.ReportKey,
								"periodKey": d.PeriodKey,
							}).Warn("📊 [REPORT_DIRTY] SetDirtyProcessed thất bại")
							continue
						}
						totalProcessed++
					}
				}

				if totalProcessed > 0 {
					log.WithFields(map[string]interface{}{
						"processed": totalProcessed,
					}).Info("📊 [REPORT_DIRTY] Đã xử lý hết dirty periods trong hàng đợi")
				}
			}()
		}
	}
}
