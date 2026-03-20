package worker

import (
	"context"
	"fmt"
	"strings"
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
	if reportsvc.IsOrderReportKeyDisabled(d.ReportKey) {
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
		}).Info("📊 [REPORT_DIRTY] Đã xóa dirty period và snapshot (chu kỳ order tắt)")
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

// ReportDirtyWorker worker xử lý report_dirty_periods cho một domain (ads, order, customer).
// Mỗi domain là worker độc lập — config riêng (priority, active, pool size, interval).
type ReportDirtyWorker struct {
	reportService *reportsvc.ReportService
	workerName    string // report_dirty_ads, report_dirty_order, report_dirty_customer
	domain        string // ads, order, customer
}

// NewReportDirtyWorker tạo worker cho một domain. domain: "ads", "order", "customer".
// Config interval/batch từ reportSchedules (env REPORT_*_INTERVAL, REPORT_*_BATCH hoặc API).
func NewReportDirtyWorker(domain string) (*ReportDirtyWorker, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain != "ads" && domain != "order" && domain != "customer" {
		return nil, fmt.Errorf("domain phải là ads, order hoặc customer, nhận: %s", domain)
	}
	reportService, err := reportsvc.NewReportService()
	if err != nil {
		return nil, err
	}
	workerName := "report_dirty_" + domain
	return &ReportDirtyWorker{
		reportService: reportService,
		workerName:    workerName,
		domain:        domain,
	}, nil
}

// Start chạy worker: một goroutine, xử lý dirty periods theo domain.
func (w *ReportDirtyWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	configs := reportsvc.GetReportScheduleConfigs()
	var sched *reportsvc.ReportScheduleConfig
	for i := range configs {
		if configs[i].Name == w.domain {
			sched = &configs[i]
			break
		}
	}
	if sched == nil {
		log.WithField("domain", w.domain).Error("📊 [REPORT_DIRTY] Không tìm thấy config cho domain")
		return
	}
	log.WithFields(map[string]interface{}{
		"worker":   w.workerName,
		"interval": sched.Interval.String(),
		"batch":    sched.BatchSize,
	}).Info("📊 [REPORT_DIRTY] Starting Report Dirty Worker")
	w.runLoop(ctx, sched)
	log.WithField("worker", w.workerName).Info("📊 [REPORT_DIRTY] Report Dirty Worker stopped")
}

// runLoop chạy vòng lặp: mỗi vòng đọc config mới (hỗ trợ thay đổi qua API), chờ interval rồi xử lý.
func (w *ReportDirtyWorker) runLoop(ctx context.Context, sched *reportsvc.ReportScheduleConfig) {
	log := logger.GetAppLogger()

	for {
		configs := reportsvc.GetReportScheduleConfigs()
		var cfg *reportsvc.ReportScheduleConfig
		for i := range configs {
			if configs[i].Name == w.domain {
				cfg = &configs[i]
				break
			}
		}
		if cfg == nil {
			cfg = sched
		}
		interval := cfg.Interval
		batchSize := cfg.BatchSize

		if !IsWorkerActive(w.workerName) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := GetPriority(w.workerName, PriorityCritical)
		if ShouldThrottle(p) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}
		effInterval := GetEffectiveInterval(interval, p)
		if effInterval > interval {
			select {
			case <-ctx.Done():
				return
			case <-time.After(effInterval - interval):
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{
						"panic":  r,
						"worker": w.workerName,
					}).Error("📊 [REPORT_DIRTY] Panic khi xử lý dirty periods, sẽ tiếp tục ở lần chạy tiếp theo")
				}
			}()

			batchCtx := ctx
			var totalProcessed atomic.Int32
			effBatchSize := GetEffectiveBatchSize(batchSize, p)
			basePool := GetPoolSize(w.workerName, 6)
			poolSize := GetEffectivePoolSize(basePool, p)

			for {
				if ShouldThrottle(p) {
					break
				}
				list, err := w.reportService.GetUnprocessedDirtyPeriodsByReportKeys(batchCtx, effBatchSize, sched.ReportKeys)
				if err != nil {
					log.WithError(err).WithField("schedule", sched.Name).Error("📊 [REPORT_DIRTY] Lỗi lấy danh sách dirty periods")
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
					"worker":    w.workerName,
				}).Info("📊 [REPORT_DIRTY] Đã xử lý dirty periods")
			}
		}()
	}
}
