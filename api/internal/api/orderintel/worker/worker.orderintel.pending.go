// Package worker — OrderIntelligencePendingWorker poll order_intelligence_pending, tính Raw→L1→L2→L3→Flags tại domain.
package worker

import (
	"context"
	"time"

	orderintelmodels "meta_commerce/internal/api/orderintel/models"
	orderintelsvc "meta_commerce/internal/api/orderintel/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	wk "meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OrderIntelligencePendingWorker worker xử lý job domain Order Intelligence (không tính trong consumer AI Decision).
type OrderIntelligencePendingWorker struct {
	interval time.Duration
}

// NewOrderIntelligencePendingWorker tạo mới.
func NewOrderIntelligencePendingWorker(interval time.Duration) *OrderIntelligencePendingWorker {
	if interval < 2*time.Second {
		interval = 3 * time.Second
	}
	return &OrderIntelligencePendingWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *OrderIntelligencePendingWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [ORDER_INTEL_PENDING] Starting Order Intelligence Pending Worker...")

	for {
		if !wk.IsWorkerActive(wk.WorkerOrderIntelligencePending) {
			select {
			case <-ctx.Done():
				log.Info("📋 [ORDER_INTEL_PENDING] Order Intelligence Pending Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := wk.GetPriority(wk.WorkerOrderIntelligencePending, wk.PriorityHigh)
		if wk.ShouldThrottle(p) {
			interval, _ := wk.GetEffectiveWorkerSchedule(wk.WorkerOrderIntelligencePending, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := wk.GetEffectiveWorkerSchedule(wk.WorkerOrderIntelligencePending, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [ORDER_INTEL_PENDING] Order Intelligence Pending Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [ORDER_INTEL_PENDING] Panic khi xử lý job")
				}
			}()

			coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderIntelligencePending)
			if !ok {
				return
			}

			claim := -time.Now().UnixMilli()
			filter := bson.M{"$or": []bson.M{
				{"processedAt": bson.M{"$exists": false}},
				{"processedAt": nil},
			}}
			var job orderintelmodels.OrderIntelligencePendingJob
			err := coll.FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"processedAt": claim}},
				options.FindOneAndUpdate().
					SetSort(bson.D{{Key: "createdAt", Value: 1}}).
					SetReturnDocument(options.After),
			).Decode(&job)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					return
				}
				log.WithError(err).Warn("📋 [ORDER_INTEL_PENDING] Claim job thất bại")
				return
			}

			runErr := orderintelsvc.RunPendingJob(ctx, &job)
			if runErr != nil {
				_, uerr := coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{
					"$set": bson.M{
						"processedAt":  nil,
						"processError": runErr.Error(),
					},
					"$inc": bson.M{"retryCount": 1},
				})
				if uerr != nil {
					log.WithError(uerr).WithField("jobId", job.ID.Hex()).Warn("📋 [ORDER_INTEL_PENDING] Ghi lỗi job thất bại")
				}
				log.WithError(runErr).WithField("jobId", job.ID.Hex()).Warn("📋 [ORDER_INTEL_PENDING] RunPendingJob thất bại")
				return
			}

			now := time.Now().UnixMilli()
			_, uerr := coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{"$set": bson.M{
				"processedAt":  now,
				"processError": "",
			}})
			if uerr != nil {
				log.WithError(uerr).WithField("jobId", job.ID.Hex()).Warn("📋 [ORDER_INTEL_PENDING] Đánh dấu hoàn thành thất bại")
			}
		}()
	}
}
