// Package worker — Điều phối Decision Case sau source event (PLATFORM_L1 §3.2, §15).
// Tách khỏi switch consumer để mở rộng routing/rule sau này.
package worker

import (
	"context"
	"fmt"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	orderintelsvc "meta_commerce/internal/api/orderintel/service"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrchestrateConversationSourceEvent — ResolveOrCreate case hội thoại + emit work request domain.
// Queue crm_pending_merge đã được xếp trong applyDatachangedSideEffects (trước dispatch); intel CRM sau CrmPendingMergeWorker + crm.intelligence.recompute_requested.
func OrchestrateConversationSourceEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent, skipHydrate bool) error {
	if !skipHydrate {
		svc.HydrateDatachangedPayload(ctx, evt)
	}
	convID := payloadStr(evt.Payload, "conversationId")
	custID := payloadStr(evt.Payload, "customerId")
	channel := "messenger"
	if ch, ok := evt.Payload["channel"].(string); ok && ch != "" {
		channel = ch
	}
	normalizedRecordUid := evt.EntityID
	if u, ok := evt.Payload["normalizedRecordUid"].(string); ok && strings.TrimSpace(u) != "" {
		normalizedRecordUid = u
	}

	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}

	caseDoc, createdNew, err := svc.ResolveOrCreate(ctx, &aidecisionsvc.ResolveOrCreateInput{
		EventID:    evt.EventID,
		EventType:  evt.EventType,
		OrgID:      evt.OrgID,
		OwnerOrgID: ownerOrgID,
		EntityRefs: aidecisionmodels.DecisionCaseEntityRefs{
			ConversationID: convID,
			CustomerID:     custID,
		},
		CaseType:    aidecisionmodels.CaseTypeConversationResponse,
		RequiredCtx: svc.RequiredContextsForCaseTypeFromRule(ctx, ownerOrgID, aidecisionmodels.CaseTypeConversationResponse),
		Priority:      evt.Priority,
		Urgency:       "realtime",
		TraceID:       evt.TraceID,
		CorrelationID: evt.CorrelationID,
	})
	if err != nil {
		return err
	}

	emittedCustomer := false
	if custID != "" {
		_, _ = svc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
			EventType:     eventtypes.CustomerContextRequested,
			EventSource:   aidecisionsvc.EventSourceAIDecision,
			EntityType:    "customer",
			EntityID:      custID,
			OrgID:         evt.OrgID,
			OwnerOrgID:    ownerOrgID,
			Priority:      "high",
			Lane:          aidecisionmodels.EventLaneFast,
			TraceID:       evt.TraceID,
			CorrelationID: evt.CorrelationID,
			Payload: map[string]interface{}{
				"conversationId": convID,
				"customerId":     custID,
				"channel":        channel,
			},
		})
		emittedCustomer = true
	}

	emittedCix := false
	if convID == "" {
		publishOrchestrateConversation(ownerOrgID, evt, caseDoc, createdNew, convID, custID, channel, normalizedRecordUid, emittedCustomer, emittedCix)
		return nil
	}
	_, err = svc.EmitEvent(ctx, &aidecisionsvc.EmitEventInput{
		EventType:     eventtypes.CixAnalysisRequested,
		EventSource:   aidecisionsvc.EventSourceAIDecision,
		EntityType:    "conversation",
		EntityID:      convID,
		OrgID:         evt.OrgID,
		OwnerOrgID:    ownerOrgID,
		Priority:      "high",
		Lane:          aidecisionmodels.EventLaneFast,
		TraceID:       evt.TraceID,
		CorrelationID: evt.CorrelationID,
		Payload: map[string]interface{}{
			"conversationId":      convID,
			"customerId":          custID,
			"channel":             channel,
			"normalizedRecordUid": normalizedRecordUid,
		},
	})
	emittedCix = true
	publishOrchestrateConversation(ownerOrgID, evt, caseDoc, createdNew, convID, custID, channel, normalizedRecordUid, emittedCustomer, emittedCix)
	return err
}

