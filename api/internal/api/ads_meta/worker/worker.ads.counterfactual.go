// Package worker — Counterfactual Kill Tracker (FolkForm v4.1 Section 2.3).
// B2–B3: Định kỳ đánh giá kill_snapshots đã qua 4h → tạo counterfactual_outcomes.
package worker

import (
	"context"
	"time"

	adssvc "meta_commerce/internal/api/ads_meta/service"
	"meta_commerce/internal/logger"
	coreworker "meta_commerce/internal/worker"
)

// AdsCounterfactualWorker đánh giá kill đã qua 4h, tạo counterfactual_outcome.
type AdsCounterfactualWorker struct {
	interval time.Duration
}

// NewAdsCounterfactualWorker tạo worker mới.
func NewAdsCounterfactualWorker(interval time.Duration) *AdsCounterfactualWorker {
	if interval < 10*time.Minute {
		interval = 30 * time.Minute
	}
	return &AdsCounterfactualWorker{interval: interval}
}

// Start chạy worker định kỳ.
func (w *AdsCounterfactualWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval": w.interval.String(),
	}).Info("🔍 [COUNTERFACTUAL] Starting Counterfactual Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Info("🔍 [COUNTERFACTUAL] Worker stopped")
			return
		case <-ticker.C:
			if !coreworker.IsWorkerActive(coreworker.WorkerAdsCounterfactual) {
				time.Sleep(1 * time.Minute)
				continue
			}
			p := coreworker.GetPriority(coreworker.WorkerAdsCounterfactual, coreworker.PriorityNormal)
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

func (w *AdsCounterfactualWorker) process(ctx context.Context) {
	log := logger.GetAppLogger()
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(map[string]interface{}{"panic": r}).Error("🔍 [COUNTERFACTUAL] Panic")
		}
	}()

	evaluated, err := adssvc.EvaluatePendingKills(ctx)
	if err != nil {
		log.WithError(err).Warn("🔍 [COUNTERFACTUAL] Lỗi đánh giá pending kills")
		return
	}
	if evaluated > 0 {
		log.WithFields(map[string]interface{}{"evaluated": evaluated}).Info("🔍 [COUNTERFACTUAL] Đã đánh giá outcomes")
	}
}
