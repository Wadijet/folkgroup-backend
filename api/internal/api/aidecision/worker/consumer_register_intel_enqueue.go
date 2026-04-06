package worker

import (
	"context"

	"meta_commerce/internal/api/aidecision/consumerreg"
	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
)

func init() {
	reg := consumerreg.Register
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
