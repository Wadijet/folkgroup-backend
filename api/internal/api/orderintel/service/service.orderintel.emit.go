// Package orderintelsvc — Order Intelligence: enqueue domain (không emit order.intelligence_requested vào decision_events_queue).
package orderintelsvc

import (
	"context"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
)

// EventTypeOrderIntelligenceRequested — legacy: event trong queue cũ; consumer chỉ chuyển sang order_intelligence_pending.
const EventTypeOrderIntelligenceRequested = "order.intelligence_requested"

// EventTypeOrderRecomputeRequested — on-demand; consumer chỉ enqueue domain (tính tại worker Order Intelligence).
const EventTypeOrderRecomputeRequested = "order.recompute_requested"

// EmitOrderIntelligenceRequested đưa job vào order_intelligence_pending sau order.inserted/updated (đã hydrate). Không ghi decision_events_queue.
func EmitOrderIntelligenceRequested(ctx context.Context, _ *aidecisionsvc.AIDecisionService, parent *aidecisionmodels.DecisionEvent) error {
	return EnqueueOrderIntelligenceFromParent(ctx, parent)
}
