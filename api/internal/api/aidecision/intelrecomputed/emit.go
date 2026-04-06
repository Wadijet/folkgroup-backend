// Package intelrecomputed — emit *_{domain}_intel_recomputed sau khi worker domain tính Intelligence xong (cùng mẫu campaign_intel_recomputed).
package intelrecomputed

import (
	"context"
	"strings"

	"meta_commerce/internal/api/aidecision/eventemit"
	"meta_commerce/internal/api/aidecision/eventtypes"
	"meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/api/aidecision/queuedepth"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// EventTypeCrmIntelRecomputed — worker crm_intel_compute xong một job (refresh/recalculate/…).
	EventTypeCrmIntelRecomputed = eventtypes.CrmIntelRecomputed
	// EventTypeOrderIntelRecomputed — worker order_intel_compute xong snapshot.
	EventTypeOrderIntelRecomputed = eventtypes.OrderIntelRecomputed
	// EventTypeCixIntelRecomputed — worker cix_intel_compute xong AnalyzeSession.
	EventTypeCixIntelRecomputed = eventtypes.CixIntelRecomputed
)

func refreshOrgDepth(ctx context.Context, ownerOrgID primitive.ObjectID) {
	if !ownerOrgID.IsZero() {
		_ = queuedepth.RefreshOrg(ctx, ownerOrgID)
	}
}

// EmitCrmIntelRecomputed ghi queue sau job CRM intelligence thành công.
func EmitCrmIntelRecomputed(ctx context.Context, ownerOrgID primitive.ObjectID, crmJobIDHex, parentDecisionEventID, operation, unifiedID string) error {
	if ownerOrgID.IsZero() {
		return nil
	}
	orgHex := ownerOrgID.Hex()
	traceID := utility.GenerateUID(utility.UIDPrefixTrace)
	correlationID := utility.GenerateUID(utility.UIDPrefixCorrelation)
	payload := map[string]interface{}{
		"triggerFromIntelCompute": true,
		"crmIntelJobId":           strings.TrimSpace(crmJobIDHex),
		"operation":               strings.TrimSpace(operation),
		"ownerOrgIdHex":           orgHex,
	}
	if parentDecisionEventID != "" {
		payload["parentDecisionEventId"] = parentDecisionEventID
	}
	if uid := strings.TrimSpace(unifiedID); uid != "" {
		payload["unifiedId"] = uid
	}
	entID := strings.TrimSpace(unifiedID)
	if entID == "" {
		entID = strings.TrimSpace(crmJobIDHex)
	}
	_, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:     EventTypeCrmIntelRecomputed,
		EventSource:   eventtypes.EventSourceCrmIntel,
		EntityType:    "crm_customer",
		EntityID:      entID,
		OrgID:         orgHex,
		OwnerOrgID:    ownerOrgID,
		Priority:      "normal",
		Lane:          models.EventLaneNormal,
		TraceID:       traceID,
		CorrelationID: correlationID,
		Payload:       payload,
	})
	if err == nil {
		refreshOrgDepth(ctx, ownerOrgID)
	}
	return err
}

// EmitOrderIntelRecomputed ghi queue sau Order Intelligence worker xong một job (một event duy nhất thay cho flags_emitted / commerce.order_completed).
// extras: flags, layer1–3, orderCompletedTransition, flagsChanged, orderId, totalAfterDiscountVnd, … (merge vào payload).
func EmitOrderIntelRecomputed(ctx context.Context, ownerOrgID primitive.ObjectID, orderIntelJobIDHex, orderUid, customerID, conversationID, parentEventID, parentEventType, traceID, correlationID string, extras map[string]interface{}) error {
	if ownerOrgID.IsZero() {
		return nil
	}
	orgHex := ownerOrgID.Hex()
	tid := strings.TrimSpace(traceID)
	if tid == "" {
		tid = utility.GenerateUID(utility.UIDPrefixTrace)
	}
	cid := strings.TrimSpace(correlationID)
	if cid == "" {
		cid = utility.GenerateUID(utility.UIDPrefixCorrelation)
	}
	payload := map[string]interface{}{
		"triggerFromIntelCompute": true,
		"orderIntelJobId":         strings.TrimSpace(orderIntelJobIDHex),
		"orderUid":                strings.TrimSpace(orderUid),
		"customerId":              strings.TrimSpace(customerID),
		"conversationId":          strings.TrimSpace(conversationID),
		"ownerOrgIdHex":           orgHex,
	}
	if parentEventID != "" {
		payload["parentEventId"] = parentEventID
	}
	if parentEventType != "" {
		payload["parentEventType"] = parentEventType
	}
	for k, v := range extras {
		if strings.TrimSpace(k) == "" {
			continue
		}
		payload[k] = v
	}
	entID := strings.TrimSpace(orderUid)
	if entID == "" {
		entID = strings.TrimSpace(orderIntelJobIDHex)
	}
	_, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:     EventTypeOrderIntelRecomputed,
		EventSource:   eventtypes.EventSourceOrderIntel,
		EntityType:    "order",
		EntityID:      entID,
		OrgID:         orgHex,
		OwnerOrgID:    ownerOrgID,
		Priority:      "high",
		Lane:          models.EventLaneFast,
		TraceID:       tid,
		CorrelationID: cid,
		Payload:       payload,
	})
	if err == nil {
		refreshOrgDepth(ctx, ownerOrgID)
	}
	return err
}

// EmitCixIntelRecomputed ghi queue sau CIX worker phân tích xong một job.
// analysisResultID: _id bản ghi cix_analysis_results vừa Insert — consumer gọi ReceiveCixPayload (thay luồng datachanged cix_analysis_result.*).
func EmitCixIntelRecomputed(ctx context.Context, ownerOrgID primitive.ObjectID, cixJobIDHex, conversationID, customerID, channel, cioEventUid, analysisResultID string) error {
	if ownerOrgID.IsZero() {
		return nil
	}
	orgHex := ownerOrgID.Hex()
	traceID := utility.GenerateUID(utility.UIDPrefixTrace)
	correlationID := utility.GenerateUID(utility.UIDPrefixCorrelation)
	ch := strings.TrimSpace(channel)
	if ch == "" {
		ch = "messenger"
	}
	payload := map[string]interface{}{
		"triggerFromIntelCompute": true,
		"cixIntelJobId":           strings.TrimSpace(cixJobIDHex),
		"conversationId":          strings.TrimSpace(conversationID),
		"customerId":              strings.TrimSpace(customerID),
		"channel":                 ch,
		"ownerOrgIdHex":           orgHex,
	}
	if u := strings.TrimSpace(cioEventUid); u != "" {
		payload["cioEventUid"] = u
	}
	if ar := strings.TrimSpace(analysisResultID); ar != "" {
		payload["analysisResultId"] = ar
	}
	entID := strings.TrimSpace(conversationID)
	if entID == "" {
		entID = strings.TrimSpace(cixJobIDHex)
	}
	_, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:     EventTypeCixIntelRecomputed,
		EventSource:   eventtypes.EventSourceCixIntel,
		EntityType:    "conversation",
		EntityID:      entID,
		OrgID:         orgHex,
		OwnerOrgID:    ownerOrgID,
		Priority:      "high",
		Lane:          models.EventLaneFast,
		TraceID:       traceID,
		CorrelationID: correlationID,
		Payload:       payload,
	})
	if err == nil {
		refreshOrgDepth(ctx, ownerOrgID)
	}
	return err
}
