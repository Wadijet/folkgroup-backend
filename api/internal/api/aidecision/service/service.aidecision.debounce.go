// Package aidecisionsvc — Debounce: gom message, emit message.batch_ready sau window.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §2.6. Debounce key: org_id:conversation_id:event_group.
package aidecisionsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const debounceWindowSec = 30
const eventGroupMessageBurst = "message_burst"

// CriticalPatterns từ khóa flush ngay, không chờ window.
var criticalPatterns = []string{"huỷ đơn", "hủy đơn", "cancel", "tôi muốn huỷ"}

// UpsertDebounceState cập nhật debounce state khi có event mới.
// Trả về shouldFlushImmediate=true nếu match critical pattern.
func (s *AIDecisionService) UpsertDebounceState(ctx context.Context, orgID string, ownerOrgID primitive.ObjectID, conversationID, customerID, channel, eventID string, payload map[string]interface{}) (shouldFlushImmediate bool, err error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionDebounceState)
	if !ok {
		return false, nil
	}

	// Kiểm tra critical pattern
	text := ""
	if payload != nil {
		if msg, ok := payload["lastMessage"].(string); ok {
			text = strings.ToLower(msg)
		}
	}
	for _, p := range criticalPatterns {
		if strings.Contains(text, p) {
			return true, nil
		}
	}

	now := time.Now().UnixMilli()
	debounceKey := fmt.Sprintf("%s:%s:%s", orgID, conversationID, eventGroupMessageBurst)

	filter := bson.M{"debounceKey": debounceKey}
	update := bson.M{
		"$set": bson.M{
			"orgId":          orgID,
			"ownerOrgId":     ownerOrgID,
			"conversationId": conversationID,
			"customerId":     customerID,
			"channel":        channel,
			"lastEventId":    eventID,
			"lastMessageAt":  now,
			"createdAt":      now,
		},
	}
	opts := options.Update().SetUpsert(true)
	_, err = coll.UpdateOne(ctx, filter, update, opts)
	return false, err
}

// FlushExpired tìm state hết window, emit message.batch_ready, xóa state.
func (s *AIDecisionService) FlushExpired(ctx context.Context) (int, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionDebounceState)
	if !ok {
		return 0, nil
	}

	now := time.Now().UnixMilli()
	cutoff := now - debounceWindowSec*1000

	cursor, err := coll.Find(ctx, bson.M{"lastMessageAt": bson.M{"$lt": cutoff}})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var states []aidecisionmodels.DebounceState
	if err = cursor.All(ctx, &states); err != nil {
		return 0, err
	}

	emitted := 0
	for _, st := range states {
		_, err = s.EmitEvent(ctx, &EmitEventInput{
			EventType:   "message.batch_ready",
			EventSource: "debounce",
			EntityType:  "conversation",
			EntityID:    st.ConversationID,
			OrgID:       st.OrgID,
			OwnerOrgID:  st.OwnerOrgID,
			Priority:    "high",
			Lane:        "fast",
			Payload: map[string]interface{}{
				"conversationId":      st.ConversationID,
				"customerId":          st.CustomerID,
				"channel":             st.Channel,
				"normalizedRecordUid": st.LastEventID,
			},
		})
		if err != nil {
			continue
		}
		_, _ = coll.DeleteOne(ctx, bson.M{"debounceKey": st.DebounceKey})
		emitted++
	}
	return emitted, nil
}
