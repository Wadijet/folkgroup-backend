// Package aidecisionsvc — Emit cix.analysis_requested (queue AI Decision).
//
// Quy ước EventSource (chuỗi xin/trả context): ghi đúng **module hoặc lớp phát event vào queue**.
//   - aidecision — orchestrate/consumer AI Decision (xin CRM/CIX/Ads context nội bộ).
//   - cix_api — HTTP handler CIX đưa yêu cầu phân tích vào queue.
//   - crm / orderintel / meta_ads_intel — worker hoặc svc domain sau khi có payload (context_ready, flags, campaign_intel_recomputed, …).
//   - datachanged / debounce — hook Mongo hoặc flush gom message.
package aidecisionsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EventTypeCixAnalysisRequested — AI Decision consumer xử lý → EnqueueAnalysis → cix_intel_compute (cùng mẫu CRM enqueue).
const EventTypeCixAnalysisRequested = eventtypes.CixAnalysisRequested

// EventSourceCixHTTP — event do API HTTP CIX phát vào queue (handler gọi EmitCixAnalysisRequested).
const EventSourceCixHTTP = eventtypes.EventSourceCixHTTP

// EventSourceAIDecision — event do lớp AI Decision (orchestrate / consumer nội bộ) phát: xin context, bước pipeline.
const EventSourceAIDecision = eventtypes.EventSourceAIDecision

// EmitCixAnalysisRequested đưa yêu cầu phân tích CIX vào decision_events_queue.
// eventSource: EventSourceCixHTTP khi gọi từ handler CIX; EventSourceAIDecision khi consumer bridge/orchestrate phát (không nhầm với HTTP).
// Luồng: consumer → EnqueueAnalysis → WorkerCixIntelCompute (cix_intel_compute) → AnalyzeSession (không gọi AnalyzeSession từ HTTP).
func EmitCixAnalysisRequested(ctx context.Context, conversationID, customerID, channel string, ownerOrgID primitive.ObjectID, normalizedRecordUID, traceID, correlationID, eventSource string) (eventID string, err error) {
	if strings.TrimSpace(conversationID) == "" {
		return "", fmt.Errorf("conversationId bắt buộc")
	}
	ch := strings.TrimSpace(channel)
	if ch == "" {
		ch = "messenger"
	}
	svc := NewAIDecisionService()
	payload := map[string]interface{}{
		"conversationId": conversationID,
		"customerId":     customerID,
		"channel":        ch,
	}
	if strings.TrimSpace(normalizedRecordUID) != "" {
		payload["normalizedRecordUid"] = normalizedRecordUID
	}
	if crmqueue.ExtractCausalOrderingAtMs(payload) <= 0 {
		payload[crmqueue.PayloadKeyCausalOrderingAtMs] = time.Now().UnixMilli()
	}
	src := strings.TrimSpace(eventSource)
	if src == "" {
		src = EventSourceAIDecision
	}
	stage := eventtypes.PipelineStageAIDCoordination
	if src == EventSourceCixHTTP {
		stage = eventtypes.PipelineStageExternalIngest
	}
	res, err := svc.EmitEvent(ctx, &EmitEventInput{
		EventType:       EventTypeCixAnalysisRequested,
		EventSource:     src,
		PipelineStage:   stage,
		EntityType:      "conversation",
		EntityID:      conversationID,
		OrgID:         ownerOrgID.Hex(),
		OwnerOrgID:    ownerOrgID,
		Priority:      "high",
		Lane:          aidecisionmodels.EventLaneFast,
		TraceID:       strings.TrimSpace(traceID),
		CorrelationID: strings.TrimSpace(correlationID),
		Payload:       payload,
	})
	if err != nil {
		return "", err
	}
	return res.EventID, nil
}
