// Package worker — OrderContextWorker consume order.recompute_requested, tính flags đơn hàng, emit order.flags_emitted.
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

// OrderContextWorker consume order.recompute_requested → load order, tính flags → emit order.flags_emitted.
type OrderContextWorker struct {
	interval time.Duration
}

// NewOrderContextWorker tạo mới.
func NewOrderContextWorker(interval time.Duration) *OrderContextWorker {
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	return &OrderContextWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *OrderContextWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [ORDER_CONTEXT] Starting Order Context Worker...")

	decSvc := aidecisionsvc.NewAIDecisionService()
	workerID := "order-context-1"
	leaseSec := 60
	lane := aidecisionmodels.EventLaneNormal

	for {
		if !worker.IsWorkerActive(worker.WorkerOrderContext) {
			select {
			case <-ctx.Done():
				log.Info("📋 [ORDER_CONTEXT] Order Context Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerOrderContext, worker.PriorityNormal)
		if worker.ShouldThrottle(p) {
			interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerOrderContext, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerOrderContext, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [ORDER_CONTEXT] Order Context Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [ORDER_CONTEXT] Panic khi xử lý order.recompute_requested")
				}
			}()

			evt, err := decSvc.LeaseOneByEventType(ctx, "order.recompute_requested", lane, workerID, leaseSec)
			if err != nil || evt == nil {
				return
			}

			processErr := processOrderRecomputeRequested(ctx, decSvc, evt)
			if processErr != nil {
				retryable := true
				_ = decSvc.FailEvent(ctx, evt.EventID, retryable, processErr.Error())
				log.WithError(processErr).WithField("eventId", evt.EventID).Warn("📋 [ORDER_CONTEXT] Xử lý order.recompute_requested thất bại")
			} else {
				_ = decSvc.CompleteEvent(ctx, evt.EventID)
			}
		}()
	}
}

func processOrderRecomputeRequested(ctx context.Context, decSvc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	orderID := ""
	if o, ok := evt.Payload["orderId"].(string); ok {
		orderID = o
	}
	custID := ""
	if c, ok := evt.Payload["customerId"].(string); ok {
		custID = c
	}
	convID := ""
	if c, ok := evt.Payload["conversationId"].(string); ok {
		convID = c
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}

	// Phase 2 stub: tính flags đơn hàng (sau này gọi report/meta evaluation).
	// Hiện tại emit flags rỗng để AI Decision không chờ vô hạn.
	flags := computeOrderFlags(ctx, orderID, custID, ownerOrgID)

	return emitOrderFlagsEmitted(ctx, decSvc, evt, orderID, custID, convID, ownerOrgID, flags)
}

func computeOrderFlags(ctx context.Context, orderID, custID string, ownerOrgID primitive.ObjectID) []map[string]interface{} {
	// Stub: trả flags rỗng. Phase 2+ tích hợp report order evaluation, meta alert flags.
	return []map[string]interface{}{}
}

func emitOrderFlagsEmitted(ctx context.Context, decSvc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent, orderID, custID, convID string, ownerOrgID primitive.ObjectID, flags []map[string]interface{}) error {
	_, err := decSvc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
		EventType:     "order.flags_emitted",
		EventSource:   "order",
		EntityType:    "order",
		EntityID:      orderID,
		OrgID:         evt.OrgID,
		OwnerOrgID:    ownerOrgID,
		Priority:      "normal",
		Lane:          aidecisionmodels.EventLaneNormal,
		TraceID:      evt.TraceID,
		CorrelationID: evt.CorrelationID,
		Payload: map[string]interface{}{
			"orderId":        orderID,
			"customerId":    custID,
			"conversationId": convID,
			"flags":         flags,
		},
	})
	return err
}
