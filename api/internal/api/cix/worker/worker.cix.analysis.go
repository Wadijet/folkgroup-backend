// Package worker — CixAnalysisWorker xử lý cix_pending_analysis: phân tích hội thoại qua Rule Engine.
//
// Nằm trong module cix để tránh import cycle (worker -> cix/service -> decision -> approval).
package worker

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	cixmodels "meta_commerce/internal/api/cix/models"
	cixsvc "meta_commerce/internal/api/cix/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"
)

// CixAnalysisWorker worker poll cix_pending_analysis, gọi AnalyzeSession.
type CixAnalysisWorker struct {
	interval  time.Duration
	batchSize int
}

// NewCixAnalysisWorker tạo mới CixAnalysisWorker.
func NewCixAnalysisWorker(interval time.Duration, batchSize int) *CixAnalysisWorker {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &CixAnalysisWorker{interval: interval, batchSize: batchSize}
}

// Start chạy worker trong vòng lặp. Implement worker.Worker.
func (w *CixAnalysisWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithFields(map[string]interface{}{
		"interval":  w.interval.String(),
		"batchSize": w.batchSize,
	}).Info("📋 [CIX_ANALYSIS] Starting CIX Analysis Worker...")

	analysisSvc, err := cixsvc.NewCixAnalysisService()
	if err != nil {
		log.WithError(err).Error("📋 [CIX_ANALYSIS] Không tạo được CixAnalysisService")
		return
	}

	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CixPendingAnalysis)
	if !ok {
		log.Error("📋 [CIX_ANALYSIS] Không tìm thấy collection cix_pending_analysis")
		return
	}

	for {
		interval, batchSize := worker.GetEffectiveWorkerSchedule(worker.WorkerCixAnalysis, w.interval, w.batchSize)

		if !worker.IsWorkerActive(worker.WorkerCixAnalysis) {
			select {
			case <-ctx.Done():
				log.Info("📋 [CIX_ANALYSIS] CIX Analysis Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerCixAnalysis, worker.PriorityNormal)
		if worker.ShouldThrottle(p) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		select {
		case <-ctx.Done():
			log.Info("📋 [CIX_ANALYSIS] CIX Analysis Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("📋 [CIX_ANALYSIS] Panic khi xử lý")
				}
			}()

			filter := bson.M{"processedAt": nil}
			opts := options.Find().SetLimit(int64(batchSize)).SetSort(bson.D{{Key: "createdAt", Value: 1}})
			cursor, err := coll.Find(ctx, filter, opts)
			if err != nil {
				log.WithError(err).Warn("📋 [CIX_ANALYSIS] Lỗi query cix_pending_analysis")
				return
			}
			var jobs []cixmodels.CixPendingAnalysis
			if err = cursor.All(ctx, &jobs); err != nil {
				cursor.Close(ctx)
				return
			}
			cursor.Close(ctx)

			now := time.Now().UnixMilli()
			for _, job := range jobs {
				_, err := analysisSvc.AnalyzeSession(ctx, job.ConversationID, job.CustomerID, job.OwnerOrganizationID)
				update := bson.M{"$set": bson.M{"processedAt": now}}
				if err != nil {
					update["$set"].(bson.M)["processError"] = err.Error()
					update["$inc"] = bson.M{"retryCount": 1}
				}
				_, _ = coll.UpdateOne(ctx, bson.M{"_id": job.ID}, update)
			}
		}()
	}
}
