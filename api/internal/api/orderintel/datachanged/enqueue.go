// Package datachanged — Miền Order Intelligence: xếp job tính toán từ event cha AI Decision (đã hydrate).
package datachanged

import (
	"context"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	orderintelsvc "meta_commerce/internal/api/orderintel/service"
)

// EnqueueIntelligenceFromParentEvent giao việc order_intel_compute cho domain Order Intelligence.
func EnqueueIntelligenceFromParentEvent(ctx context.Context, parent *aidecisionmodels.DecisionEvent) error {
	return orderintelsvc.EnqueueOrderIntelligenceFromParent(ctx, parent)
}
