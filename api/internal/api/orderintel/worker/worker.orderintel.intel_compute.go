// Package worker — OrderIntelComputeWorker poll order_intel_compute, tính Raw→L1→L2→L3→Flags tại domain.
package worker

import (
	"context"
	"strings"
	"time"

	"meta_commerce/internal/api/aidecision/decisionlive"
	orderintelmodels "meta_commerce/internal/api/orderintel/models"
	orderintelsvc "meta_commerce/internal/api/orderintel/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	wk "meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OrderIntelComputeWorker worker xử lý job domain Order Intelligence (không tính trong consumer AI Decision).
type OrderIntelComputeWorker struct {
	interval time.Duration
}

// NewOrderIntelComputeWorker tạo mới.
func NewOrderIntelComputeWorker(interval time.Duration) *OrderIntelComputeWorker {
	if interval < 2*time.Second {
		interval = 3 * time.Second
	}
	return &OrderIntelComputeWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *OrderIntelComputeWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [ORDER_INTEL] Starting Order Intel Compute Worker (order_intel_compute)...")

	for {
		if !wk.IsWorkerActive(wk.WorkerOrderIntelCompute) {
			select {
			case <-ctx.Done():
				log.Info("📋 [ORDER_INTEL] Order Intel Compute Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := wk.GetPriority(wk.WorkerOrderIntelCompute, wk.PriorityHigh)
		if wk.ShouldThrottle(p) {
			interval, _ := wk.GetEffectiveWorkerSchedule(wk.WorkerOrderIntelCompute, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := wk.GetEffectiveWorkerSchedule(wk.WorkerOrderIntelCompute, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [ORDER_INTEL] Order Intel Compute Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [ORDER_INTEL] Panic khi xử lý job")
				}
			}()

			coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderIntelCompute)
			if !ok {
				return
			}

			claim := -time.Now().UnixMilli()
			filter := bson.M{"$or": []bson.M{
				{"processedAt": bson.M{"$exists": false}},
				{"processedAt": nil},
			}}
			var job orderintelmodels.OrderIntelComputeJob
			err := coll.FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"processedAt": claim}},
				options.FindOneAndUpdate().
					SetSort(bson.D{{Key: "createdAt", Value: 1}}).
					SetReturnDocument(options.After),
			).Decode(&job)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					return
				}
				log.WithError(err).Warn("📋 [ORDER_INTEL] Claim job thất bại")
				return
			}

			tid := strings.TrimSpace(job.TraceID)
			jobExtra := map[string]string{"jobId": job.ID.Hex()}
			if job.OrderUid != "" {
				jobExtra["orderUid"] = job.OrderUid
			}
			if tid != "" {
				decisionlive.PublishIntelDomainMilestone(job.OwnerOrganizationID, tid, job.CorrelationID, decisionlive.IntelDomainOrderIntel, decisionlive.IntelMilestoneStart,
					"Worker Order Intel: bắt đầu tính Raw → L3 / cờ trên đơn.",
					[]string{"Job order_intel_compute đang chạy tại domain đơn hàng — kết quả sẽ bàn giao về AI Decision."},
					jobExtra)
			}

			runErr := orderintelsvc.RunOrderIntelComputeJob(ctx, &job)
			if runErr != nil {
				_, uerr := coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{
					"$set": bson.M{
						"processedAt":  nil,
						"processError": runErr.Error(),
					},
					"$inc": bson.M{"retryCount": 1},
				})
				if uerr != nil {
					log.WithError(uerr).WithField("jobId", job.ID.Hex()).Warn("📋 [ORDER_INTEL] Ghi lỗi job thất bại")
				}
				log.WithError(runErr).WithField("jobId", job.ID.Hex()).Warn("📋 [ORDER_INTEL] RunOrderIntelComputeJob thất bại")
				if tid != "" {
					msg := strings.TrimSpace(runErr.Error())
					if len(msg) > 400 {
						msg = msg[:400] + "…"
					}
					decisionlive.PublishIntelDomainMilestone(job.OwnerOrganizationID, tid, job.CorrelationID, decisionlive.IntelDomainOrderIntel, decisionlive.IntelMilestoneError,
						"Worker Order Intel: lỗi khi tính intelligence đơn.",
						[]string{"Chi tiết (rút gọn): " + msg},
						jobExtra)
				}
				return
			}

			now := time.Now().UnixMilli()
			_, uerr := coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{"$set": bson.M{
				"processedAt":  now,
				"processError": "",
			}})
			if uerr != nil {
				log.WithError(uerr).WithField("jobId", job.ID.Hex()).Warn("📋 [ORDER_INTEL] Đánh dấu hoàn thành thất bại")
			}
			if tid != "" {
				decisionlive.PublishIntelDomainMilestone(job.OwnerOrganizationID, tid, job.CorrelationID, decisionlive.IntelDomainOrderIntel, decisionlive.IntelMilestoneDone,
					"Worker Order Intel: hoàn tất job tính intelligence.",
					[]string{"Đã cập nhật snapshot intel đơn; có thể đã emit order_intel_recomputed về hàng đợi AI Decision."},
					jobExtra)
			}
		}()
	}
}
