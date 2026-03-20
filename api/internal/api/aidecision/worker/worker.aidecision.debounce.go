// Package worker — AIDecisionDebounceWorker flush debounce state hết window → emit message.batch_ready.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §2.6. Chạy mỗi 5s khi AI_DECISION_DEBOUNCE_ENABLED=true.
package worker

import (
	"context"
	"os"
	"strings"
	"time"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"
)

// AIDecisionDebounceWorker worker flush debounce state hết window.
type AIDecisionDebounceWorker struct {
	interval time.Duration
}

// NewAIDecisionDebounceWorker tạo mới.
func NewAIDecisionDebounceWorker(interval time.Duration) *AIDecisionDebounceWorker {
	if interval < 2*time.Second {
		interval = 2 * time.Second
	}
	return &AIDecisionDebounceWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *AIDecisionDebounceWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	if strings.TrimSpace(strings.ToLower(os.Getenv("AI_DECISION_DEBOUNCE_ENABLED"))) != "true" {
		log.Info("📋 [AI_DECISION_DEBOUNCE] Debounce tắt (AI_DECISION_DEBOUNCE_ENABLED != true), worker không chạy")
		return
	}
	log.WithField("interval", w.interval.String()).Info("📋 [AI_DECISION_DEBOUNCE] Starting Debounce Worker...")

	svc := aidecisionsvc.NewAIDecisionService()

	for {
		if !worker.IsWorkerActive(worker.WorkerAIDecisionDebounce) {
			select {
			case <-ctx.Done():
				log.Info("📋 [AI_DECISION_DEBOUNCE] Debounce Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerAIDecisionDebounce, worker.PriorityNormal)
		if worker.ShouldThrottle(p) {
			interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAIDecisionDebounce, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAIDecisionDebounce, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [AI_DECISION_DEBOUNCE] Debounce Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [AI_DECISION_DEBOUNCE] Panic khi flush debounce")
				}
			}()

			n, err := svc.FlushExpired(ctx)
			if err != nil {
				log.WithError(err).Warn("📋 [AI_DECISION_DEBOUNCE] Flush debounce lỗi")
				return
			}
			if n > 0 {
				log.WithField("emitted", n).Info("📋 [AI_DECISION_DEBOUNCE] Đã emit message.batch_ready từ debounce")
			}
		}()
	}
}
