// Package worker — AIDecisionConsumerWorker consume decision_events_queue, xử lý event.
//
// Luồng (Vision): event vào queue → worker consume → AI Decision (ResolveOrCreate, context, decision) → proposals → Executor.
package worker

import (
	"context"
	"os"
	"strings"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	cixsvc "meta_commerce/internal/api/cix/service"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIDecisionConsumerWorker worker poll decision_events_queue, xử lý event.
type AIDecisionConsumerWorker struct {
	interval time.Duration
}

// NewAIDecisionConsumerWorker tạo mới.
func NewAIDecisionConsumerWorker(interval time.Duration) *AIDecisionConsumerWorker {
	if interval < 2*time.Second {
		interval = 2 * time.Second
	}
	return &AIDecisionConsumerWorker{interval: interval}
}

// Start chạy worker. Implement worker.Worker.
func (w *AIDecisionConsumerWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	log.WithField("interval", w.interval.String()).Info("📋 [AI_DECISION] Starting AI Decision Consumer Worker...")

	svc := aidecisionsvc.NewAIDecisionService()

	workerID := "aidecision-consumer-1"
	leaseSec := 60

	for {
		if !worker.IsWorkerActive(worker.WorkerAIDecisionConsumer) {
			select {
			case <-ctx.Done():
				log.Info("📋 [AI_DECISION] AI Decision Consumer Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerAIDecisionConsumer, worker.PriorityHigh)
		if worker.ShouldThrottle(p) {
			interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAIDecisionConsumer, w.interval, 1)
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			continue
		}

		interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAIDecisionConsumer, w.interval, 1)
		select {
		case <-ctx.Done():
			log.Info("📋 [AI_DECISION] AI Decision Consumer Worker stopped")
			return
		case <-time.After(interval):
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [AI_DECISION] Panic khi xử lý event")
				}
			}()

			// Lane ưu tiên: fast → normal → batch
			for _, lane := range []string{aidecisionmodels.EventLaneFast, aidecisionmodels.EventLaneNormal, aidecisionmodels.EventLaneBatch} {
				evt, err := svc.LeaseOne(ctx, lane, workerID, leaseSec)
				if err != nil || evt == nil {
					continue
				}
				processErr := processEvent(ctx, svc, evt)
				if processErr != nil {
					retryable := true
					_ = svc.FailEvent(ctx, evt.EventID, retryable, processErr.Error())
					log.WithError(processErr).WithField("eventId", evt.EventID).Warn("📋 [AI_DECISION] Xử lý event thất bại")
				} else {
					_ = svc.CompleteEvent(ctx, evt.EventID)
				}
				// Chỉ xử lý 1 event mỗi tick
				return
			}
		}()
	}
}

// processEvent xử lý theo event_type.
func processEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	switch evt.EventType {
	case "conversation.message_inserted", "message.batch_ready":
		return processSourceEvent(ctx, svc, evt, false)
	case "conversation.inserted", "conversation.updated", "message.inserted", "message.updated":
		return processSyncedSourceWithDebounce(ctx, svc, evt)
	case "order.inserted", "order.updated":
		return processOrderEvent(ctx, svc, evt)
	case "cix.analysis_completed":
		return processCixAnalysisCompleted(ctx, svc, evt)
	case "customer.context_ready":
		return processCustomerContextReady(ctx, svc, evt)
	case "order.flags_emitted":
		return processOrderFlagsEmitted(ctx, svc, evt)
	case "ads.context_ready":
		return processAdsContextReady(ctx, svc, evt)
	case aidecisionsvc.EventTypeAdsProposeRequested:
		return processAdsProposeRequested(ctx, svc, evt)
	default:
		return nil
	}
}

