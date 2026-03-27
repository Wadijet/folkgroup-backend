// Package worker — Ads Daily Scheduler: chạy jobs theo giờ cố định (FolkForm v4.1).
// 05:30 Reset, 06:00 Morning On, 07:30 Mode Detection, 12:30/14:00 Noon Cut, 14:30 Bật lại, 21-23h Night Off.
package worker

import (
	"context"
	"time"

	"meta_commerce/internal/api/aidecision/adsautop"
	adssvc "meta_commerce/internal/api/ads/service"
	"meta_commerce/internal/logger"
	coreworker "meta_commerce/internal/worker"
)

// AdsDailySchedulerWorker chạy mỗi phút, kiểm tra giờ và gọi job tương ứng.
type AdsDailySchedulerWorker struct {
	interval time.Duration
	baseURL  string
}

// NewAdsDailySchedulerWorker tạo worker mới.
func NewAdsDailySchedulerWorker(interval time.Duration, baseURL string) *AdsDailySchedulerWorker {
	if interval < 30*time.Second {
		interval = 1 * time.Minute
	}
	return &AdsDailySchedulerWorker{interval: interval, baseURL: baseURL}
}

// Start chạy worker.
func (w *AdsDailySchedulerWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.WithFields(map[string]interface{}{
		"interval": w.interval.String(),
	}).Info("📅 [ADS_DAILY] Starting Daily Scheduler...")

	for {
		select {
		case <-ctx.Done():
			log.Info("📅 [ADS_DAILY] Scheduler stopped")
			return
		case <-ticker.C:
			if !coreworker.IsWorkerActive(coreworker.WorkerAdsDailyScheduler) {
				time.Sleep(1 * time.Minute)
				continue
			}
			p := coreworker.GetPriority(coreworker.WorkerAdsDailyScheduler, coreworker.PriorityNormal)
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

func (w *AdsDailySchedulerWorker) process(ctx context.Context) {
	log := logger.GetAppLogger()
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(map[string]interface{}{"panic": r}).Error("📅 [ADS_DAILY] Panic")
		}
	}()

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	h, m := now.Hour(), now.Minute()
	weekday := now.Weekday() // 0=Sun, 1=Mon, ...

	// 05:30 — Reset Budget
	if h == 5 && m == 30 {
		adssvc.RunResetBudget(ctx)
	}
	// 06:00 — Morning On
	if h == 6 && m == 0 {
		adssvc.RunMorningOn(ctx, w.baseURL)
	}
	// 06:05 Thứ 2 — Weekly Feedback Loop (Kill Accuracy, đề xuất threshold)
	if weekday == time.Monday && h == 6 && m == 5 {
		adssvc.RunWeeklyFeedbackLoop(ctx)
	}
	// 07:30 — Mode Detection
	if h == 7 && m == 30 {
		if _, err := adssvc.RunModeDetection(ctx); err != nil {
			log.WithError(err).Warn("📅 [ADS_DAILY] Mode Detection lỗi")
		}
	}
	// 07:45 — Predictive Trend Alerts (FolkForm v4.1 Section 2.4)
	if h == 7 && m == 45 {
		if _, err := adssvc.RunPredictiveTrendAlerts(ctx, w.baseURL); err != nil {
			log.WithError(err).Warn("📅 [ADS_DAILY] Predictive Trend Alerts lỗi")
		}
	}
	// 12:30 — Noon Cut Off
	if h == 12 && m == 30 {
		adssvc.RunNoonCutOff(ctx)
	}
	// 14:00 — Noon Cut lần 2
	if h == 14 && m == 0 {
		adssvc.RunNoonCutOff(ctx)
	}
	// 14:30 — Bật lại
	if h == 14 && m == 30 {
		adssvc.RunNoonCutResume(ctx)
	}
	// 21:00, 22:00, 22:30, 23:00 — Night Off (theo mode)
	if (h == 21 && m == 0) || (h == 22 && (m == 0 || m == 30)) || (h == 23 && m == 0) {
		adssvc.RunNightOff(ctx)
	}
	// 16:00, 18:00 — Volume Push (BLITZ 16h, NORMAL 18h)
	if (h == 16 || h == 18) && m == 0 {
		adsautop.RunVolumePush(ctx, w.baseURL)
	}
	// Mỗi :00 trong khung 9–20h — Rule 13 Throttle (1-2-6): cap Ad Set tệ 15%
	if h >= 9 && h <= 20 && m == 0 {
		if n, err := adssvc.RunThrottleCheck(ctx); err != nil {
			log.WithError(err).Warn("📅 [ADS_DAILY] Throttle Check lỗi")
		} else if n > 0 {
			log.WithFields(map[string]interface{}{"throttled": n}).Info("📅 [ADS_DAILY] Throttle đã cap Ad Set tệ")
		}
	}
	// Mỗi :00 và :30 — Pre-Peak Boost, Post-Peak Trim (FolkForm v4.1 Section 05)
	if m == 0 || m == 30 {
		if _, err := adssvc.RunPrePeakBoost(ctx, w.baseURL); err != nil {
			log.WithError(err).Warn("📅 [ADS_DAILY] Pre-Peak Boost lỗi")
		}
		if _, err := adssvc.RunPostPeakTrim(ctx, w.baseURL); err != nil {
			log.WithError(err).Warn("📅 [ADS_DAILY] Post-Peak Trim lỗi")
		}
	}
}
