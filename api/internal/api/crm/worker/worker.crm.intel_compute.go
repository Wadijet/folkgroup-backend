// Package worker — CrmIntelComputeWorker poll crm_intel_compute; tính CRM Intelligence tại domain CRM (không trong consumer AI Decision).
package worker

import (
	"context"
	"strings"
	"time"

	"meta_commerce/internal/api/aidecision/decisionlive"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	wk "meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CrmIntelComputeWorker worker domain CRM — xử lý job từ crm_intel_compute.
type CrmIntelComputeWorker struct {
	interval time.Duration
}

// NewCrmIntelComputeWorker tạo mới.
func NewCrmIntelComputeWorker(interval time.Duration) *CrmIntelComputeWorker {
	if interval < 2*time.Second {
		interval = 3 * time.Second
	}
	return &CrmIntelComputeWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *CrmIntelComputeWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [CRM_INTEL] Starting CRM Intel Compute Worker (crm_intel_compute)...")

	for {
		if !wk.IsWorkerActive(wk.WorkerCrmIntelCompute) {
			select {
			case <-ctx.Done():
				log.Info("📋 [CRM_INTEL] CRM Intel Compute Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := wk.GetPriority(wk.WorkerCrmIntelCompute, wk.PriorityHigh)
		if wk.ShouldThrottle(p) {
			interval, _ := wk.GetEffectiveWorkerSchedule(wk.WorkerCrmIntelCompute, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := wk.GetEffectiveWorkerSchedule(wk.WorkerCrmIntelCompute, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [CRM_INTEL] CRM Intel Compute Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [CRM_INTEL] Panic khi xử lý job")
				}
			}()

			coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CustomerIntelCompute)
			if !ok {
				return
			}

			claim := -time.Now().UnixMilli()
			filter := bson.M{"$or": []bson.M{
				{"processedAt": bson.M{"$exists": false}},
				{"processedAt": nil},
			}}
			var job crmmodels.CrmIntelComputeJob
			err := coll.FindOneAndUpdate(ctx, filter, bson.M{"$set": bson.M{"processedAt": claim}},
				options.FindOneAndUpdate().
					SetSort(bson.D{{Key: "createdAt", Value: 1}}).
					SetReturnDocument(options.After),
			).Decode(&job)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					return
				}
				log.WithError(err).Warn("📋 [CRM_INTEL] Claim job thất bại")
				return
			}

			tid, cid := crmIntelTraceFromJobPayload(job.Payload)
			op, _ := job.Payload["operation"].(string)
			extra := map[string]string{"jobId": job.ID.Hex(), "operation": strings.TrimSpace(op)}
			if u, ok := job.Payload["unifiedId"].(string); ok && strings.TrimSpace(u) != "" {
				extra["unifiedId"] = strings.TrimSpace(u)
			}
			if tid != "" {
				decisionlive.PublishIntelDomainMilestone(job.OwnerOrganizationID, tid, cid, decisionlive.IntelDomainCRMIntel, decisionlive.IntelMilestoneStart,
					"Worker CRM Intel: bắt đầu tính / làm mới intelligence khách.",
					[]string{"Job crm_intel_compute chạy tại domain CRM — sau khi xong sẽ emit crm_intel_recomputed về AI Decision (cùng trace nếu có)."},
					extra)
			}

			runErr := crmvc.RunCrmIntelComputeJob(ctx, &job)
			if runErr != nil {
				_, uerr := coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{
					"$set": bson.M{
						"processedAt":  nil,
						"processError": runErr.Error(),
					},
					"$inc": bson.M{"retryCount": 1},
				})
				if uerr != nil {
					log.WithError(uerr).WithField("jobId", job.ID.Hex()).Warn("📋 [CRM_INTEL] Ghi lỗi job thất bại")
				}
				log.WithError(runErr).WithField("jobId", job.ID.Hex()).Warn("📋 [CRM_INTEL] RunCrmIntelComputeJob thất bại")
				if tid != "" {
					msg := strings.TrimSpace(runErr.Error())
					if len(msg) > 400 {
						msg = msg[:400] + "…"
					}
					decisionlive.PublishIntelDomainMilestone(job.OwnerOrganizationID, tid, cid, decisionlive.IntelDomainCRMIntel, decisionlive.IntelMilestoneError,
						"Worker CRM Intel: lỗi khi chạy job.",
						[]string{"Chi tiết (rút gọn): " + msg},
						extra)
				}
				return
			}

			now := time.Now().UnixMilli()
			_, uerr := coll.UpdateOne(ctx, bson.M{"_id": job.ID}, bson.M{"$set": bson.M{
				"processedAt":  now,
				"processError": "",
			}})
			if uerr != nil {
				log.WithError(uerr).WithField("jobId", job.ID.Hex()).Warn("📋 [CRM_INTEL] Đánh dấu hoàn thành thất bại")
			}
			if tid != "" {
				decisionlive.PublishIntelDomainMilestone(job.OwnerOrganizationID, tid, cid, decisionlive.IntelDomainCRMIntel, decisionlive.IntelMilestoneDone,
					"Worker CRM Intel: hoàn tất job.",
					[]string{"Đã chạy thao tác intelligence trên CRM; kiểm tra crm_intel_recomputed trên cùng luồng trace."},
					extra)
			}
		}()
	}
}

// crmIntelTraceFromJobPayload đọc traceId / correlationId do EnqueueCrmIntelComputeFromDecisionEvent ghi vào payload.
func crmIntelTraceFromJobPayload(p map[string]interface{}) (traceID, correlationID string) {
	if p == nil {
		return "", ""
	}
	if s, ok := p["traceId"].(string); ok {
		traceID = strings.TrimSpace(s)
	}
	if s, ok := p["correlationId"].(string); ok {
		correlationID = strings.TrimSpace(s)
	}
	return traceID, correlationID
}
