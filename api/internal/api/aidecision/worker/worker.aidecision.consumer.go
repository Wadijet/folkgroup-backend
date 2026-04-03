// Package worker — AIDecisionConsumerWorker consume decision_events_queue, xử lý event.
//
// Luồng (Vision): event vào queue → worker consume → AI Decision (ResolveOrCreate, context, decision) → proposals → Executor.
//
// Nguyên tắc: pipeline raw → các lớp (L1/L2/L3) → flag/metrics chỉ do worker domain (crm_intel_compute, order_intel_compute, ads_intel_compute, cix_intel_compute, …).
// Consumer AI Decision chỉ enqueue job domain / điều phối case / merge payload đã có — không đọc meta_campaigns để dựng Intelligence, không gọi RefreshMetrics/AnalyzeSession/ApplyAdsIntelligence.
package worker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"meta_commerce/internal/api/aidecision/adsautop"
	"meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/eventintake"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	cixsvc "meta_commerce/internal/api/cix/service"
	crmvc "meta_commerce/internal/api/crm/service"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/approval"
	"meta_commerce/internal/logger"
	"meta_commerce/internal/traceutil"
	"meta_commerce/internal/utility"
	"meta_commerce/internal/worker"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AIDecisionConsumerWorker worker poll decision_events_queue, xử lý event.
type AIDecisionConsumerWorker struct {
	interval time.Duration
}

// NewAIDecisionConsumerWorker tạo mới.
func NewAIDecisionConsumerWorker(interval time.Duration) *AIDecisionConsumerWorker {
	// Sàn thấp để WORKER_AI_DECISION_CONSUMER_INTERVAL / schedule có thể giảm (ví dụ 1s); luồng busy dùng busy-poll riêng.
	if interval < 500*time.Millisecond {
		interval = 500 * time.Millisecond
	}
	return &AIDecisionConsumerWorker{interval: interval}
}

// parseAIDecisionConsumerBusyPollInterval nghỉ ngắn sau khi đã xử lý batch (queue còn khả năng đầy — tránh chờ hết interval idle).
func parseAIDecisionConsumerBusyPollInterval() time.Duration {
	d := 200 * time.Millisecond
	if v := strings.TrimSpace(os.Getenv("AI_DECISION_CONSUMER_BUSY_POLL_INTERVAL")); v != "" {
		if x, err := time.ParseDuration(v); err == nil && x >= 50*time.Millisecond && x <= 30*time.Second {
			d = x
		}
	}
	return d
}

// parseAIDecisionConsumerBurstMaxRounds số vòng lease+process tối đa liên tiếp khi mỗi vòng đủ pool (xả hàng nhanh).
func parseAIDecisionConsumerBurstMaxRounds() int {
	n := 24
	if v := strings.TrimSpace(os.Getenv("AI_DECISION_CONSUMER_BURST_MAX_ROUNDS")); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x >= 1 && x <= 500 {
			n = x
		}
	}
	return n
}

