package worker

import (
	"context"

	"meta_commerce/internal/api/aidecision/consumerreg"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
)

func init() {
	regMany := consumerreg.RegisterMany
	regMany(eventtypes.MessageFastPathEventTypes, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return processSourceEvent(ctx, svc, evt, false)
	})
	regMany(eventtypes.ConversationLifecycleEventTypes, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return processSyncedSourceWithDebounce(ctx, svc, evt)
	})
	regMany(eventtypes.OrderLifecycleEventTypes, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		return processOrderEvent(ctx, svc, evt)
	})
}
