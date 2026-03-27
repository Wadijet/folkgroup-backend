// Package aidecisionsvc — Emit conversation.intelligence_requested (bridge tới CIX / Conversation Intelligence).
package aidecisionsvc

import (
	"context"
	"fmt"
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EventTypeConversationIntelligenceRequested — consumer AI Decision → EmitCixAnalysisRequested.
const EventTypeConversationIntelligenceRequested = "conversation.intelligence_requested"

// EmitConversationIntelligenceRequested ghi event bridge sau khi gom tin nhắn chi tiết (fb_message_items).
func EmitConversationIntelligenceRequested(ctx context.Context, conversationID, customerID, channel, traceID, correlationID string, ownerOrgID primitive.ObjectID, orgIDHex string) (eventID string, err error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", fmt.Errorf("conversationId bắt buộc")
	}
	ch := strings.TrimSpace(channel)
	if ch == "" {
		ch = "messenger"
	}
	orgHex := strings.TrimSpace(orgIDHex)
	if orgHex == "" {
		orgHex = ownerOrgID.Hex()
	}
	svc := NewAIDecisionService()
	payload := map[string]interface{}{
		"conversationId": conversationID,
		"customerId":     strings.TrimSpace(customerID),
		"channel":        ch,
	}
	res, err := svc.EmitEvent(ctx, &EmitEventInput{
		EventType:     EventTypeConversationIntelligenceRequested,
		EventSource:   "fb_message_item",
		EntityType:    "conversation",
		EntityID:      conversationID,
		OrgID:         orgHex,
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
