// Package conversationintel — Hook sau datachanged fb_message_items: gấp + gom → conversation.intelligence_requested.
package conversationintel

import (
	"context"
	"strings"

	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Từ khóa gấp (flush ngay, không chờ cửa sổ gom) — đồng bộ tinh thần aidecision debounce criticalPatterns.
var urgentMessagePatterns = []string{"huỷ đơn", "hủy đơn", "cancel", "tôi muốn huỷ", "khiếu nại", "complaint"}

// ProcessDataChangeForMessageItem gọi từ applyDatachangedSideEffects khi nguồn là fb_message_items.
// Trích conversationId, gom theo hội thoại, rồi emit conversation.intelligence_requested (sau debounce).
func ProcessDataChangeForMessageItem(ctx context.Context, e events.DataChangeEvent, traceID, correlationID, orgIDHex string) {
	if e.Document == nil {
		return
	}
	ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
	if ownerOrgID.IsZero() {
		return
	}
	raw, ok := e.Document.(bson.M)
	if !ok {
		return
	}
	convID := strings.TrimSpace(stringField(raw, "conversationId"))
	if convID == "" {
		convID = strings.TrimSpace(stringField(raw, "ConversationId"))
	}
	if convID == "" {
		return
	}

	text := messageItemTextForUrgency(raw)
	debounceMs := EffectiveDebounceMs()
	if isUrgentMessageItemText(text) {
		debounceMs = DebounceMsUrgent
	}

	customerID := resolveCustomerIDForConversation(ctx, convID, ownerOrgID)
	channel := "messenger"

	key := debounceKey(ownerOrgHex(ownerOrgID), convID)
	tid, cid := strings.TrimSpace(traceID), strings.TrimSpace(correlationID)
	orgHex := strings.TrimSpace(orgIDHex)
	if orgHex == "" {
		orgHex = ownerOrgID.Hex()
	}

	scheduleConversationIntel(key, debounceMs, func() {
		bg := context.Background()
		_, _ = aidecisionsvc.EmitConversationIntelligenceRequested(bg, convID, customerID, channel, tid, cid, ownerOrgID, orgHex)
	})
}

func ownerOrgHex(id primitive.ObjectID) string {
	if id.IsZero() {
		return ""
	}
	return id.Hex()
}

func stringField(m bson.M, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func messageItemTextForUrgency(m bson.M) string {
	md, _ := m["messageData"].(map[string]interface{})
	if md == nil {
		md, _ = m["MessageData"].(map[string]interface{})
	}
	if md == nil {
		return ""
	}
	for _, k := range []string{"text", "message", "body", "content"} {
		switch x := md[k].(type) {
		case string:
			if strings.TrimSpace(x) != "" {
				return strings.ToLower(x)
			}
		}
	}
	return ""
}

func isUrgentMessageItemText(text string) bool {
	if text == "" {
		return false
	}
	for _, p := range urgentMessagePatterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

func resolveCustomerIDForConversation(ctx context.Context, conversationID string, ownerOrgID primitive.ObjectID) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || ownerOrgID.IsZero() {
		return ""
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok || coll == nil {
		return ""
	}
	var doc struct {
		CustomerID string `bson:"customerId"`
	}
	err := coll.FindOne(ctx, bson.M{
		"conversationId":      conversationID,
		"ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.CustomerID)
}
