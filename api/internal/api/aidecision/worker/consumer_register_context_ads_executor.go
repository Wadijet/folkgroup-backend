package worker

import (
	"context"

	"meta_commerce/internal/api/aidecision/consumerreg"
	"meta_commerce/internal/api/aidecision/eventtypes"
	"meta_commerce/internal/api/aidecision/intelrecomputed"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
)

func init() {
	reg := consumerreg.Register
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
	regMany := consumerreg.RegisterMany
	regMany(eventtypes.MetaCampaignPipelineHookEventTypes, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return svc.ProcessMetaCampaignDataChanged(ctx, evt)
	})
	reg(intelrecomputed.EventTypeCrmIntelRecomputed, processIntelRecomputedForAID)
	reg(intelrecomputed.EventTypeCixIntelRecomputed, processCixIntelRecomputed)
}
