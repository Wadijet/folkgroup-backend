// Package orderintelsvc — Order Intelligence: enqueue domain (không emit order.intelligence_requested vào decision_events_queue).
package orderintelsvc

import (
	"context"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
)

// EventTypeOrderIntelligenceRequested — legacy: event trong queue cũ; consumer chỉ chuyển sang order_intel_compute.
const EventTypeOrderIntelligenceRequested = eventtypes.OrderIntelligenceRequested

// EventTypeOrderRecomputeRequested — on-demand; consumer chỉ enqueue domain (tính tại worker Order Intelligence).
const EventTypeOrderRecomputeRequested = eventtypes.OrderRecomputeRequested

// EmitOrderIntelligenceRequested đưa job vào order_intel_compute sau order.inserted/updated (đã hydrate). Không ghi decision_events_queue.
func EmitOrderIntelligenceRequested(ctx context.Context, _ *aidecisionsvc.AIDecisionService, parent *aidecisionmodels.DecisionEvent) error {
	return EnqueueOrderIntelligenceFromParent(ctx, parent)
}
