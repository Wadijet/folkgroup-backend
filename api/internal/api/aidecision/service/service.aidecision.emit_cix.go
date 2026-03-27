// Package aidecisionsvc — Emit cix.analysis_requested (queue AI Decision).
package aidecisionsvc

import (
	"context"
	"fmt"
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EventTypeCixAnalysisRequested — CixRequestWorker consume → EnqueueAnalysis → cix_pending_analysis.
const EventTypeCixAnalysisRequested = "cix.analysis_requested"

// EmitCixAnalysisRequested đưa yêu cầu phân tích CIX vào decision_events_queue.
// Luồng: CixRequestWorker → EnqueueAnalysis → CixAnalysisWorker → AnalyzeSession (không gọi AnalyzeSession từ HTTP).
func EmitCixAnalysisRequested(ctx context.Context, conversationID, customerID, channel string, ownerOrgID primitive.ObjectID, normalizedRecordUID, traceID, correlationID string) (eventID string, err error) {
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
	res, err := svc.EmitEvent(ctx, &EmitEventInput{
		EventType:     EventTypeCixAnalysisRequested,
		EventSource:   "cix_api",
		EntityType:    "conversation",
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