// processSyncedSourceWithDebounce: bản ghi đồng bộ từ nguồn (conversation/message trên Mongo). Debounce: upsert state → message.batch_ready.
func processSyncedSourceWithDebounce(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	svc.HydrateDatachangedPayload(ctx, evt)
	debounceEnabled := strings.TrimSpace(strings.ToLower(os.Getenv("AI_DECISION_DEBOUNCE_ENABLED"))) == "true"

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
		return nil
	}

	if debounceEnabled {
		shouldFlush, err := svc.UpsertDebounceState(ctx, evt.OrgID, ownerOrgID, convID, custID, channel, normalizedRecordUid, evt.Payload)
		if err != nil {
			return err
		}
		if shouldFlush {
			// Critical pattern → xử lý ngay, không chờ debounce
			return processSourceEvent(ctx, svc, evt, true)
		}
		// Đã upsert debounce, worker sẽ emit message.batch_ready khi hết window
		return nil
	}

	return processSourceEvent(ctx, svc, evt, true)
}

// processOrderEvent xử lý order.inserted, order.updated — từ bản ghi chuẩn hóa đơn hàng (UpsertNormalizedPosOrder).
// Phase 1: no-op, event đã vào queue cho Order Intelligence / context aggregation sau.
func processOrderEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	svc.HydrateDatachangedPayload(ctx, evt)
	return nil
}

// skipHydrate: true khi processSyncedSourceWithDebounce đã gọi HydrateDatachangedPayload (tránh FindOne trùng).
func processSourceEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent, skipHydrate bool) error {
	if !skipHydrate {
		svc.HydrateDatachangedPayload(ctx, evt)
	}
	convID := ""
	if conv, ok := evt.Payload["conversationId"].(string); ok {
		convID = conv
	}
	custID := ""
	if cust, ok := evt.Payload["customerId"].(string); ok {
		custID = cust
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
		return nil
	}

	// ResolveOrCreate case
	_, _, err := svc.ResolveOrCreate(ctx, &aidecisionsvc.ResolveOrCreateInput{
		EventID:    evt.EventID,
		EventType:  evt.EventType,
		OrgID:      evt.OrgID,
		OwnerOrgID: ownerOrgID,
		EntityRefs: aidecisionmodels.DecisionCaseEntityRefs{
			ConversationID: convID,
			CustomerID:     custID,
		},
		CaseType:      aidecisionmodels.CaseTypeConversationResponse,
		RequiredCtx:   []string{"cix", "customer"},
		Priority:      evt.Priority,
		Urgency:       "realtime",
		TraceID:       evt.TraceID,
		CorrelationID: evt.CorrelationID,
	})
	if err != nil {
		return err
	}

	// Emit customer.context_requested — CRM worker sẽ load customer và emit customer.context_ready
	if custID != "" {
		_, _ = svc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
			EventType:     "customer.context_requested",
			EventSource:   "aidecision",
			EntityType:    "customer",
			EntityID:      custID,
			OrgID:         evt.OrgID,
			OwnerOrgID:    ownerOrgID,
			Priority:      "high",
			Lane:          aidecisionmodels.EventLaneFast,
			TraceID:       evt.TraceID,
			CorrelationID: evt.CorrelationID,
			Payload: map[string]interface{}{
				"conversationId": convID,
				"customerId":     custID,
				"channel":        channel,
			},
		})
	}

	// Bridge CIX: luôn emit cix.analysis_requested — CixRequestWorker consume → EnqueueAnalysis → cix_pending_analysis.
	if convID == "" {
		return nil
	}
	_, _ = svc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
		EventType:     "cix.analysis_requested",
		EventSource:   "aidecision",
		EntityType:    "conversation",
		EntityID:      convID,
		OrgID:         evt.OrgID,
		OwnerOrgID:    ownerOrgID,
		Priority:      "high",
		Lane:          aidecisionmodels.EventLaneFast,
		TraceID:       evt.TraceID,
		CorrelationID: evt.CorrelationID,
		Payload: map[string]interface{}{
			"conversationId":      convID,
			"customerId":          custID,
			"channel":             channel,
			"normalizedRecordUid": normalizedRecordUid,
		},
	})
	return nil
}

