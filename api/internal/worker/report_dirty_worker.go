package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	reportmodels "meta_commerce/internal/api/report/models"
	reportsvc "meta_commerce/internal/api/report/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker/metrics"
)

// processOneDirtyPeriod xử lý một dirty period: kiểm tra disabled, compute, set processed.
// Trả về true nếu đã xử lý thành công (để tăng totalProcessed).
func (w *ReportDirtyWorker) processOneDirtyPeriod(ctx context.Context, d *reportmodels.ReportDirtyPeriod, totalProcessed *atomic.Int32) {
	log := logger.GetAppLogger()
	if reportsvc.IsCustomerReportKeyDisabled(d.ReportKey) {
		if err := w.reportService.DeleteDirtyPeriod(ctx, d.ReportKey, d.PeriodKey, d.OwnerOrganizationID, d.AdAccountId); err != nil {
			log.WithError(err).WithFields(map[string]interface{}{
				"reportKey": d.ReportKey,
				"periodKey": d.PeriodKey,
			}).Warn("📊 [REPORT_DIRTY] DeleteDirtyPeriod thất bại")
			return
		}
		_ = w.reportService.DeleteReportSnapshot(ctx, d.ReportKey, d.PeriodKey, d.OwnerOrganizationID, d.AdAccountId)
		log.WithFields(map[string]interface{}{
			"reportKey": d.ReportKey,
			"periodKey": d.PeriodKey,
		}).Info("📊 [REPORT_DIRTY] Đã xóa dirty period và snapshot (chu kỳ tắt)")
		return
	}
	if reportsvc.IsAdsReportKeyDisabled(d.ReportKey) {
		if err := w.reportService.DeleteDirtyPeriod(ctx, d.ReportKey, d.PeriodKey, d.OwnerOrganizationID, d.AdAccountId); err != nil {
			log.WithError(err).WithFields(map[string]interface{}{
				"reportKey": d.ReportKey,
				"periodKey": d.PeriodKey,
			}).Warn("📊 [REPORT_DIRTY] DeleteDirtyPeriod thất bại")
			return
		}
		_ = w.reportService.DeleteReportSnapshot(ctx, d.ReportKey, d.PeriodKey, d.OwnerOrganizationID, d.AdAccountId)
		log.WithFields(map[string]interface{}{
			"reportKey": d.ReportKey,
			"periodKey": d.PeriodKey,
		}).Info("📊 [REPORT_DIRTY] Đã xóa dirty period và snapshot (chu kỳ ads tắt)")
		return
	}
	start := time.Now()
	if err := w.reportService.Compute(ctx, d.ReportKey, d.PeriodKey, d.OwnerOrganizationID, d.AdAccountId); err != nil {
		log.WithError(err).WithFields(map[string]interface{}{
			"reportKey":  d.ReportKey,
			"periodKey":  d.PeriodKey,
			"orgId":      d.OwnerOrganizationID.Hex(),
		}).Warn("📊 [REPORT_DIRTY] Compute thất bại, bỏ qua và sẽ thử lại lần sau")
		return
	}
	if err := w.reportService.SetDirtyProcessed(ctx, d.ReportKey, d.PeriodKey, d.OwnerOrganizationID, d.AdAccountId); err != nil {
		log.WithError(err).WithFields(map[string]interface{}{
			"reportKey": d.ReportKey,
			"periodKey": d.PeriodKey,
		}).Warn("📊 [REPORT_DIRTY] SetDirtyProcessed thất bại")
		return
	}
	metrics.RecordDuration("report_dirty:"+d.ReportKey, time.Since(start))
	totalProcessed.Add(1)
}

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

	startFields := map[string]interface{}{
		"interval":  w.interval.String(),
		"batchSize": w.batchSize,
	}
	if disabled := reportsvc.GetDisabledCustomerReportKeys(); len(disabled) > 0 {
		startFields["customerReportKeysDisabled"] = disabled
	}
	log.WithFields(startFields).Info("📊 [REPORT_DIRTY] Starting Report Dirty Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("📊 [REPORT_DIRTY] Report Dirty Worker stopped")
			return
		case <-ticker.C:
			if !IsWorkerActive(WorkerReportDirty) {
				time.Sleep(1 * time.Minute)
				continue
			}
			p := GetPriority(WorkerReportDirty, PriorityCritical)
			if ShouldThrottle(p) {
				continue
			}
			if effInterval := GetEffectiveInterval(w.interval, p); effInterval > w.interval {
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
				var totalProcessed atomic.Int32
				batchSize := GetEffectiveBatchSize(w.batchSize, p)
				basePool := GetPoolSize(WorkerReportDirty, 6)
				poolSize := GetEffectivePoolSize(basePool, p)

				for {
					// Kiểm tra throttle giữa mỗi batch — tránh xử lý hết hàng đợi khi CPU/RAM đã tăng trong lúc chạy.
					if ShouldThrottle(p) {
						break
					}
					list, err := w.reportService.GetUnprocessedDirtyPeriods(batchCtx, batchSize)
					if err != nil {
						log.WithError(err).Error("📊 [REPORT_DIRTY] Lỗi lấy danh sách dirty periods")
						return
					}
					if len(list) == 0 {
						break
					}

					if poolSize <= 1 {
						for i := range list {
							w.processOneDirtyPeriod(batchCtx, &list[i], &totalProcessed)
						}
					} else {
						jobs := make(chan *reportmodels.ReportDirtyPeriod, len(list))
						var wg sync.WaitGroup
						for i := 0; i < poolSize; i++ {
							wg.Add(1)
							go func() {
								defer wg.Done()
								for d := range jobs {
									w.processOneDirtyPeriod(batchCtx, d, &totalProcessed)
								}
							}()
						}
						for i := range list {
							jobs <- &list[i]
						}
						close(jobs)
						wg.Wait()
					}
				}

				if n := totalProcessed.Load(); n > 0 {
					log.WithFields(map[string]interface{}{
						"processed": n,
					}).Info("📊 [REPORT_DIRTY] Đã xử lý hết dirty periods trong hàng đợi")
				}
			}()
		}
	}
}
