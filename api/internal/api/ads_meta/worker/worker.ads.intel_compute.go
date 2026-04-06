// Package worker — AdsIntelComputeWorker poll ads_intel_compute; tính toán trong metasvc (domain ads, không trong consumer AI Decision).
package worker

import (
	"context"
	"time"

	adsmodels "meta_commerce/internal/api/ads_meta/models"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	wk "meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AdsIntelComputeWorker worker domain ads — xử lý job từ ads_intel_compute.
type AdsIntelComputeWorker struct {
	interval time.Duration
}

// NewAdsIntelComputeWorker tạo mới.
func NewAdsIntelComputeWorker(interval time.Duration) *AdsIntelComputeWorker {
	if interval < 2*time.Second {
		interval = 3 * time.Second
	}
	return &AdsIntelComputeWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *AdsIntelComputeWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [ADS_INTEL] Starting Ads Intel Compute Worker (ads_intel_compute)...")

	for {
		if !wk.IsWorkerActive(wk.WorkerAdsIntelCompute) {
			select {
			case <-ctx.Done():
				log.Info("📋 [ADS_INTEL] Ads Intel Compute Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := wk.GetPriority(wk.WorkerAdsIntelCompute, wk.PriorityHigh)
		if wk.ShouldThrottle(p) {
			interval, _ := wk.GetEffectiveWorkerSchedule(wk.WorkerAdsIntelCompute, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := wk.GetEffectiveWorkerSchedule(wk.WorkerAdsIntelCompute, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [ADS_INTEL] Ads Intel Compute Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [ADS_INTEL] Panic khi xử lý job")
				}
			}()

			coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsIntelCompute)
			if !ok {
				return
			}

			claim := -time.Now().UnixMilli()
			filter := bson.M{"$or": []bson.M{
				{"processedAt": bson.M{"$exists": false}},
				{"processedAt": nil},
			}}
			var job adsmodels.AdsIntelComputeJob
			err := coll.FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"processedAt": claim}},
				options.FindOneAndUpdate().
					SetSort(bson.D{{Key: "createdAt", Value: 1}}).
					SetReturnDocument(options.After),
			).Decode(&job)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					return
				}
				log.WithError(err).Warn("📋 [ADS_INTEL] Claim job thất bại")
				return
			}

			runErr := metasvc.RunAdsIntelComputeJob(ctx, &job)
			if runErr != nil {
				_, uerr := coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{
					"$set": bson.M{
						"processedAt":  nil,
						"processError": runErr.Error(),
					},
					"$inc": bson.M{"retryCount": 1},
				})
				if uerr != nil {
					log.WithError(uerr).WithField("jobId", job.ID.Hex()).Warn("📋 [ADS_INTEL] Ghi lỗi job thất bại")
				}
				log.WithError(runErr).WithField("jobId", job.ID.Hex()).Warn("📋 [ADS_INTEL] RunAdsIntelComputeJob thất bại")
				return
			}

			now := time.Now().UnixMilli()
			_, uerr := coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{"$set": bson.M{
				"processedAt":  now,
				"processError": "",
			}})
			if uerr != nil {
				log.WithError(uerr).WithField("jobId", job.ID.Hex()).Warn("📋 [ADS_INTEL] Đánh dấu hoàn thành thất bại")
			}
		}()
	}
}
