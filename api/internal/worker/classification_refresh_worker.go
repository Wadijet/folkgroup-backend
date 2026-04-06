// Package worker - ClassificationRefreshWorker định kỳ enqueue phân loại khách (lifecycle, journey...)
// qua queue AI Decision (consumer gọi RunClassificationRefreshBatch), không gọi RefreshMetrics trực tiếp tại đây.
package worker

import (
	"context"
	"time"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker/metrics"
)

// ClassificationRefreshMode chế độ chạy worker.
const (
	ClassificationRefreshModeFull  = "full"  // Tất cả khách có đơn
	ClassificationRefreshModeSmart = "smart" // Chỉ khách gần ngưỡng lifecycle
)

// ClassificationRefreshWorker mỗi tick ghi event classification_refresh vào decision_events_queue.
type ClassificationRefreshWorker struct {
	interval  time.Duration
	batchSize int
	mode      string // "full" hoặc "smart"
}

// NewClassificationRefreshWorker tạo worker mới.
func NewClassificationRefreshWorker(interval time.Duration, batchSize int, mode string) (*ClassificationRefreshWorker, error) {
	if interval < time.Hour {
		interval = 24 * time.Hour
	}
	if batchSize <= 0 {
		batchSize = 200
	}
	if mode != ClassificationRefreshModeFull && mode != ClassificationRefreshModeSmart {
		mode = ClassificationRefreshModeSmart
	}
	return &ClassificationRefreshWorker{
		interval:  interval,
		batchSize: batchSize,
		mode:      mode,
	}, nil
}

// Start chạy worker trong vòng lặp (cùng mẫu CrmPendingMergeWorker: không dùng Ticker — tránh khi inactive phải chờ cả chu kỳ 24h mới kiểm tra lại).
func (w *ClassificationRefreshWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()

	schedName := WorkerClassificationFull
	if w.mode == ClassificationRefreshModeSmart {
		schedName = WorkerClassificationSmart
	}

	log.WithFields(map[string]interface{}{
		"interval":    w.interval.String(),
		"batchSize":   w.batchSize,
		"mode":        w.mode,
		"schedWorker": schedName,
	}).Info("📊 [CLASSIFICATION_REFRESH] Starting Classification Refresh Worker (AI Decision queue)...")

	// Chờ hệ thống ổn định trước tick đầu (Mongo/registry).
	select {
	case <-ctx.Done():
		log.Info("📊 [CLASSIFICATION_REFRESH] Classification Refresh Worker stopped")
		return
	case <-time.After(1 * time.Minute):
	}

	for {
		interval, batchFromSchedule := GetEffectiveWorkerSchedule(schedName, w.interval, w.batchSize)

		if !IsWorkerActive(schedName) {
			select {
			case <-ctx.Done():
				log.Info("📊 [CLASSIFICATION_REFRESH] Classification Refresh Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}

		p := GetPriority(schedName, PriorityLowest)
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
				log.Info("📊 [CLASSIFICATION_REFRESH] Classification Refresh Worker stopped")
				return
			case <-time.After(effInterval - interval):
			}
		}

		select {
		case <-ctx.Done():
			log.Info("📊 [CLASSIFICATION_REFRESH] Classification Refresh Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📊 [CLASSIFICATION_REFRESH] Panic khi enqueue, sẽ thử lần sau")
				}
			}()
			batchSize := GetEffectiveBatchSize(batchFromSchedule, p)
			start := time.Now()
			eventID, err := crmqueue.EmitCrmIntelligenceClassificationRefreshRequested(ctx, w.mode, batchSize)
			metrics.RecordDuration("classification_refresh:"+w.mode, time.Since(start))
			if err != nil {
				log.WithError(err).Warn("📊 [CLASSIFICATION_REFRESH] Không ghi được event vào AI Decision queue")
				return
			}
			log.WithFields(map[string]interface{}{
				"eventId":   eventID,
				"batchSize": batchSize,
				"mode":      w.mode,
			}).Info("📊 [CLASSIFICATION_REFRESH] Đã enqueue classification refresh (AI Decision)")
		}()
	}
}
