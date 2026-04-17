// Package datachanged — Miền Order Intelligence: xếp job tính toán từ event cha AI Decision (đã hydrate).
package datachanged

import (
	"context"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	orderintelsvc "meta_commerce/internal/api/orderintel/service"
)

// EnqueueIntelligenceFromParentEvent — side-effect datachanged orderintel (package orderintel).
func EnqueueIntelligenceFromParentEvent(ctx context.Context, parent *aidecisionmodels.DecisionEvent) error {
	return orderintelsvc.EnqueueOrderIntelligenceFromParent(ctx, parent, crmqueue.EnqueueSourceOrderIntel)
}

// EnqueueIntelligenceFromAIDWorker — flush defer trong worker AID.
func EnqueueIntelligenceFromAIDWorker(ctx context.Context, parent *aidecisionmodels.DecisionEvent) error {
	return orderintelsvc.EnqueueOrderIntelligenceFromParent(ctx, parent, crmqueue.EnqueueSourceAIDecision)
}