// Start chạy worker. Implement worker.Worker.
func (w *AIDecisionConsumerWorker) Start(ctx context.Context) {
	log := logger.GetAppLogger()
	minPool := worker.MinPoolSizeAIDecisionConsumer()
	defPool := worker.GetPoolSize(worker.WorkerAIDecisionConsumer, minPool)
	log.WithFields(map[string]interface{}{
		"interval":               w.interval.String(),
		"consumerMinPool":        minPool,
		"poolSizeDefault":        defPool,
		"busyPollDefault":        parseAIDecisionConsumerBusyPollInterval().String(),
		"burstMaxRoundsDefault":  parseAIDecisionConsumerBurstMaxRounds(),
		"resourceThrottleBypass": worker.IsWorkerBypassingResourceThrottle(worker.WorkerAIDecisionConsumer),
	}).Info("📋 [AI_DECISION] Starting AI Decision Consumer Worker (pool + burst + poll thích ứng)...")

	svc := aidecisionsvc.NewAIDecisionService()

	leaseSec := 60
	const maxFairOrgHistory = 5
	var recentFairOrgs []primitive.ObjectID
	var lastEscalate time.Time
	var lastNoLeaseLog time.Time
	var lastInactiveHintLog time.Time
	var lastThrottleHintLog time.Time

	busyPollBase := parseAIDecisionConsumerBusyPollInterval()
	maxBurstRounds := parseAIDecisionConsumerBurstMaxRounds()
	lanes := []string{aidecisionmodels.EventLaneFast, aidecisionmodels.EventLaneNormal, aidecisionmodels.EventLaneBatch}

	// 0 = lần đầu chạy ngay; sau mỗi lần xử lý gán idle/busy cho lần chờ kế tiếp.
	nextSleep := time.Duration(0)

	for {
		if !worker.IsWorkerActive(worker.WorkerAIDecisionConsumer) {
			nextSleep = 0
			if time.Since(lastInactiveHintLog) >= 2*time.Minute {
				lastInactiveHintLog = time.Now()
				log.Warn("📋 [AI_DECISION] Consumer đang TẮT (IsWorkerActive=false). Bật: API worker-config workerActive hoặc env WORKER_ACTIVE_AI_DECISION_CONSUMER=true")
			}
			select {
			case <-ctx.Done():
				log.Info("📋 [AI_DECISION] AI Decision Consumer Worker stopped")
				return
			case <-time.After(1 * time.Minute):
			}
			continue
		}
		p := worker.GetPriority(worker.WorkerAIDecisionConsumer, worker.PriorityCritical)
		if worker.ShouldThrottleWorker(worker.WorkerAIDecisionConsumer, p) {
			nextSleep = 0
			if time.Since(lastThrottleHintLog) >= 1*time.Minute {
				lastThrottleHintLog = time.Now()
				log.WithField("priority", int(p)).Warn("📋 [AI_DECISION] Consumer bị bỏ qua chu kỳ do WorkerController (pause/throttle). Gợi ý: WORKER_AI_DECISION_CONSUMER_IGNORE_RESOURCE_THROTTLE=1 hoặc WORKER_PRIORITY_AI_DECISION_CONSUMER=1")
			}
			interval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAIDecisionConsumer, w.interval, 1)
			throttleSleep := worker.GetEffectiveIntervalForWorker(worker.WorkerAIDecisionConsumer, interval, p)
			select {
			case <-ctx.Done():
				return
			case <-time.After(throttleSleep):
			}
			continue
		}

		if nextSleep > 0 {
			select {
			case <-ctx.Done():
				log.Info("📋 [AI_DECISION] AI Decision Consumer Worker stopped")
				return
			case <-time.After(nextSleep):
			}
		}

		hadWork := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.WithField("panic", r).Error("📋 [AI_DECISION] Panic khi xử lý event")
				}
			}()

			if ctx.Err() != nil {
				return
			}

			escEvery := 5 * time.Minute
			if v := strings.TrimSpace(os.Getenv("AI_DECISION_ESCALATE_INTERVAL_SEC")); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					escEvery = time.Duration(n) * time.Second
				}
			}
			flushDeferredDatachangedSideEffects(ctx, svc)

			if time.Since(lastEscalate) >= escEvery {
				lastEscalate = time.Now()
				if n, err := svc.EscalateStalePendingEvents(ctx); err != nil {
					log.WithError(err).Debug("📋 [AI_DECISION] EscalateStalePendingEvents lỗi (bỏ qua)")
				} else if n > 0 {
					log.WithField("count", n).Debug("📋 [AI_DECISION] Đã nâng priority cho event pending quá lâu")
				}
			}

			minP := worker.MinPoolSizeAIDecisionConsumer()
			basePool := worker.GetPoolSize(worker.WorkerAIDecisionConsumer, minP)
			if basePool < minP {
				basePool = minP
			}
			poolSize := worker.GetEffectivePoolSizeForWorker(worker.WorkerAIDecisionConsumer, basePool, p)
			if poolSize < minP {
				poolSize = minP
			}
			if poolSize < 1 {
				poolSize = 1
			}

			type leasedJob struct {
				evt  *aidecisionmodels.DecisionEvent
				lane string
				slot int
			}

			leaseOneBatch := func() []leasedJob {
				var jobs []leasedJob
				for slot := 0; slot < poolSize; slot++ {
					workerID := fmt.Sprintf("aidecision-consumer-%d", slot)
					var got *aidecisionmodels.DecisionEvent
					var gotLane string
					for _, ln := range lanes {
						e, err := svc.LeaseOneFair(ctx, ln, workerID, leaseSec, recentFairOrgs)
						if err != nil || e == nil {
							continue
						}
						recentFairOrgs = append(recentFairOrgs, e.OwnerOrganizationID)
						if len(recentFairOrgs) > maxFairOrgHistory {
							recentFairOrgs = recentFairOrgs[len(recentFairOrgs)-maxFairOrgHistory:]
						}
						got, gotLane = e, ln
						break
					}
					if got == nil {
						break
					}
					jobs = append(jobs, leasedJob{evt: got, lane: gotLane, slot: slot})
				}
				return jobs
			}

			for burst := 0; burst < maxBurstRounds; burst++ {
				if ctx.Err() != nil {
					return
				}

				jobs := leaseOneBatch()
				if len(jobs) == 0 {
					if burst == 0 && decisionlive.MetricsChangeLogEnabled() && time.Since(lastNoLeaseLog) >= 30*time.Second {
						lastNoLeaseLog = time.Now()
						log.Debug("📋 [AI_DECISION] 30s không lease được event nào — kiểm tra worker bật, CPU throttle, scheduledAt/deferred, hoặc toàn bộ queue không khớp lane fast|normal|batch")
					}
					return
				}
				hadWork = true

				var wg sync.WaitGroup
				for _, job := range jobs {
					job := job
					wg.Add(1)
					go func() {
						defer wg.Done()
						defer func() {
							if r := recover(); r != nil {
								log.WithFields(map[string]interface{}{"panic": r, "slot": job.slot}).Error("📋 [AI_DECISION] Panic goroutine consumer")
							}
						}()
						evt, lane := job.evt, job.lane
						ensureDecisionEventTraceIDs(evt)
						if err := svc.PersistDecisionEventTraceFields(ctx, evt); err != nil {
							log.WithError(err).WithField("eventId", evt.EventID).Warn("📋 [AI_DECISION] Không ghi lại traceId/w3cTraceId lên Mongo sau khi bù — tra cứu DB có thể thiếu")
						}
						if decisionlive.MetricsChangeLogEnabled() {
							log.WithFields(map[string]interface{}{
								"eventId":     evt.EventID,
								"eventType":   evt.EventType,
								"eventSource": evt.EventSource,
								"lane":        lane,
								"orgHex":      evt.OwnerOrganizationID.Hex(),
								"traceId":     evt.TraceID,
								"poolSlot":    job.slot,
							}).Debug("📋 [AI_DECISION] Đã lease event — bắt đầu processEvent")
						}
						oid := ownerOrgIDFromDecisionEvent(evt)
						decisionlive.RecordConsumerWorkBegin(oid, evt.EventType, evt.TraceID)
						publishQueueConsumerLifecycleStart(oid, evt)
						t0 := time.Now()
						completionKind, processErr := processEvent(ctx, svc, evt)
						publishQueueConsumerLifecycleEnd(oid, evt, processErr, completionKind)
						durMs := time.Since(t0).Milliseconds()
						decisionlive.RecordConsumerCompletion(oid, evt.EventType, evt.TraceID, processErr == nil, durMs, completionKind)
						if processErr != nil {
							retryable := true
							_ = svc.FailEvent(ctx, evt.EventID, retryable, processErr.Error())
							log.WithError(processErr).WithField("eventId", evt.EventID).Warn("📋 [AI_DECISION] Xử lý event thất bại")
						} else {
							switch completionKind {
							case aidecisionmodels.ConsumerCompletionKindNoHandler:
								_ = svc.CompleteEventWithStatus(ctx, evt.EventID, aidecisionmodels.EventStatusCompletedNoHandler)
							case aidecisionmodels.ConsumerCompletionKindRoutingSkipped:
								_ = svc.CompleteEventWithStatus(ctx, evt.EventID, aidecisionmodels.EventStatusCompletedRoutingSkipped)
							default:
								_ = svc.CompleteEvent(ctx, evt.EventID)
							}
						}
					}()
				}
				wg.Wait()

				// Hết hàng hoặc chưa đủ pool — dừng burst, chờ idle/busy.
				if len(jobs) < poolSize {
					return
				}
			}
		}()

		if ctx.Err() != nil {
			log.Info("📋 [AI_DECISION] AI Decision Consumer Worker stopped")
			return
		}

		idleInterval, _ := worker.GetEffectiveWorkerSchedule(worker.WorkerAIDecisionConsumer, w.interval, 1)
		idleSleep := worker.GetEffectiveIntervalForWorker(worker.WorkerAIDecisionConsumer, idleInterval, p)
		busySleep := worker.GetEffectiveIntervalForWorker(worker.WorkerAIDecisionConsumer, busyPollBase, p)
		if busySleep > idleSleep {
			busySleep = idleSleep
		}
		if hadWork {
			nextSleep = busySleep
		} else {
			nextSleep = idleSleep
		}
	}
}

