// Package worker — CixRequestWorker consume cix.analysis_requested → EnqueueAnalysis.
//
// Bắt buộc cho luồng event-driven: AIDecision consumer luôn emit cix.analysis_requested.
package worker

import (
	"context"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	cixsvc "meta_commerce/internal/api/cix/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"
)

// CixRequestWorker worker consume cix.analysis_requested, gọi EnqueueAnalysis.
type CixRequestWorker struct {
	interval time.Duration
}

// NewCixRequestWorker tạo mới.
func NewCixRequestWorker(interval time.Duration) *CixRequestWorker {
	if interval < 2*time.Second {
		interval = 5 * time.Second
	}
	return &CixRequestWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *CixRequestWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [CIX_REQUEST] Starting CIX Request Worker...")

	decSvc := aidecisionsvc.NewAIDecisionService()
	queueSvc, err := cixsvc.NewCixQueueService()
	if err != nil {
		log.WithError(err).Error("📋 [CIX_REQUEST] Không tạo được CixQueueService")
		return
	}

	workerID := "cix-request-1"
	leaseSec := 60
	lane := aidecisionmodels.EventLaneFast

	for {
		if !worker.IsWorkerActive(worker.WorkerCixRequest) {
			select {
			case <-ctx.Done():
				log.Info("📋 [CIX_REQUEST] CIX Request Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerCixRequest, worker.PriorityNormal)
		if worker.ShouldThrottle(p) {
			interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerCixRequest, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerCixRequest, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [CIX_REQUEST] CIX Request Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [CIX_REQUEST] Panic khi xử lý cix.analysis_requested")
				}
			}()

			evt, err := decSvc.LeaseOneByEventType(ctx, "cix.analysis_requested", lane, workerID, leaseSec)
			if err != nil || evt == nil {
				return
			}

			convID := ""
			if c, ok := evt.Payload["conversationId"].(string); ok {
				convID = c
			}
			custID := ""
			if c, ok := evt.Payload["customerId"].(string); ok {
				custID = c
			}
			channel := "messenger"
			if ch, ok := evt.Payload["channel"].(string); ok {
				channel = ch
			}
			normalizedRecordUid := evt.EntityID
			if u, ok := evt.Payload["normalizedRecordUid"].(string); ok {
				normalizedRecordUid = u
			}
			ownerOrgID := evt.OwnerOrganizationID
			if ownerOrgID.IsZero() {
				_ = decSvc.CompleteEvent(ctx, evt.EventID)
				return
			}

			enqErr := queueSvc.EnqueueAnalysis(ctx, cixsvc.EnqueueAnalysisInput{
				ConversationID:      convID,
				CustomerID:          custID,
				Channel:             channel,
				CioEventUid:         normalizedRecordUid,
				OwnerOrganizationID: ownerOrgID,
			})
			if enqErr != nil {
				_ = decSvc.FailEvent(ctx, evt.EventID, true, enqErr.Error())
				log.WithError(enqErr).WithField("eventId", evt.EventID).Warn("📋 [CIX_REQUEST] EnqueueAnalysis thất bại")
				return
			}
			_ = decSvc.CompleteEvent(ctx, evt.EventID)
		}()
	}
}
