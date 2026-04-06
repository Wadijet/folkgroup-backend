// Package aidecisionsubscribe — đăng ký consumer AI Decision cho miền orderintel (blank import từ worker).
package aidecisionsubscribe

import (
	"context"

	"meta_commerce/internal/api/aidecision/consumerreg"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	orderintelsvc "meta_commerce/internal/api/orderintel/service"
)

func init() {
	reg := consumerreg.Register
	reg(orderintelsvc.EventTypeOrderIntelligenceRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return orderintelsvc.EnqueueFromLegacyIntelligenceRequestedDecisionEvent(ctx, evt)
	})
	reg(orderintelsvc.EventTypeOrderRecomputeRequested, func(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
		_ = svc
		return orderintelsvc.EnqueueFromRecomputeDecisionEvent(ctx, evt)
	})
}