// ensureDecisionEventTraceIDs gán traceId/correlationId khi thiếu (bản ghi queue cũ hoặc emit không truyền).
func ensureDecisionEventTraceIDs(evt *aidecisionmodels.DecisionEvent) {
	if evt == nil {
		return
	}
	if strings.TrimSpace(evt.TraceID) == "" {
		evt.TraceID = utility.GenerateUID(utility.UIDPrefixTrace)
	}
	if strings.TrimSpace(evt.CorrelationID) == "" {
		evt.CorrelationID = utility.GenerateUID(utility.UIDPrefixCorrelation)
	}
	if strings.TrimSpace(evt.W3CTraceID) == "" && strings.TrimSpace(evt.TraceID) != "" {
		evt.W3CTraceID = traceutil.W3CTraceIDFromKey(strings.TrimSpace(evt.TraceID))
	}
}

// processEvent xử lý theo event_type — đăng ký tại consumer_dispatch.go (EvaluateCaseStep/rule store sau này).
func processEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) (aidecisionmodels.ConsumerCompletionKind, error) {
	if evt == nil {
		return aidecisionmodels.ConsumerCompletionKindProcessed, nil
	}
	// Vision L1: side-effect từ CRUD (CRM pending ingest, Report MarkDirty, Ads debounce) chỉ chạy trong consumer sau event datachanged.
	if evt.EventSource == "datachanged" {
		_ = applyDatachangedSideEffects(ctx, svc, evt)
		publishQueueDatachangedEffectsDone(ownerOrgIDFromDecisionEvent(evt), evt)
	}
	// decision_routing_rules: behavior noop → không gọi handler đã đăng ký (event vẫn complete).
	if svc.ShouldSkipDispatchForRoutingRule(ctx, ownerOrgIDFromDecisionEvent(evt), evt.EventType) {
		publishQueueRoutingSkipped(ownerOrgIDFromDecisionEvent(evt), evt)
		return aidecisionmodels.ConsumerCompletionKindRoutingSkipped, nil
	}
	return dispatchConsumerEvent(ctx, svc, evt)
}

