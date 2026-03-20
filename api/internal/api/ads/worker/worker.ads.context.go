// Package worker — AdsContextWorker consume ads.context_requested, load ads config/context, emit ads.context_ready.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT. Domain worker: Work Request → Result.
package worker

import (
	"context"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AdsContextWorker consume ads.context_requested → load ads context → emit ads.context_ready.
type AdsContextWorker struct {
	interval time.Duration
}

// NewAdsContextWorker tạo mới.
func NewAdsContextWorker(interval time.Duration) *AdsContextWorker {
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	return &AdsContextWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *AdsContextWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [ADS_CONTEXT] Starting Ads Context Worker...")

	decSvc := aidecisionsvc.NewAIDecisionService()
	workerID := "ads-context-1"
	leaseSec := 60
	lane := aidecisionmodels.EventLaneBatch

	for {
		if !worker.IsWorkerActive(worker.WorkerAdsContext) {
			select {
			case <-ctx.Done():
				log.Info("📋 [ADS_CONTEXT] Ads Context Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerAdsContext, worker.PriorityLow)
		if worker.ShouldThrottle(p) {
			interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAdsContext, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAdsContext, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [ADS_CONTEXT] Ads Context Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [ADS_CONTEXT] Panic khi xử lý ads.context_requested")
				}
			}()

			evt, err := decSvc.LeaseOneByEventType(ctx, "ads.context_requested", lane, workerID, leaseSec)
			if err != nil || evt == nil {
				return
			}

			processErr := processAdsContextRequested(ctx, decSvc, evt)
			if processErr != nil {
				retryable := true
				_ = decSvc.FailEvent(ctx, evt.EventID, retryable, processErr.Error())
				log.WithError(processErr).WithField("eventId", evt.EventID).Warn("📋 [ADS_CONTEXT] Xử lý ads.context_requested thất bại")
			} else {
				_ = decSvc.CompleteEvent(ctx, evt.EventID)
			}
		}()
	}
}

func processAdsContextRequested(ctx context.Context, decSvc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	adAccountID := ""
	if a, ok := evt.Payload["adAccountId"].(string); ok {
		adAccountID = a
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}

	// Phase 2 stub: load ads config cho org (sau này gọi ads meta_config, campaign summary).
	// Hiện tại emit context rỗng để AI Decision không chờ vô hạn.
	adsPayload := loadAdsContext(ctx, adAccountID, ownerOrgID)

	return emitAdsContextReady(ctx, decSvc, evt, adAccountID, ownerOrgID, adsPayload)
}

func loadAdsContext(ctx context.Context, adAccountID string, ownerOrgID primitive.ObjectID) map[string]interface{} {
	// Stub: trả context rỗng. Phase 2+ tích hợp ads meta_config, campaign metrics.
	return map[string]interface{}{
		"found":       false,
		"adAccountId": adAccountID,
	}
}

func emitAdsContextReady(ctx context.Context, decSvc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent, adAccountID string, ownerOrgID primitive.ObjectID, adsPayload map[string]interface{}) error {
	_, err := decSvc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
		EventType:     "ads.context_ready",
		EventSource:   "ads",
		EntityType:    "ad_account",
		EntityID:      adAccountID,
		OrgID:         evt.OrgID,
		OwnerOrgID:    ownerOrgID,
		Priority:      "normal",
		Lane:          aidecisionmodels.EventLaneBatch,
		TraceID:       evt.TraceID,
		CorrelationID: evt.CorrelationID,
		Payload: map[string]interface{}{
			"adAccountId": adAccountID,
			"ads":         adsPayload,
		},
	})
	return err
}
