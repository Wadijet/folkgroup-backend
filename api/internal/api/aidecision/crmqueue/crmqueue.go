// Package crmqueue — emit event CRM Intelligence vào decision_events_queue.
// Tách khỏi aidecisionsvc để internal/worker gọi được mà không vướng import cycle (delivery → worker).
package crmqueue

import (
	"context"
	"strings"

	"meta_commerce/internal/api/aidecision/eventemit"
	"meta_commerce/internal/api/aidecision/eventtypes"
	"meta_commerce/internal/api/aidecision/queuedepth"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EventTypeCrmIntelligenceComputeRequested — consumer chỉ enqueue crm_intel_compute; worker domain CRM thực thi (payload.operation).
const EventTypeCrmIntelligenceComputeRequested = eventtypes.CrmIntelligenceComputeRequested

// EventTypeCrmIntelligenceRecomputeRequested — yêu cầu tính lại CRM intelligence (cùng dạng tên với ads.intelligence.recompute_requested).
// Sau ingest worker: consumer AID debounce theo unifiedId rồi xếp crm_intel_compute (refresh).
const EventTypeCrmIntelligenceRecomputeRequested = eventtypes.CrmIntelligenceRecomputeRequested

// CrmComputeOperation — giá trị payload "operation".
const (
	CrmComputeOpRefresh                       = "refresh"
	CrmComputeOpRecalculateOne                = "recalculate_one"
	CrmComputeOpRecalculateAll                = "recalculate_all"
	CrmComputeOpRecalculateBatch              = "recalculate_batch"
	CrmComputeOpRecalculateMismatch           = "recalculate_mismatch"
	CrmComputeOpRecalculateOrderCountMismatch = "recalculate_order_count_mismatch"
	CrmComputeOpRecalculateAllOrgs            = "recalculate_all_orgs"
	CrmComputeOpClassificationRefresh         = "classification_refresh"
)

func emitCrmIntelligenceCompute(ctx context.Context, operation string, ownerOrgID primitive.ObjectID, payload map[string]interface{}, lane string) (eventID string, err error) {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	payload["operation"] = operation
	orgIDStr := "system"
	if !ownerOrgID.IsZero() {
		orgIDStr = ownerOrgID.Hex()
		payload["ownerOrgIdHex"] = ownerOrgID.Hex()
	}
	if lane == "" {
		lane = aidecisionmodels.EventLaneNormal
	}
	entID := ""
	if u, ok := payload["unifiedId"].(string); ok {
		entID = u
	}
	res, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:   EventTypeCrmIntelligenceComputeRequested,
		EventSource: eventtypes.EventSourceCRM,
		EntityType:  "crm_customer",
		EntityID:    entID,
		OrgID:       orgIDStr,
		OwnerOrgID:  ownerOrgID,
		Priority:    "normal",
		Lane:        lane,
		Payload:     payload,
	})
	if err != nil {
		return "", err
	}
	if !ownerOrgID.IsZero() {
		_ = queuedepth.RefreshOrg(ctx, ownerOrgID)
	}
	return res.EventID, nil
}

// EmitCrmIntelligenceRefreshRequested — sau ingest order/conversation (thay RefreshMetrics trực tiếp).
func EmitCrmIntelligenceRefreshRequested(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID) (string, error) {
	return emitCrmIntelligenceCompute(ctx, CrmComputeOpRefresh, ownerOrgID, map[string]interface{}{
		"unifiedId": unifiedId,
	}, aidecisionmodels.EventLaneFast)
}

// EmitCrmIntelligenceRecomputeRequested — sau CrmPendingMergeWorker merge L1→L2 → queue AID (debounce → crm_intel_compute).
func EmitCrmIntelligenceRecomputeRequested(ctx context.Context, unifiedID string, ownerOrgID primitive.ObjectID, sourceCollection, pendingMergeJobHex string) (string, error) {
	unifiedID = strings.TrimSpace(unifiedID)
	if unifiedID == "" || ownerOrgID.IsZero() {
		return "", nil
	}
	payload := map[string]interface{}{
		"unifiedId":     unifiedID,
		"ownerOrgIdHex": ownerOrgID.Hex(),
	}
	if strings.TrimSpace(sourceCollection) != "" {
		payload["sourceCollection"] = strings.TrimSpace(sourceCollection)
	}
	if strings.TrimSpace(pendingMergeJobHex) != "" {
		payload["pendingMergeJobId"] = strings.TrimSpace(pendingMergeJobHex)
	}
	res, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:   EventTypeCrmIntelligenceRecomputeRequested,
		EventSource: eventtypes.EventSourceCrmMergeQueue,
		EntityType:  "crm_customer",
		EntityID:    unifiedID,
		OrgID:       ownerOrgID.Hex(),
		OwnerOrgID:  ownerOrgID,
		Priority:    "normal",
		Lane:        aidecisionmodels.EventLaneNormal,
		Payload:     payload,
	})
	if err != nil {
		return "", err
	}
	_ = queuedepth.RefreshOrg(ctx, ownerOrgID)
	return res.EventID, nil
}