// ownerOrgIDFromDecisionEvent lấy OwnerOrganizationID từ envelope hoặc payload (ownerOrgIdHex).
func ownerOrgIDFromDecisionEvent(evt *aidecisionmodels.DecisionEvent) primitive.ObjectID {
	if evt == nil {
		return primitive.NilObjectID
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		if hex, ok := evt.Payload["ownerOrgIdHex"].(string); ok && hex != "" {
			if oid, err := primitive.ObjectIDFromHex(hex); err == nil {
				ownerOrgID = oid
			}
		}
	}
	return ownerOrgID
}

// processAdsIntelligenceRecomputeRequested — consumer chỉ enqueue ads_intel_compute; tính toán do worker domain ads.
func processAdsIntelligenceRecomputeRequested(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	objectType, _ := evt.Payload["objectType"].(string)
	objectId, _ := evt.Payload["objectId"].(string)
	adAccountId, _ := evt.Payload["adAccountId"].(string)
	source, _ := evt.Payload["source"].(string)
	recomputeMode, _ := evt.Payload["recomputeMode"].(string)
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		if hex, ok := evt.Payload["ownerOrgIdHex"].(string); ok && hex != "" {
			oid, err := primitive.ObjectIDFromHex(hex)
			if err == nil {
				ownerOrgID = oid
			}
		}
	}
	if objectType == "" || objectId == "" || adAccountId == "" || ownerOrgID.IsZero() {
		return nil
	}
	return metasvc.EnqueueAdsIntelCompute(ctx, objectType, objectId, adAccountId, ownerOrgID, source, recomputeMode, evt.EventID)
}

