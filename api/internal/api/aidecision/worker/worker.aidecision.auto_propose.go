// Package worker — Auto propose Ads: do AI Decision điều phối (cùng package consumer).
// Momentum / CPM spike / RunAutoPropose — chuẩn queue & propose qua aidecision/adsautop.
package worker

import (
	"context"
	"time"

	"meta_commerce/internal/api/aidecision/adsautop"
	adssvc "meta_commerce/internal/api/ads_meta/service"
	"meta_commerce/internal/logger"
	coreworker "meta_commerce/internal/worker"
)

// AdsAutoProposeWorker chạy chu kỳ: momentum, CPM spike, adsautop.RunAutoPropose (emit ads.propose_requested).
type AdsAutoProposeWorker struct {
	interval time.Duration
	baseURL  string
}

// NewAdsAutoProposeWorker tạo worker (đăng ký WorkerAdsAutoPropose — module aidecision trong metadata).
func NewAdsAutoProposeWorker(interval time.Duration, baseURL string) *AdsAutoProposeWorker {
	if interval < 5*time.Minute {
		interval = 30 * time.Minute
	}
	if baseURL == "" {
		baseURL = "https://localhost"
	}
	return &AdsAutoProposeWorker{interval: interval, baseURL: baseURL}
}

// Start vòng lặp định kỳ (giữ hành vi tương thích worker ads_auto_propose cũ).
func (w *AdsAutoProposeWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval": w.interval.String(),
		"baseURL":  w.baseURL,
	}).Info("📢 [AI_DECISION_ADS_AUTO] Starting Ads Auto Propose (AI Decision pipeline)...")

	w.process(ctx)
	for {
		select {
		case <-ctx.Done():
			log.Info("📢 [AI_DECISION_ADS_AUTO] Ads Auto Propose stopped")
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

func (w *AdsAutoProposeWorker) process(ctx context.Context) {
	log := logger.GetAppLogger()
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(map[string]interface{}{"panic": r}).Error("📢 [AI_DECISION_ADS_AUTO] Panic khi xử lý")
		}
	}()

	if n, err := adssvc.RunMomentumTracking(ctx); err == nil && n > 0 {
		log.WithFields(map[string]interface{}{"updated": n}).Info("📊 [MOMENTUM] Đã cập nhật momentum state")
	}
	if err := adssvc.RunCPMSpikeDetectionAndActions(ctx, w.baseURL); err != nil {
		log.WithError(err).Warn("⚔️ [SELF_COMP] Lỗi CPM Spike Detection")
	}
	proposed, err := adsautop.RunAutoPropose(ctx, w.baseURL)
	if err != nil {
		log.WithError(err).Error("📢 [AI_DECISION_ADS_AUTO] Lỗi RunAutoPropose")
		return
	}
	if proposed > 0 {
		log.WithField("proposed", proposed).Info("📢 [AI_DECISION_ADS_AUTO] Đã enqueue đề xuất tự động")
	}
}