// EmitCrmIntelligenceRecalculateOneRequested — thay RecalculateCustomerFromAllSources gọi trực tiếp.
func EmitCrmIntelligenceRecalculateOneRequested(ctx context.Context, unifiedId string, ownerOrgID primitive.ObjectID) (string, error) {
	return emitCrmIntelligenceCompute(ctx, CrmComputeOpRecalculateOne, ownerOrgID, map[string]interface{}{
		"unifiedId": unifiedId,
	}, aidecisionmodels.EventLaneNormal)
}

// EmitCrmIntelligenceRecalculateAllRequested — một org, limit + poolSize.
func EmitCrmIntelligenceRecalculateAllRequested(ctx context.Context, ownerOrgID primitive.ObjectID, limit, poolSize int) (string, error) {
	return emitCrmIntelligenceCompute(ctx, CrmComputeOpRecalculateAll, ownerOrgID, map[string]interface{}{
		"limit":    limit,
		"poolSize": poolSize,
	}, aidecisionmodels.EventLaneBatch)
}

// EmitCrmIntelligenceRecalculateBatchRequested — batch offset/limit.
func EmitCrmIntelligenceRecalculateBatchRequested(ctx context.Context, ownerOrgID primitive.ObjectID, offset, limit, poolSize int) (string, error) {
	return emitCrmIntelligenceCompute(ctx, CrmComputeOpRecalculateBatch, ownerOrgID, map[string]interface{}{
		"offset":   offset,
		"limit":    limit,
		"poolSize": poolSize,
	}, aidecisionmodels.EventLaneBatch)
}

// EmitCrmIntelligenceRecalculateMismatchRequested — engaged/visitor mismatch.
func EmitCrmIntelligenceRecalculateMismatchRequested(ctx context.Context, ownerOrgID primitive.ObjectID, limit, poolSize int) (string, error) {
	return emitCrmIntelligenceCompute(ctx, CrmComputeOpRecalculateMismatch, ownerOrgID, map[string]interface{}{
		"limit":    limit,
		"poolSize": poolSize,
	}, aidecisionmodels.EventLaneBatch)
}

// EmitCrmIntelligenceRecalculateOrderCountMismatchRequested — first/repeat/vip mismatch.
func EmitCrmIntelligenceRecalculateOrderCountMismatchRequested(ctx context.Context, ownerOrgID primitive.ObjectID, limit, poolSize int) (string, error) {
	return emitCrmIntelligenceCompute(ctx, CrmComputeOpRecalculateOrderCountMismatch, ownerOrgID, map[string]interface{}{
		"limit":    limit,
		"poolSize": poolSize,
	}, aidecisionmodels.EventLaneBatch)
}

// EmitCrmIntelligenceRecalculateAllOrgsRequested — startup / bảo trì toàn org.
func EmitCrmIntelligenceRecalculateAllOrgsRequested(ctx context.Context, poolSize int) (string, error) {
	return emitCrmIntelligenceCompute(ctx, CrmComputeOpRecalculateAllOrgs, primitive.NilObjectID, map[string]interface{}{
		"poolSize": poolSize,
	}, aidecisionmodels.EventLaneBatch)
}

// EmitCrmIntelligenceClassificationRefreshRequested — định kỳ phân loại (consumer gọi RunClassificationRefreshBatch).
func EmitCrmIntelligenceClassificationRefreshRequested(ctx context.Context, mode string, batchSize int) (string, error) {
	return emitCrmIntelligenceCompute(ctx, CrmComputeOpClassificationRefresh, primitive.NilObjectID, map[string]interface{}{
		"mode":      mode,
		"batchSize": batchSize,
	}, aidecisionmodels.EventLaneBatch)
}