// processAdsIntelligenceRecalculateAllRequested — consumer chỉ enqueue ads_intel_compute; batch chạy tại worker domain ads.
func processAdsIntelligenceRecalculateAllRequested(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
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
	limit := payloadIntFromDecisionPayload(evt.Payload, "limit")
	return metasvc.EnqueueAdsIntelComputeRecalculateAll(ctx, ownerOrgID, limit, evt.EventID)
}

func payloadIntFromDecisionPayload(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return 0
	}
}

// processCixAnalysisRequested — consumer chỉ enqueue cix_intel_compute (cùng mẫu CRM/Ads/Order); AnalyzeSession chạy tại WorkerCixIntelCompute.
func processCixAnalysisRequested(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	queueSvc, err := cixsvc.NewCixQueueService()
	if err != nil {
		return err
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
	return queueSvc.EnqueueAnalysis(ctx, cixsvc.EnqueueAnalysisInput{
		ConversationID:      convID,
		CustomerID:          custID,
		Channel:             channel,
		CioEventUid:         normalizedRecordUid,
		OwnerOrganizationID: ownerOrgID,
	})
}

// processCrmIntelligenceComputeRequested — consumer chỉ enqueue crm_intel_compute; RefreshMetrics / Recalculate* chạy tại worker domain CRM.
func processCrmIntelligenceComputeRequested(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		if hex, ok := evt.Payload["ownerOrgIdHex"].(string); ok && hex != "" {
			oid, err := primitive.ObjectIDFromHex(hex)
			if err == nil {
				ownerOrgID = oid
			}
		}
	}
	return crmvc.EnqueueCrmIntelComputeFromDecisionEvent(ctx, evt.EventID, ownerOrgID, evt.Payload)
}

// processCrmIntelligenceRecomputeRequested — crm.intelligence.recompute_requested (như ads.intelligence.recompute_requested): debounce theo org+unifiedId rồi xếp crm_intel_compute (refresh); gấp → enqueue ngay.
func processCrmIntelligenceRecomputeRequested(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
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
	unifiedID := strings.TrimSpace(strFromPayload(evt.Payload, "unifiedId"))
	if unifiedID == "" {
		return nil
	}
	payload := map[string]interface{}{
		"operation":     crmqueue.CrmComputeOpRefresh,
		"unifiedId":     unifiedID,
		"ownerOrgIdHex": ownerOrgID.Hex(),
	}
	if eventintake.PayloadMarksIntelUrgent(evt.Payload) {
		return crmvc.EnqueueCrmIntelComputeFromDecisionEvent(ctx, evt.EventID, ownerOrgID, payload)
	}
	win := eventintake.CrmIntelAfterIngestDebounceWindow()
	if win <= 0 {
		return crmvc.EnqueueCrmIntelComputeFromDecisionEvent(ctx, evt.EventID, ownerOrgID, payload)
	}
	eventintake.ScheduleCrmIntelligenceRecomputeDebounce(ownerOrgID.Hex(), unifiedID, win, evt.TraceID, evt.CorrelationID, evt.EventID)
	return nil
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
		shouldFlush, err := svc.UpsertDebounceState(ctx, evt.OrgID, ownerOrgID, convID, custID, channel, normalizedRecordUid, evt.TraceID, evt.CorrelationID, evt.Payload)
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

// processOrderEvent xử lý order.inserted, order.updated — hydrate + CRM + Decision case order_risk + enqueue Order Intelligence (domain worker).
func processOrderEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	return OrchestrateOrderSourceEvent(ctx, svc, evt)
}