// processCixAnalysisCompleted xử lý cix.analysis_completed — fetch result, gọi ReceiveCixPayload.
func processCixAnalysisCompleted(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	analysisID := ""
	if id, ok := evt.Payload["analysisResultId"].(string); ok {
		analysisID = id
	}
	if analysisID == "" {
		return nil
	}

	analysisSvc, err := cixsvc.NewCixAnalysisService()
	if err != nil {
		return err
	}

	oid, err := primitive.ObjectIDFromHex(analysisID)
	if err != nil {
		return err
	}

	result, err := analysisSvc.FindOneById(ctx, oid)
	if err != nil {
		return err
	}

	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		ownerOrgID = result.OwnerOrganizationID
	}

	return svc.ReceiveCixPayload(ctx, &result, ownerOrgID)
}

// processCustomerContextReady cập nhật case với customer context, gọi TryExecuteIfReady.
func processCustomerContextReady(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	convID := ""
	if c, ok := evt.Payload["conversationId"].(string); ok {
		convID = c
	}
	custID := ""
	if c, ok := evt.Payload["customerId"].(string); ok {
		custID = c
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	customerPayload, _ := evt.Payload["customer"].(map[string]interface{})
	if customerPayload == nil {
		customerPayload = evt.Payload
	}
	if err := svc.UpdateCaseWithCustomerContext(ctx, convID, custID, evt.OrgID, ownerOrgID, customerPayload); err != nil {
		return err
	}
	return svc.TryExecuteIfReady(ctx, convID, custID, evt.OrgID, ownerOrgID)
}

// processOrderFlagsEmitted cập nhật case với order flags, gọi TryExecuteIfReady nếu case cần order context.
func processOrderFlagsEmitted(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
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
	flags, _ := evt.Payload["flags"].([]interface{})
	orderPayload := map[string]interface{}{"flags": flags}
	if err := svc.UpdateCaseWithOrderContext(ctx, orderID, custID, convID, evt.OrgID, ownerOrgID, orderPayload); err != nil {
		return err
	}
	// Chỉ TryExecuteIfReady khi có conv/cust (case conversation_response)
	if convID != "" && custID != "" {
		return svc.TryExecuteIfReady(ctx, convID, custID, evt.OrgID, ownerOrgID)
	}
	return nil
}

// processAdsContextReady cập nhật case với ads context, gọi TryExecuteIfReady nếu case cần ads context.
func processAdsContextReady(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	adAccountID := ""
	if a, ok := evt.Payload["adAccountId"].(string); ok {
		adAccountID = a
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	adsPayload, _ := evt.Payload["ads"].(map[string]interface{})
	if adsPayload == nil {
		adsPayload = evt.Payload
	}
	if err := svc.UpdateCaseWithAdsContext(ctx, adAccountID, evt.OrgID, ownerOrgID, adsPayload); err != nil {
		return err
	}
	// Case ads_optimization không dùng TryExecuteIfReady (logic khác). Bỏ qua.
	return nil
}

// processAdsProposeRequested xử lý ads.propose_requested — gọi ProposeForAds (Vision 08 event-driven).
func processAdsProposeRequested(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		if hex, ok := evt.Payload["ownerOrgIdHex"].(string); ok && hex != "" {
			oid, err := primitive.ObjectIDFromHex(hex)
			if err == nil {
				ownerOrgID = oid
			}
		}
	}
	if ownerOrgID.IsZero() {
		return nil
	}
	baseURL, _ := evt.Payload["baseURL"].(string)
	proposeInput, err := aidecisionsvc.ParseProposeInputFromEventPayload(evt.Payload)
	if err != nil {
		return err
	}
	// Vision 08: chỉ AI Decision gán decisionId, contextSnapshot — Ads gửi raw payload
	aidecisionsvc.EnrichProposeInputWithTrace("ads", &proposeInput)
	_, err = aidecisionsvc.ProposeForAds(ctx, proposeInput, ownerOrgID, baseURL)
	return err
}
