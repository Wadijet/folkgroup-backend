// Package crmqueue — emit event CRM Intelligence vào decision_events_queue.
// Tách khỏi aidecisionsvc để internal/worker gọi được mà không vướng import cycle (delivery → worker).
package crmqueue

import (
	"context"
	"strings"
	"time"

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

// PayloadKeyCausalOrderingAtMs — unix ms của thay đổi nguồn (L1 / merge); copy vào job crm_intel_compute để sort lịch sử intel đúng thứ tự nghiệp vụ khi sự kiện không FIFO.
const PayloadKeyCausalOrderingAtMs = "causalOrderingAtMs"

// ExtractCausalOrderingAtMs đọc causalOrderingAtMs từ payload event/job (JSON/BSON có thể là float64).
func ExtractCausalOrderingAtMs(m map[string]interface{}) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[PayloadKeyCausalOrderingAtMs]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case float64:
		return int64(t)
	default:
		return 0
	}
}

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
	// Mốc nghiệp vụ cho lịch sử intel: nếu caller chưa set (merge/defer đã set riêng), dùng thời điểm emit (API/bulk/refresh).
	if ExtractCausalOrderingAtMs(payload) <= 0 {
		payload[PayloadKeyCausalOrderingAtMs] = time.Now().UnixMilli()
	}
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
		EventType:       EventTypeCrmIntelligenceComputeRequested,
		EventSource:     eventtypes.EventSourceCRM,
		PipelineStage:   eventtypes.PipelineStageAfterL1Change,
		EntityType:      "crm_customer",
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

// IsPostL2MergeCrmIntelEnvelope — sau merge L2: cần debounce → crm_intel_compute; không đi consumerreg.Lookup theo .changed (tránh nhầm orchestrate L1).
// Wire: eventSource l2_datachanged + pipeline after_l2_merge + unifiedId; eventType = <prefix>.changed hoặc (tạm) crm.intelligence.recompute_requested.
func IsPostL2MergeCrmIntelEnvelope(evt *aidecisionmodels.DecisionEvent) bool {
	if evt == nil || evt.Payload == nil {
		return false
	}
	if !eventtypes.IsL2DatachangedEventSource(evt.EventSource) {
		return false
	}
	if strings.TrimSpace(evt.PipelineStage) != eventtypes.PipelineStageAfterL2Merge {
		return false
	}
	uid, ok := evt.Payload["unifiedId"].(string)
	if !ok || strings.TrimSpace(uid) == "" {
		return false
	}
	et := strings.TrimSpace(evt.EventType)
	if et == EventTypeCrmIntelligenceRecomputeRequested {
		return true
	}
	dot := strings.LastIndexByte(et, '.')
	return dot > 0 && strings.HasSuffix(et, ".changed")
}

// EmitAfterL2MergeForCrmIntel — sau CrmPendingMergeWorker merge L1→L2: wire đồng bộ catalog G2-S05 (l2_datachanged, after_l2_merge); eventType thường <prefix>.changed từ collection nguồn.
func EmitAfterL2MergeForCrmIntel(ctx context.Context, eventType string, unifiedID string, ownerOrgID primitive.ObjectID, sourceCollection, pendingMergeJobHex string, causalOrderingAtMs int64, traceID, correlationID string) (string, error) {
	unifiedID = strings.TrimSpace(unifiedID)
	if unifiedID == "" || ownerOrgID.IsZero() {
		return "", nil
	}
	et := strings.TrimSpace(eventType)
	if et == "" {
		et = EventTypeCrmIntelligenceRecomputeRequested
	}
	payload := map[string]interface{}{
		"unifiedId":     unifiedID,
		"ownerOrgIdHex": ownerOrgID.Hex(),
	}
	if causalOrderingAtMs > 0 {
		payload[PayloadKeyCausalOrderingAtMs] = causalOrderingAtMs
	}
	if strings.TrimSpace(sourceCollection) != "" {
		payload["sourceCollection"] = strings.TrimSpace(sourceCollection)
	}
	if strings.TrimSpace(pendingMergeJobHex) != "" {
		payload["pendingMergeJobId"] = strings.TrimSpace(pendingMergeJobHex)
	}
	res, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:       et,
		EventSource:     eventtypes.EventSourceL2Datachanged,
		PipelineStage:   eventtypes.PipelineStageAfterL2Merge,
		EntityType:      "crm_customer",
		EntityID:        unifiedID,
		OrgID:           ownerOrgID.Hex(),
		OwnerOrgID:      ownerOrgID,
		Priority:        "normal",
		Lane:            aidecisionmodels.EventLaneNormal,
		TraceID:         strings.TrimSpace(traceID),
		CorrelationID:   strings.TrimSpace(correlationID),
		Payload:         payload,
	})
	if err != nil {
		return "", err
	}
	_ = queuedepth.RefreshOrg(ctx, ownerOrgID)
	return res.EventID, nil
}

// EmitCrmIntelligenceRecomputeRequested — enqueue recompute CRM intel với eventSource crm_merge_queue (luồng defer/legacy, không phải wire L2-datachanged mới sau merge).
// causalOrderingAtMs: thời điểm nghiệp vụ (thường updatedAt nguồn L1 ms); 0 = không gửi (worker dùng mặc định khi persist).
// traceID / correlationID — nối timeline với luồng datachanged / merge queue (có thể rỗng).
func EmitCrmIntelligenceRecomputeRequested(ctx context.Context, unifiedID string, ownerOrgID primitive.ObjectID, sourceCollection, pendingMergeJobHex string, causalOrderingAtMs int64, traceID, correlationID string) (string, error) {
	unifiedID = strings.TrimSpace(unifiedID)
	if unifiedID == "" || ownerOrgID.IsZero() {
		return "", nil
	}
	payload := map[string]interface{}{
		"unifiedId":     unifiedID,
		"ownerOrgIdHex": ownerOrgID.Hex(),
	}
	if causalOrderingAtMs > 0 {
		payload[PayloadKeyCausalOrderingAtMs] = causalOrderingAtMs
	}
	if strings.TrimSpace(sourceCollection) != "" {
		payload["sourceCollection"] = strings.TrimSpace(sourceCollection)
	}
	if strings.TrimSpace(pendingMergeJobHex) != "" {
		payload["pendingMergeJobId"] = strings.TrimSpace(pendingMergeJobHex)
	}
	res, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:       EventTypeCrmIntelligenceRecomputeRequested,
		EventSource:     eventtypes.EventSourceCrmMergeQueue,
		PipelineStage:   eventtypes.PipelineStageAfterL2Merge,
		EntityType:      "crm_customer",
		EntityID:    unifiedID,
		OrgID:       ownerOrgID.Hex(),
		OwnerOrgID:  ownerOrgID,
		Priority:    "normal",
		Lane:        aidecisionmodels.EventLaneNormal,
		TraceID:       strings.TrimSpace(traceID),
		CorrelationID: strings.TrimSpace(correlationID),
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