// skipHydrate: true khi processSyncedSourceWithDebounce đã gọi HydrateDatachangedPayload (tránh FindOne trùng).
func processSourceEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent, skipHydrate bool) error {
	return OrchestrateConversationSourceEvent(ctx, svc, evt, skipHydrate)
}

// receiveCixAnalysisIntoAID đọc bản ghi cix_analysis_results theo _id hex → ReceiveCixPayload (merge case + TryExecuteIfReady).
func receiveCixAnalysisIntoAID(ctx context.Context, svc *aidecisionsvc.AIDecisionService, analysisID string, ownerOrgID primitive.ObjectID) error {
	analysisID = strings.TrimSpace(analysisID)
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

// processOrderIntelRecomputed — sau worker Order Intelligence (một event duy nhất: flags + layer + orderCompletedTransition).
func processOrderIntelRecomputed(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	orderID := strings.TrimSpace(strFromPayload(evt.Payload, "orderId"))
	if orderID == "" {
		orderID = strings.TrimSpace(strFromPayload(evt.Payload, "orderUid"))
	}
	custID := strings.TrimSpace(strFromPayload(evt.Payload, "customerId"))
	convID := strings.TrimSpace(strFromPayload(evt.Payload, "conversationId"))

	if payloadBoolTrue(evt.Payload, "orderCompletedTransition") {
		logger.GetAppLogger().WithFields(map[string]interface{}{
			"eventId": evt.EventID, "orderUid": orderID, "eventType": evt.EventType,
		}).Debug("📋 [ORDER_INTEL] order_intel_recomputed — đơn vừa chuyển completed; Phase 2 Learning Engine có thể bám payload")
	}

	flags, _ := evt.Payload["flags"].([]interface{})
	orderPayload := map[string]interface{}{"flags": flags}
	if v, ok := evt.Payload["layer1"]; ok {
		orderPayload["layer1"] = v
	}
	if v, ok := evt.Payload["layer2"]; ok {
		orderPayload["layer2"] = v
	}
	if v, ok := evt.Payload["layer3"]; ok {
		orderPayload["layer3"] = v
	}
	if err := svc.UpdateCaseWithOrderContext(ctx, orderID, custID, convID, evt.OrgID, ownerOrgID, orderPayload); err != nil {
		return err
	}
	emittedOrderRisk := false
	if orderID != "" {
		var err error
		emittedOrderRisk, err = svc.TryExecuteOrderRiskIfReady(ctx, orderID, evt.OrgID, ownerOrgID)
		if err != nil {
			return err
		}
	}
	if convID != "" && custID != "" {
		if emittedOrderRisk && !aidecisionsvc.OrderFlagsAllowDualExecute() {
			return nil
		}
		return svc.TryExecuteIfReady(ctx, convID, custID, evt.OrgID, ownerOrgID)
	}
	return nil
}

// processIntelRecomputedForAID — sau CRM intelligence: cập nhật case theo unifiedId + TryExecuteIfReady khi có conv+cust trên payload.
func processIntelRecomputedForAID(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	orgID := strings.TrimSpace(evt.OrgID)
	if orgID == "" {
		orgID = ownerOrgID.Hex()
	}
	unifiedID := strings.TrimSpace(strFromPayload(evt.Payload, "unifiedId"))
	if unifiedID != "" {
		_ = svc.RefreshOpenCasesAfterCrmIntel(ctx, unifiedID, orgID, ownerOrgID)
	}
	convID := strings.TrimSpace(strFromPayload(evt.Payload, "conversationId"))
	custID := strings.TrimSpace(strFromPayload(evt.Payload, "customerId"))
	if convID != "" && custID != "" {
		return svc.TryExecuteIfReady(ctx, convID, custID, orgID, ownerOrgID)
	}
	return nil
}

// processCixIntelRecomputed — sau worker CIX: ReceiveCixPayload theo analysisResultId; event cũ không có id chỉ TryExecuteIfReady.
func processCixIntelRecomputed(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	analysisID := strings.TrimSpace(strFromPayload(evt.Payload, "analysisResultId"))
	if analysisID != "" {
		return receiveCixAnalysisIntoAID(ctx, svc, analysisID, ownerOrgID)
	}
	convID := strings.TrimSpace(strFromPayload(evt.Payload, "conversationId"))
	custID := strings.TrimSpace(strFromPayload(evt.Payload, "customerId"))
	if convID == "" || custID == "" {
		return nil
	}
	return svc.TryExecuteIfReady(ctx, convID, custID, evt.OrgID, ownerOrgID)
}

func strFromPayload(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func payloadBoolTrue(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	default:
		return false
	}
}

// processAdsContextRequested — bước 4 (phần consumer): chỉ enqueue ads_intel_compute jobKind=context_ready.
// Worker domain ads đọc meta_campaigns, đóng gói payload → emit ads.context_ready (không đọc DB nặng tại đây).
func processAdsContextRequested(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	adAccountID := ""
	if a, ok := evt.Payload["adAccountId"].(string); ok {
		adAccountID = a
	}
	campaignID := ""
	if c, ok := evt.Payload["campaignId"].(string); ok {
		campaignID = c
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() || campaignID == "" {
		return nil
	}
	orgID := strings.TrimSpace(evt.OrgID)
	if orgID == "" {
		orgID = ownerOrgID.Hex()
	}
	return metasvc.EnqueueAdsIntelComputeContextReady(ctx, evt.EventID, orgID, evt.TraceID, evt.CorrelationID, campaignID, adAccountID, ownerOrgID)
}

// processAdsContextReady — bước 5: snapshot đã có trong payload (ads.context_ready).
// Gắn context vào case → RunAdsProposeFromContextReady (ACTION_RULE) → có thể ads.propose_requested / executor.
func processAdsContextReady(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	adAccountID := ""
	if a, ok := evt.Payload["adAccountId"].(string); ok {
		adAccountID = a
	}
	campaignID := ""
	if c, ok := evt.Payload["campaignId"].(string); ok {
		campaignID = c
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	adsPayload, _ := evt.Payload["ads"].(map[string]interface{})
	if adsPayload == nil {
		adsPayload = evt.Payload
	}
	if err := svc.UpdateCaseWithAdsContext(ctx, campaignID, evt.OrgID, ownerOrgID, adsPayload); err != nil {
		return err
	}
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://localhost"
	}
	return adsautop.RunAdsProposeFromContextReady(ctx, svc, ownerOrgID, evt.OrgID, campaignID, adAccountID, baseURL, evt)
}

// processExecutorProposeRequested xử lý executor.propose_requested (và ads.propose_requested cũ) — gọi ProposeForAds / approval.Propose (Vision 08: chỉ qua consumer).
func processExecutorProposeRequested(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	_ = svc
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
	domain, _ := evt.Payload["domain"].(string)
	if domain == "" && evt.EventType == aidecisionsvc.EventTypeAdsProposeRequested {
		domain = "ads"
	}
	if domain == "" {
		return nil
	}
	baseURL, _ := evt.Payload["baseURL"].(string)
	proposeInput, err := aidecisionsvc.ParseProposeInputFromEventPayload(evt.Payload)
	if err != nil {
		return err
	}
	aidecisionsvc.MergeQueueEnvelopeIntoProposePayload(evt, &proposeInput)
	aidecisionsvc.EnrichProposeInputWithTrace(domain, &proposeInput)
	if domain == "ads" {
		_, err = aidecisionsvc.ProposeForAds(ctx, proposeInput, ownerOrgID, baseURL)
		return err
	}
	_, err = approval.Propose(ctx, domain, proposeInput, ownerOrgID, baseURL)
	return err
}
