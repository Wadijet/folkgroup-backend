// Package aidecisionsvc — Gắn kết CRM intelligence (sau worker crm_intel_compute) với decision_cases_runtime.
package aidecisionsvc

import (
	"context"
	"strings"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	crmmodels "meta_commerce/internal/api/crm/models"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// RefreshOpenCasesAfterCrmIntel đọc crm_customer theo unifiedId, cập nhật contextPackets.customer cho case đang mở khớp source customerId, rồi TryExecuteIfReady (conversation_response).
func (s *AIDecisionService) RefreshOpenCasesAfterCrmIntel(ctx context.Context, unifiedID, orgID string, ownerOrgID primitive.ObjectID) error {
	unifiedID = strings.TrimSpace(unifiedID)
	orgID = strings.TrimSpace(orgID)
	if unifiedID == "" || orgID == "" || ownerOrgID.IsZero() {
		return nil
	}
	collCRM, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CrmCustomers)
	if !ok {
		return nil
	}
	var cust crmmodels.CrmCustomer
	err := collCRM.FindOne(ctx, bson.M{"unifiedId": unifiedID, "ownerOrganizationId": ownerOrgID}).Decode(&cust)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		return err
	}
	if strings.TrimSpace(cust.UnifiedId) == "" {
		return nil
	}
	ids := crmvc.CollectSourceCustomerIds(&cust)
	if len(ids) == 0 {
		return nil
	}

	customerPayload := crmCustomerIntelPayloadFromModel(&cust)

	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return nil
	}
	nowMs := time.Now().UnixMilli()
	filter := bson.M{
		"orgId":                 orgID,
		"ownerOrganizationId":   ownerOrgID,
		"status":                bson.M{"$nin": []string{aidecisionmodels.CaseStatusClosed, "cancelled", "expired", "dropped"}},
		"entityRefs.customerId": bson.M{"$in": ids},
	}
	_, err = coll.UpdateMany(ctx, filter, bson.M{
		"$set": bson.M{
			"contextPackets.customer": customerPayload,
			"updatedAt":               nowMs,
		},
		"$addToSet": bson.M{"receivedContexts": "customer"},
	})
	if err != nil {
		return err
	}

	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var c aidecisionmodels.DecisionCase
		if err := cursor.Decode(&c); err != nil {
			continue
		}
		if c.CaseType != aidecisionmodels.CaseTypeConversationResponse {
			continue
		}
		convID := strings.TrimSpace(c.EntityRefs.ConversationID)
		custID := strings.TrimSpace(c.EntityRefs.CustomerID)
		if convID == "" || custID == "" {
			continue
		}
		_ = s.TryExecuteIfReady(ctx, convID, custID, orgID, ownerOrgID)
	}
	return cursor.Err()
}

func crmCustomerIntelPayloadFromModel(c *crmmodels.CrmCustomer) map[string]interface{} {
	if c == nil {
		return nil
	}
	m := map[string]interface{}{
		"unifiedId":           strings.TrimSpace(c.UnifiedId),
		"source":              "crm_intel_recomputed",
		"totalSpent":          c.TotalSpent,
		"orderCount":          c.OrderCount,
		"valueTier":           c.ValueTier,
		"lifecycleStage":      c.LifecycleStage,
		"journeyStage":        c.JourneyStage,
		"channel":             c.Channel,
		"loyaltyStage":        c.LoyaltyStage,
		"momentumStage":       c.MomentumStage,
		"lastOrderAt":         c.LastOrderAt,
		"lastConversationAt":  c.LastConversationAt,
		"hasConversation":     c.HasConversation,
		"hasOrder":            c.HasOrder,
		"conversationTags":    c.ConversationTags,
		"orderCountOnline":    c.OrderCountOnline,
		"orderCountOffline":   c.OrderCountOffline,
	}
	if c.CurrentMetrics != nil {
		m["currentMetrics"] = c.CurrentMetrics
	}
	return m
}
