// Package worker — AIDecisionClosureWorker đóng case quá hạn với closed_timeout.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT. Case ở decided/actions_created quá X giờ → closed_timeout.
package worker

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"
)

// AIDecisionClosureWorker worker đóng case quá hạn.
type AIDecisionClosureWorker struct {
	interval time.Duration
}

// NewAIDecisionClosureWorker tạo mới.
func NewAIDecisionClosureWorker(interval time.Duration) *AIDecisionClosureWorker {
	if interval < 1*time.Minute {
		interval = 10 * time.Minute
	}
	return &AIDecisionClosureWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *AIDecisionClosureWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [AI_DECISION_CLOSURE] Starting Closure Worker...")

	svc := aidecisionsvc.NewAIDecisionService()
	maxAgeHours := 24
	if s := strings.TrimSpace(os.Getenv("AI_DECISION_CLOSURE_MAX_AGE_HOURS")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			maxAgeHours = n
		}
	}

	for {
		if !worker.IsWorkerActive(worker.WorkerAIDecisionClosure) {
			select {
			case <-ctx.Done():
				log.Info("📋 [AI_DECISION_CLOSURE] Closure Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerAIDecisionClosure, worker.PriorityLow)
		if worker.ShouldThrottle(p) {
			interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAIDecisionClosure, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAIDecisionClosure, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [AI_DECISION_CLOSURE] Closure Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [AI_DECISION_CLOSURE] Panic khi đóng case")
				}
			}()

			n, err := svc.CloseStaleCases(ctx, maxAgeHours)
			if err != nil {
				log.WithError(err).Warn("📋 [AI_DECISION_CLOSURE] CloseStaleCases lỗi")
				return
			}
			if n > 0 {
				log.WithField("closed", n).WithField("maxAgeHours", maxAgeHours).Info("📋 [AI_DECISION_CLOSURE] Đã đóng case quá hạn")
			}
		}()
	}
}
