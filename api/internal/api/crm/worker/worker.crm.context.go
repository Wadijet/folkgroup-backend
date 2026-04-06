// Package worker — CrmContextWorker consume customer.context_requested, load customer, emit customer.context_ready.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT. Domain worker: Work Request → Result.
package worker

import (
	"context"
	"time"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CrmContextWorker worker consume customer.context_requested, load customer, emit customer.context_ready.
type CrmContextWorker struct {
	interval time.Duration
}

// NewCrmContextWorker tạo mới.
func NewCrmContextWorker(interval time.Duration) *CrmContextWorker {
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	return &CrmContextWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *CrmContextWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [CRM_CONTEXT] Starting CRM Context Worker...")

	crmSvc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		log.WithError(err).Error("📋 [CRM_CONTEXT] Không tạo được CrmCustomerService")
		return
	}
	decSvc := aidecisionsvc.NewAIDecisionService()

	workerID := "crm-context-1"
	leaseSec := 60
	lane := aidecisionmodels.EventLaneFast

	for {
		if !worker.IsWorkerActive(worker.WorkerCrmContext) {
			select {
			case <-ctx.Done():
				log.Info("📋 [CRM_CONTEXT] CRM Context Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerCrmContext, worker.PriorityNormal)
		if worker.ShouldThrottle(p) {
			interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerCrmContext, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerCrmContext, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [CRM_CONTEXT] CRM Context Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [CRM_CONTEXT] Panic khi xử lý customer.context_requested")
				}
			}()

			evt, err := decSvc.LeaseOneByEventType(ctx, eventtypes.CustomerContextRequested, lane, workerID, leaseSec)
			if err != nil || evt == nil {
				return
			}

			processErr := processCustomerContextRequested(ctx, crmSvc, decSvc, evt)
			if processErr != nil {
				retryable := true
				_ = decSvc.FailEvent(ctx, evt.EventID, retryable, processErr.Error())
				log.WithError(processErr).WithField("eventId", evt.EventID).Warn("📋 [CRM_CONTEXT] Xử lý customer.context_requested thất bại")
			} else {
				_ = decSvc.CompleteEvent(ctx, evt.EventID)
			}
		}()
	}
}

func processCustomerContextRequested(ctx context.Context, crmSvc *crmvc.CrmCustomerService, decSvc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	custID := ""
	if c, ok := evt.Payload["customerId"].(string); ok {
		custID = c
	}
	convID := ""
	if c, ok := evt.Payload["conversationId"].(string); ok {
		convID = c
	}
	channel := "messenger"
	if ch, ok := evt.Payload["channel"].(string); ok {
		channel = ch
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	if custID == "" {
		return nil
	}

	// Tìm customer theo customerId (fb, pos, zalo, unifiedId, uid)
	cust, err := crmSvc.FindOne(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"sourceIds.pos": custID},
			{"sourceIds.fb": custID},
			{"sourceIds.zalo": custID},
			{"sourceIds.allInboxIds": custID},
			{"unifiedId": custID},
			{"uid": custID},
		},
	}, nil)
	if err != nil {
		// Không tìm thấy — emit context rỗng để AI Decision không chờ
		_ = emitCustomerContextReady(ctx, decSvc, evt, convID, custID, channel, ownerOrgID, map[string]interface{}{"found": false})
		return nil
	}

	// Build payload cho AI Decision
	payload := buildCustomerContextPayload(&cust)
	return emitCustomerContextReady(ctx, decSvc, evt, convID, custID, channel, ownerOrgID, payload)
}

func buildCustomerContextPayload(c *crmmodels.CrmCustomer) map[string]interface{} {
	m := map[string]interface{}{
		"found":         true,
		"unifiedId":     c.UnifiedId,
		"uid":           c.Uid,
		"lifecycleStage": c.LifecycleStage,
		"journeyStage":  c.JourneyStage,
		"momentumStage": c.MomentumStage,
		"valueTier":     c.ValueTier,
		"channel":       c.Channel,
		"loyaltyStage":  c.LoyaltyStage,
		"totalSpent":    c.TotalSpent,
		"orderCount":    c.OrderCount,
		"lastOrderAt":   c.LastOrderAt,
		"hasConversation": c.HasConversation,
		"hasOrder":      c.HasOrder,
	}
	if c.CurrentMetrics != nil {
		m["currentMetrics"] = c.CurrentMetrics
	}
	return m
}

func emitCustomerContextReady(ctx context.Context, decSvc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent, convID, custID, channel string, ownerOrgID primitive.ObjectID, customerPayload map[string]interface{}) error {
	_, err := decSvc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
		EventType:   eventtypes.CustomerContextReady,
		EventSource: eventtypes.EventSourceCRM,
		EntityType:  "customer",
		EntityID:    custID,
		OrgID:       evt.OrgID,
		OwnerOrgID:  ownerOrgID,
		Priority:    "high",
		Lane:        aidecisionmodels.EventLaneFast,
		TraceID:     evt.TraceID,
		CorrelationID: evt.CorrelationID,
		Payload: map[string]interface{}{
			"conversationId": convID,
			"customerId":    custID,
			"channel":       channel,
			"customer":      customerPayload,
		},
	})
	return err
}
