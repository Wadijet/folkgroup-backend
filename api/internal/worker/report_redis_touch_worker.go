package worker

import (
	"context"
	"time"

	reportsvc "meta_commerce/internal/api/report/service"
	"meta_commerce/internal/logger"
)

// ReportRedisTouchFlushWorker — một worker, ba nhịp flush touch trong RAM → MarkDirty (ads / order / customer) theo config.
type ReportRedisTouchFlushWorker struct{}

// NewReportRedisTouchFlushWorker tạo worker (chu kỳ từng loại: config + reportsvc.GetReportRedisTouchFlushIntervals).
func NewReportRedisTouchFlushWorker() *ReportRedisTouchFlushWorker {
	return &ReportRedisTouchFlushWorker{}
}

// Start vòng lặp: poll tick ngắn → kiểm tra từng domain đã đến hạn flush chưa.
func (w *ReportRedisTouchFlushWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	adsI, ordI, custI := reportsvc.GetReportRedisTouchFlushIntervals()
	log.WithFields(map[string]interface{}{
		"ads": adsI.String(), "order": ordI.String(), "customer": custI.String(),
		"pollTick": reportsvc.GetReportRedisTouchPollTick().String(),
	}).Info("📊 [REPORT_TOUCH] Starting report touch flush worker — RAM → MarkDirty (multi-rate)")

	var lastAds, lastOrder, lastCustomer time.Time

	for {
		if !IsWorkerActive(WorkerReportRedisTouchFlush) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := GetPriority(WorkerReportRedisTouchFlush, PriorityNormal)
		pollTick := reportsvc.GetReportRedisTouchPollTick()
		if ShouldThrottle(p) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(GetEffectiveInterval(pollTick, p)):
			}
			continue
		}

		now := time.Now()
		adsI, ordI, custI = reportsvc.GetReportRedisTouchFlushIntervals()

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📊 [REPORT_TOUCH] Panic khi flush touch")
				}
			}()

			flushIfDue := func(domain string, iv time.Duration, last *time.Time, label string) {
				if !last.IsZero() && now.Sub(*last) < iv {
					return
				}
				n, err := reportsvc.FlushReportTouchesForDomain(ctx, domain)
				if err != nil {
					log.WithError(err).WithField("domain", label).Warn("📊 [REPORT_TOUCH] FlushReportTouchesForDomain lỗi")
					return
				}
				*last = now
				if n > 0 {
					log.WithFields(map[string]interface{}{"domain": label, "flushedKeys": n}).Info("📊 [REPORT_TOUCH] Đã flush touch → MarkDirty")
				}
			}

			flushIfDue(reportsvc.ReportRedisFlushDomainAds, adsI, &lastAds, "ads")
			flushIfDue(reportsvc.ReportRedisFlushDomainOrder, ordI, &lastOrder, "order")
			flushIfDue(reportsvc.ReportRedisFlushDomainCustomer, custI, &lastCustomer, "customer")
		}()

		select {
		case <-ctx.Done():
			log.Info("📊 [REPORT_REDIS] Report Redis touch flush worker stopped")
			return
		case <-time.After(GetEffectiveInterval(pollTick, p)):
		}
	}
}