// OrchestrateOrderSourceEvent — ResolveOrCreate case order_risk + enqueue Order Intelligence (domain worker).
// crm_pending_merge đã xếp trong applyDatachangedSideEffects.
func OrchestrateOrderSourceEvent(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	svc.HydrateDatachangedPayload(ctx, evt)

	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}

	orderUid := strings.TrimSpace(payloadStr(evt.Payload, "orderUid"))
	if orderUid == "" {
		orderUid = strings.TrimSpace(payloadStr(evt.Payload, "uid"))
	}
	if orderUid == "" {
		orderUid = strings.TrimSpace(payloadStr(evt.Payload, "normalizedRecordUid"))
	}
	if orderUid == "" {
		orderUid = strings.TrimSpace(evt.EntityID)
	}
	custID := payloadStr(evt.Payload, "customerId")
	convID := payloadStr(evt.Payload, "conversationId")

	var caseDoc *aidecisionmodels.DecisionCase
	var createdNew bool
	if orderUid != "" {
		var errResolve error
		caseDoc, createdNew, errResolve = svc.ResolveOrCreate(ctx, &aidecisionsvc.ResolveOrCreateInput{
			EventID:    evt.EventID,
			EventType:  evt.EventType,
			OrgID:      evt.OrgID,
			OwnerOrgID: ownerOrgID,
			EntityRefs: aidecisionmodels.DecisionCaseEntityRefs{
				OrderID:        orderUid,
				CustomerID:     custID,
				ConversationID: convID,
			},
			CaseType: aidecisionmodels.CaseTypeOrderRisk,
			RequiredCtx: svc.RequiredContextsForCaseTypeFromRule(ctx, ownerOrgID, aidecisionmodels.CaseTypeOrderRisk),
			Priority:      evt.Priority,
			Urgency:       "near_realtime",
			TraceID:       evt.TraceID,
			CorrelationID: evt.CorrelationID,
		})
		if errResolve != nil {
			return fmt.Errorf("ResolveOrCreate order_risk: %w", errResolve)
		}
	}

	err := orderintelsvc.EnqueueOrderIntelligenceFromParent(ctx, evt)
	publishOrchestrateOrder(ownerOrgID, evt, caseDoc, createdNew, orderUid, custID, convID, err == nil)
	return err
}

// publishOrchestrateConversation — Publish timeline sau khi điều phối hội thoại (neo case + việc xếp hàng tiếp theo).
func publishOrchestrateConversation(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent, caseDoc *aidecisionmodels.DecisionCase, createdNew bool, convID, custID, channel, normalizedRecordUid string, emittedCustomer, emittedCix bool) {
	tid := strings.TrimSpace(evt.TraceID)
	if tid == "" || ownerOrgID.IsZero() {
		return
	}
	evLive := livecopy.BuildOrchestrateConversationEvent(evt, caseDoc, createdNew, convID, custID, channel, normalizedRecordUid, emittedCustomer, emittedCix)
	decisionlive.Publish(ownerOrgID, tid, evLive)
}

// publishOrchestrateOrder — Publish timeline sau khi điều phối đơn (hồ sơ rủi ro đơn + xếp hàng phân tích đơn).
func publishOrchestrateOrder(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent, caseDoc *aidecisionmodels.DecisionCase, createdNew bool, orderUid, custID, convID string, enqueuedOrderIntelOK bool) {
	tid := strings.TrimSpace(evt.TraceID)
	if tid == "" || ownerOrgID.IsZero() {
		return
	}
	evLive := livecopy.BuildOrchestrateOrderEvent(evt, caseDoc, createdNew, orderUid, custID, convID, enqueuedOrderIntelOK)
	decisionlive.Publish(ownerOrgID, tid, evLive)
}

func payloadStr(p map[string]interface{}, key string) string {
	if p == nil {
		return ""
	}
	v, ok := p[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", t))
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		return ""
	}
}
