// Package worker — Đăng ký dispatch event_type → handler (mở rộng rule store sau này).
package worker

import (
	"context"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
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

	regMany([]string{"conversation.message_inserted", "message.batch_ready"}, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return processSourceEvent(ctx, svc, evt, false)
	})
	regMany([]string{"conversation.inserted", "conversation.updated", "message.inserted", "message.updated"}, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return processSyncedSourceWithDebounce(ctx, svc, evt)
	})
	regMany([]string{"order.inserted", "order.updated"}, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
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

	// pos_customer / fb_customer / crm_customer: chỉ datachanged → applyDatachangedSideEffects (ingest + refresh), không handler riêng.

	reg(aidecisionsvc.EventTypeConversationIntelligenceRequested, processConversationIntelligenceRequested)
	regMany([]string{"cix_analysis_result.inserted", "cix_analysis_result.updated"}, processCixAnalysisResultDataChanged)
	reg("cix.analysis_completed", processCixAnalysisCompleted)
	reg("customer.context_ready", processCustomerContextReady)
	reg("order.flags_emitted", processOrderFlagsEmitted)
	reg("ads.context_requested", processAdsContextRequested)
	reg("ads.context_ready", processAdsContextReady)
	reg(aidecisionsvc.EventTypeExecutorProposeRequested, processExecutorProposeRequested)
	reg(aidecisionsvc.EventTypeAdsProposeRequested, processExecutorProposeRequested)
	reg(aidecisionsvc.EventTypeExecuteRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return svc.ProcessExecuteRequestedEvent(ctx, evt)
	})
	regMany([]string{"meta_campaign.inserted", "meta_campaign.updated"}, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return svc.ProcessMetaCampaignDataChanged(ctx, evt)
	})
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
