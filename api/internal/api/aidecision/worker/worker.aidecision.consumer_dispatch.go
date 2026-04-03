// Package worker — Đăng ký dispatch event_type → handler (mở rộng rule store sau này).
//
// Intelligence nặng: consumer chỉ enqueue crm_intel_compute / ads_intel_compute / order_intel_compute / cix_intel_compute (tuỳ luồng),
// không đọc meta_campaigns để dựng snapshot Intelligence (ads.context_requested → enqueue; snapshot trong worker domain).
// Không gọi Recalculate/RefreshMetrics/ApplyAdsIntelligence/AnalyzeSession trong handler.
//
// Luồng Ads — bước 3→5 (sau khi Intelligence đã có từ worker recompute):
//
//  3) campaign_intel_recomputed (meta_ads_intel) hoặc legacy meta_campaign.inserted|updated → ProcessMetaCampaignDataChanged:
//     resolve case ads_optimization, cooldown → emit ads.context_requested (chỉ “xin snapshot”, consumer nhẹ).
//  4) ads.context_requested → processAdsContextRequested → EnqueueAdsIntelComputeContextReady;
//     worker ads_intel_compute (job context_ready) đọc DB → emit ads.context_ready (payload ads đã đóng gói).
//  5) ads.context_ready → processAdsContextReady → UpdateCaseWithAdsContext → RunAdsProposeFromContextReady
//     (ACTION_RULE, đề xuất / executor).
package worker

import (
	"context"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/eventtypes"
	"meta_commerce/internal/api/aidecision/intelrecomputed"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	orderintelsvc "meta_commerce/internal/api/orderintel/service"
)

// consumerEventHandler xử lý một DecisionEvent sau intake.
type consumerEventHandler func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error

var consumerEventRegistry map[string]consumerEventHandler

func init() {
	consumerEventRegistry = make(map[string]consumerEventHandler)

	reg := func(eventType string, h consumerEventHandler) {
		consumerEventRegistry[eventType] = h
	}
	regMany := func(types []string, h consumerEventHandler) {
		for _, t := range types {
			reg(t, h)
		}
	}

	regMany(eventtypes.MessageFastPathEventTypes, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return processSourceEvent(ctx, svc, evt, false)
	})
	regMany(eventtypes.ConversationLifecycleEventTypes, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return processSyncedSourceWithDebounce(ctx, svc, evt)
	})
	regMany(eventtypes.OrderLifecycleEventTypes, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return processOrderEvent(ctx, svc, evt)
	})
	reg(orderintelsvc.EventTypeOrderIntelligenceRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return orderintelsvc.EnqueueFromLegacyIntelligenceRequestedDecisionEvent(ctx, evt)
	})
	reg(orderintelsvc.EventTypeOrderRecomputeRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return orderintelsvc.EnqueueFromRecomputeDecisionEvent(ctx, evt)
	})

	// pos_customer / fb_customer / crm_customer: chỉ datachanged → applyDatachangedSideEffects (enqueue crm_pending_ingest), không handler riêng.

	reg(eventtypes.CustomerContextReady, processCustomerContextReady)
	reg(intelrecomputed.EventTypeOrderIntelRecomputed, processOrderIntelRecomputed)
	reg(eventtypes.AdsContextRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return processAdsContextRequested(ctx, evt)
	})
	reg(eventtypes.AdsContextReady, processAdsContextReady)
	reg(aidecisionsvc.EventTypeExecutorProposeRequested, processExecutorProposeRequested)
	reg(aidecisionsvc.EventTypeAdsProposeRequested, processExecutorProposeRequested)
	reg(aidecisionsvc.EventTypeExecuteRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return svc.ProcessExecuteRequestedEvent(ctx, evt)
	})
	regMany(eventtypes.MetaCampaignPipelineHookEventTypes, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return svc.ProcessMetaCampaignDataChanged(ctx, evt)
	})
	// Worker domain CRM — thử TryExecuteIfReady khi payload có conversationId + customerId.
	reg(intelrecomputed.EventTypeCrmIntelRecomputed, processIntelRecomputedForAID)
	// CIX — merge kết quả phân tích vào case (ReceiveCixPayload) rồi TryExecuteIfReady.
	reg(intelrecomputed.EventTypeCixIntelRecomputed, processCixIntelRecomputed)
	reg(aidecisionsvc.EventTypeAdsIntelligenceRecomputeRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return processAdsIntelligenceRecomputeRequested(ctx, evt)
	})
	reg(aidecisionsvc.EventTypeAdsIntelligenceRecalculateAllRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return processAdsIntelligenceRecalculateAllRequested(ctx, evt)
	})
	reg(crmqueue.EventTypeCrmIntelligenceComputeRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return processCrmIntelligenceComputeRequested(ctx, evt)
	})
	reg(crmqueue.EventTypeCrmIntelligenceRecomputeRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return processCrmIntelligenceRecomputeRequested(ctx, evt)
	})
	reg(aidecisionsvc.EventTypeCixAnalysisRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return processCixAnalysisRequested(ctx, evt)
	})
}

// dispatchConsumerEvent tra handler đã đăng ký; không có → no_handler (không lỗi).
func dispatchConsumerEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) (aidecisionmodels.ConsumerCompletionKind, error) {
	if evt == nil {
		return aidecisionmodels.ConsumerCompletionKindProcessed, nil
	}
	h, ok := consumerEventRegistry[evt.EventType]
	if !ok || h == nil {
		publishQueueNoRegisteredHandler(ownerOrgIDFromDecisionEvent(evt), evt)
		return aidecisionmodels.ConsumerCompletionKindNoHandler, nil
	}
	err := h(ctx, svc, evt)
	return aidecisionmodels.ConsumerCompletionKindProcessed, err
}
