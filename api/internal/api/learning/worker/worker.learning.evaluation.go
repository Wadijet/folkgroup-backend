// Package worker — LearningEvaluationWorker: batch job tính evaluation cho learning_cases chưa có.
//
// Evaluation Job tách riêng (vision 11): outcome_class, error_attribution, primary_metric, delta.
package worker

import (
	"context"
	"time"

	learningsvc "meta_commerce/internal/api/learning/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"
)

// LearningEvaluationWorker worker batch tính evaluation.
type LearningEvaluationWorker struct {
	interval   time.Duration
	batchSize  int
}

// NewLearningEvaluationWorker tạo worker mới.
func NewLearningEvaluationWorker(interval time.Duration, batchSize int) *LearningEvaluationWorker {
	if interval < time.Minute {
		interval = 5 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &LearningEvaluationWorker{interval: interval, batchSize: batchSize}
}

// Start chạy worker.
func (w *LearningEvaluationWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📚 [LEARNING] Starting Learning Evaluation Worker...")

	for {
		if !worker.IsWorkerActive(worker.WorkerLearningEvaluation) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerLearningEvaluation, w.interval, w.batchSize)
		select {
		case <-ctx.Done():
			log.Info("📚 [LEARNING] Evaluation Worker stopped")
			return
		case <-time.After(interval):
		}

		processed := learningsvc.RunEvaluationBatch(ctx, w.batchSize)
		if processed > 0 {
			log.WithField("processed", processed).Info("📚 [LEARNING] Evaluation batch completed")
		}
	}
}
