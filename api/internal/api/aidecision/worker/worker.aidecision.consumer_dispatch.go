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
//
// Đăng ký consumer tách file: consumer_register_*.go; orderintel — blank import aidecisionsubscribe.
package worker

import (
	"context"

	_ "meta_commerce/internal/api/orderintel/aidecisionsubscribe" // đăng ký consumer Order Intelligence

	"meta_commerce/internal/api/aidecision/consumerreg"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
)

// dispatchConsumerEvent tra handler đã đăng ký; không có → no_handler (không lỗi). tr ghi processTrace (lookup → handler).
func dispatchConsumerEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent, tr *queueProcessTracer) (aidecisionmodels.ConsumerCompletionKind, error) {
	if evt == nil {
		return aidecisionmodels.ConsumerCompletionKindProcessed, nil
	}
	if tr == nil {
		tr = newQueueProcessTracer(evt)
	}
	tr.noteRoutingAllowDispatch()
	tr.noteDispatchLookup()
	h, ok := consumerreg.Lookup(evt.EventType)
	if !ok {
		tr.noteNoHandlerRegistered()
		publishQueueNoRegisteredHandler(ownerOrgIDFromDecisionEvent(evt), evt, tr.snapshotTree())
		return aidecisionmodels.ConsumerCompletionKindNoHandler, nil
	}
	tr.noteHandlerInvoke()
	err := h(ctx, svc, evt)
	if err != nil {
		tr.noteHandlerError(err)
	} else {
		tr.noteHandlerSuccess()
	}
	return aidecisionmodels.ConsumerCompletionKindProcessed, err
}
