// Package crmvc - Logic phân loại khách 2 lớp theo CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN.
//
// Kiến trúc 2 lớp:
//
//	LỚP 1 — CUSTOMER JOURNEY (hành trình trưởng thành, thuần):
//	  VISITOR | ENGAGED | BLOCKED_SPAM | FIRST | REPEAT | PROMOTER
//	  (VIP dùng valueTier Lớp 2; inactive dùng lifecycleStage Lớp 2; promoter chờ dữ liệu referral)
//
//	LỚP 2 — CUSTOMER SEGMENTATION (5 trục phân loại):
//	  CHANNEL | VALUE | LIFECYCLE | LOYALTY | MOMENTUM
//
// Profile đầy đủ: Journey | Channel | Value | Lifecycle | Loyalty | Momentum
package crmvc

import (
	"context"

	crmmodels "meta_commerce/internal/api/crm/models"
	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	ruleintelsvc "meta_commerce/internal/api/ruleintel/service"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetClassificationFromCustomer gọi Rule Engine RULE_CRM_CLASSIFICATION — vỏ duy nhất khi có customer.
// Khi c.UnifiedId rỗng hoặc c.OwnerOrganizationID zero → trả map rỗng (không gọi Rule Engine).
// Dùng trong toProfileResponse, buildMetricsSnapshot.
func GetClassificationFromCustomer(ctx context.Context, c *crmmodels.CrmCustomer) map[string]interface{} {
	if c == nil || c.UnifiedId == "" || c.OwnerOrganizationID.IsZero() {
		return map[string]interface{}{}
	}
	raw := map[string]interface{}{
		"totalSpent":        GetTotalSpentFromCustomer(c),
		"orderCount":        GetOrderCountFromCustomer(c),
		"lastOrderAt":       GetLastOrderAtFromCustomer(c),
		"revenueLast30d":    GetFloatFromCustomer(c, "revenueLast30d"),
		"revenueLast90d":    GetFloatFromCustomer(c, "revenueLast90d"),
		"orderCountOnline":  GetIntFromCustomer(c, "orderCountOnline"),
		"orderCountOffline": GetIntFromCustomer(c, "orderCountOffline"),
		"hasConversation":   GetBoolFromCustomer(c, "hasConversation"),
		"conversationTags":  c.ConversationTags,
	}
	if class := computeClassificationViaRuleEngine(ctx, raw, c.UnifiedId, c.OwnerOrganizationID); class != nil {
		return class
	}
	return map[string]interface{}{}
}

// getStrFromMap trích string từ map — dùng cho classification map.
func getStrFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// computeClassificationViaRuleEngine gọi Rule Engine RULE_CRM_CLASSIFICATION.
// Trả về map classification hoặc nil khi lỗi (caller dùng fallback ComputeClassificationFromMetrics).
func computeClassificationViaRuleEngine(ctx context.Context, raw map[string]interface{}, unifiedId string, ownerOrgID primitive.ObjectID) map[string]interface{} {
	svc, err := ruleintelsvc.NewRuleEngineService()
	if err != nil {
		return nil
	}
	if raw == nil {
		raw = map[string]interface{}{}
	}
	input := &ruleintelsvc.RunInput{
		RuleID:    "RULE_CRM_CLASSIFICATION",
		Domain:    "crm",
		EntityRef: ruleintelmodels.EntityRef{Domain: "crm", ObjectType: "customer", ObjectID: unifiedId, OwnerOrganizationID: ownerOrgID.Hex()},
		Layers:    map[string]interface{}{"raw": raw},
	}
	result, err := svc.Run(ctx, input)
	if err != nil || result == nil || result.Result == nil {
		return nil
	}
	m, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil
	}
	// Đảm bảo có đủ 6 field để $set vào crm_customers
	out := map[string]interface{}{
		"valueTier":      m["valueTier"],
		"lifecycleStage": m["lifecycleStage"],
		"journeyStage":   m["journeyStage"],
		"channel":        m["channel"],
		"loyaltyStage":   m["loyaltyStage"],
		"momentumStage":  m["momentumStage"],
	}
	return out
}

// ComputeClassificationFromMetricsOrRuleEngine gọi Rule Engine RULE_CRM_CLASSIFICATION.
// Không fallback — khi Rule Engine trả nil thì trả về map rỗng (không cập nhật classification).
// Dùng trong Recalculate, Merge, RefreshMetrics khi có ctx và unifiedId.
func ComputeClassificationFromMetricsOrRuleEngine(ctx context.Context, totalSpent float64, orderCount int, lastOrderAt int64, revenueLast30d, revenueLast90d float64, orderCountOnline, orderCountOffline int, hasConversation bool, conversationTags []string, unifiedId string, ownerOrgID primitive.ObjectID) map[string]interface{} {
	raw := map[string]interface{}{
		"totalSpent":        totalSpent,
		"orderCount":        orderCount,
		"lastOrderAt":       lastOrderAt,
		"revenueLast30d":    revenueLast30d,
		"revenueLast90d":    revenueLast90d,
		"orderCountOnline":  orderCountOnline,
		"orderCountOffline": orderCountOffline,
		"hasConversation":   hasConversation,
		"conversationTags":  conversationTags,
	}
	if class := computeClassificationViaRuleEngine(ctx, raw, unifiedId, ownerOrgID); class != nil {
		return class
	}
	return map[string]interface{}{}
}
