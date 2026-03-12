// Package worker — AdsAutoProposeWorker: poll campaigns có alertFlags, đánh giá rules, tự tạo đề xuất khi rule trigger.
// Theo FolkForm n8n Workflow Architecture v4.1 (WF-03 Kill Engine, WF-04 Budget Engine).
package worker

import (
	"context"
	"time"

	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/logger"
	coreworker "meta_commerce/internal/worker"
)

// AdsAutoProposeWorker worker poll campaigns, đánh giá alertFlags, gọi Propose khi rule trigger.
type AdsAutoProposeWorker struct {
	interval time.Duration
	baseURL  string
}

// NewAdsAutoProposeWorker tạo mới AdsAutoProposeWorker.
// interval: chu kỳ chạy (vd: 30 phút); baseURL: base URL cho approve/reject links trong notification.
func NewAdsAutoProposeWorker(interval time.Duration, baseURL string) *AdsAutoProposeWorker {
	if interval < 5*time.Minute {
		interval = 30 * time.Minute
	}
	if baseURL == "" {
		baseURL = "https://localhost"
	}
	return &AdsAutoProposeWorker{interval: interval, baseURL: baseURL}
}

// Start chạy worker trong vòng lặp. Chạy ngay lần đầu khi khởi động, sau đó mỗi interval.
func (w *AdsAutoProposeWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval": w.interval.String(),
		"baseURL":  w.baseURL,
	}).Info("📢 [ADS_AUTO_PROPOSE] Starting Ads Auto Propose Worker...")

	// Chạy ngay lần đầu khi khởi động server
	w.process(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info("📢 [ADS_AUTO_PROPOSE] Ads Auto Propose Worker stopped")
			return
		case <-ticker.C:
			if !coreworker.IsWorkerActive(coreworker.WorkerAdsAutoPropose) {
				time.Sleep(1 * time.Minute)
				continue
			}
			p := coreworker.GetPriority(coreworker.WorkerAdsAutoPropose, coreworker.PriorityNormal)
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

// process chạy một chu kỳ đánh giá và tạo đề xuất.
func (w *AdsAutoProposeWorker) process(ctx context.Context) {
	log := logger.GetAppLogger()
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(map[string]interface{}{"panic": r}).Error("📢 [ADS_AUTO_PROPOSE] Panic khi xử lý, sẽ tiếp tục lần sau")
		}
	}()

	// Momentum Tracker — cập nhật momentumState cho từng account (ACCELERATING/SLOWING/DROPPING)
	if n, err := adssvc.RunMomentumTracking(ctx); err == nil && n > 0 {
		log.WithFields(map[string]interface{}{"updated": n}).Info("📊 [MOMENTUM] Đã cập nhật momentum state")
	}

	// Anti Self-Competition (FolkForm v4.1 Section 06): CPM Spike Detection mỗi 30p
	if err := adssvc.RunCPMSpikeDetectionAndActions(ctx, w.baseURL); err != nil {
		log.WithError(err).Warn("⚔️ [SELF_COMP] Lỗi CPM Spike Detection")
	}

	proposed, err := adssvc.RunAutoPropose(ctx, w.baseURL)
	if err != nil {
		log.WithError(err).Error("📢 [ADS_AUTO_PROPOSE] Lỗi chạy auto propose")
		return
	}
	if proposed > 0 {
		log.WithFields(map[string]interface{}{
			"proposed": proposed,
		}).Info("📢 [ADS_AUTO_PROPOSE] Đã tạo đề xuất tự động")
	}
}
