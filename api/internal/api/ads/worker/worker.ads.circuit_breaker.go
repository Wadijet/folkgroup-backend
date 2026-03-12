// Package worker — Circuit Breaker Worker: check CB-1/2/3/4 mỗi 10 phút.
// Theo FolkForm v4.1 S07. Độc lập với vòng lặp 30p.
package worker

import (
	"context"
	"time"

	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/logger"
	coreworker "meta_commerce/internal/worker"
)

// AdsCircuitBreakerWorker worker chạy CheckCircuitBreaker mỗi 10 phút.
type AdsCircuitBreakerWorker struct {
	interval time.Duration
}

// NewAdsCircuitBreakerWorker tạo worker mới.
func NewAdsCircuitBreakerWorker(interval time.Duration) *AdsCircuitBreakerWorker {
	if interval < 5*time.Minute {
		interval = 10 * time.Minute
	}
	return &AdsCircuitBreakerWorker{interval: interval}
}

// Start chạy worker.
func (w *AdsCircuitBreakerWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval": w.interval.String(),
	}).Info("🚨 [ADS_CIRCUIT_BREAKER] Starting Circuit Breaker Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("🚨 [ADS_CIRCUIT_BREAKER] Worker stopped")
			return
		case <-ticker.C:
			if !coreworker.IsWorkerActive(coreworker.WorkerAdsCircuitBreaker) {
				time.Sleep(1 * time.Minute)
				continue
			}
			p := coreworker.GetPriority(coreworker.WorkerAdsCircuitBreaker, coreworker.PriorityNormal)
			if coreworker.ShouldThrottle(p) {
				continue
			}
			if effInterval := coreworker.GetEffectiveInterval(w.interval, p); effInterval > w.interval {
				time.Sleep(effInterval - w.interval)
			}
			w.process(ctx)
		}
	}
}

func (w *AdsCircuitBreakerWorker) process(ctx context.Context) {
	log := logger.GetAppLogger()
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(map[string]interface{}{"panic": r}).Error("🚨 [ADS_CIRCUIT_BREAKER] Panic")
		}
	}()

	triggered, err := adssvc.CheckCircuitBreaker(ctx)
	if err != nil {
		log.WithError(err).Error("🚨 [ADS_CIRCUIT_BREAKER] Lỗi check")
		return
	}
	if triggered > 0 {
		log.WithFields(map[string]interface{}{
			"triggered": triggered,
		}).Error("🚨 [ADS_CIRCUIT_BREAKER] Đã PAUSE toàn account")
	}
}
