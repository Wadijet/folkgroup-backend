// Package worker — CixIntelComputeWorker poll cix_intel_compute: phân tích hội thoại qua Rule Engine (cùng quy ước *_intel_compute).
//
// Nằm trong module cix để tránh import cycle (worker -> cix/service -> decision -> approval).
package worker

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"meta_commerce/internal/api/aidecision/intelrecomputed"
	cixmodels "meta_commerce/internal/api/cix/models"
	cixsvc "meta_commerce/internal/api/cix/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"
)

// cixIntelComputeMaxRetries số lần thử lại trước khi ghi bản ghi lớp A failed và đóng job.
const cixIntelComputeMaxRetries = 8

// CixIntelComputeWorker worker poll cix_intel_compute, gọi AnalyzeSession.
type CixIntelComputeWorker struct {
	interval  time.Duration
	batchSize int
}

// NewCixIntelComputeWorker tạo mới CixIntelComputeWorker.
func NewCixIntelComputeWorker(interval time.Duration, batchSize int) *CixIntelComputeWorker {
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &CixIntelComputeWorker{interval: interval, batchSize: batchSize}
}

// Start chạy worker trong vòng lặp. Implement worker.Worker.
func (w *CixIntelComputeWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithFields(map[string]interface{}{
		"interval":  w.interval.String(),
		"batchSize": w.batchSize,
	}).Info("📋 [CIX_INTEL] Starting CIX Intel Compute Worker (cix_intel_compute)...")

	analysisSvc, err := cixsvc.NewCixAnalysisService()
	if err != nil {
		log.WithError(err).Error("📋 [CIX_INTEL] Không tạo được CixAnalysisService")
		return
	}

	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CixIntelCompute)
	if !ok {
		log.Error("📋 [CIX_INTEL] Không tìm thấy collection cix_intel_compute")
		return
	}

	for {
		interval, batchSize := worker.GetEffectiveWorkerSchedule(worker.WorkerCixIntelCompute, w.interval, w.batchSize)

		if !worker.IsWorkerActive(worker.WorkerCixIntelCompute) {
			select {
			case <-ctx.Done():
				log.Info("📋 [CIX_INTEL] CIX Intel Compute Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerCixIntelCompute, worker.PriorityNormal)
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
			log.Info("📋 [CIX_INTEL] CIX Intel Compute Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithFields(map[string]interface{}{"panic": r}).Error("📋 [CIX_INTEL] Panic khi xử lý")
				}
			}()

			filter := bson.M{"processedAt": nil}
			opts := options.Find().SetLimit(int64(batchSize)).SetSort(bson.D{{Key: "createdAt", Value: 1}})
			cursor, err := coll.Find(ctx, filter, opts)
			if err != nil {
				log.WithError(err).Warn("📋 [CIX_INTEL] Lỗi query cix_intel_compute")
				return
			}
			var jobs []cixmodels.CixIntelComputeJob
			if err = cursor.All(ctx, &jobs); err != nil {
				cursor.Close(ctx)
				return
			}
			cursor.Close(ctx)

			now := time.Now().UnixMilli()
			for _, job := range jobs {
				result, err := analysisSvc.AnalyzeSessionWithParams(ctx, cixsvc.AnalyzeSessionParams{
					SessionUid:          job.ConversationID,
					CustomerUid:         job.CustomerID,
					OwnerOrganizationID: job.OwnerOrganizationID,
					ParentJobID:         job.ID,
					TraceID:             job.TraceID,
					CorrelationID:       job.CorrelationID,
					CausalOrderingAtMs:  job.CausalOrderingAtMs,
				})
				if err != nil {
					newRetry := job.RetryCount + 1
					if newRetry >= cixIntelComputeMaxRetries {
						failDoc, insErr := analysisSvc.InsertTerminalFailure(ctx, cixsvc.CixTerminalFailureInput{
							OwnerOrganizationID: job.OwnerOrganizationID,
							SessionUid:          job.ConversationID,
							CustomerUid:         job.CustomerID,
							ParentJobID:         job.ID,
							TraceID:             job.TraceID,
							CorrelationID:       job.CorrelationID,
							CausalOrderingAtMs:  job.CausalOrderingAtMs,
							Err:                 err,
						})
						analysisHex := ""
						if insErr == nil && failDoc != nil {
							analysisHex = failDoc.ID.Hex()
						}
						_ = intelrecomputed.EmitCixIntelRecomputed(ctx, job.OwnerOrganizationID, job.ID.Hex(), job.ConversationID, job.CustomerID, job.Channel, job.CioEventUid, analysisHex)
						_, _ = coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{
							"$set": bson.M{
								"processedAt":  now,
								"processError": err.Error(),
								"retryCount":   newRetry,
							},
						})
					} else {
						_, _ = coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{
							"$set": bson.M{"processError": err.Error()},
							"$inc": bson.M{"retryCount": 1},
						})
					}
					continue
				}
				analysisHex := ""
				if result != nil {
					analysisHex = result.ID.Hex()
				}
				_ = intelrecomputed.EmitCixIntelRecomputed(ctx, job.OwnerOrganizationID, job.ID.Hex(), job.ConversationID, job.CustomerID, job.Channel, job.CioEventUid, analysisHex)
				_, _ = coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{
					"$set": bson.M{
						"processedAt":  now,
						"processError": "",
						"retryCount":   0,
					},
				})
			}
		}()
	}
}
