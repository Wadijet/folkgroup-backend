// Package aidecisionsvc — Event Queue Service cho decision_events_queue.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §2. Emit event vào queue.
package aidecisionsvc

import (
	"context"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EmitEventInput input để emit event vào queue.
type EmitEventInput struct {
	EventType     string                 `json:"eventType"`
	EventSource   string                 `json:"eventSource"`
	EntityType    string                 `json:"entityType"`
	EntityID      string                 `json:"entityId"`
	OrgID         string                 `json:"orgId"`
	OwnerOrgID    primitive.ObjectID     `json:"ownerOrganizationId"`
	Priority      string                 `json:"priority"` // high | normal | low
	Lane          string                 `json:"lane"`     // fast | normal | batch
	TraceID       string                 `json:"traceId,omitempty"`
	CorrelationID string                 `json:"correlationId,omitempty"`
	Payload       map[string]interface{} `json:"payload"`
}

// EmitEventResult kết quả emit.
type EmitEventResult struct {
	EventID string `json:"eventId"`
	Status  string `json:"status"`
}

// EmitEvent ghi event vào decision_events_queue.
func (s *AIDecisionService) EmitEvent(ctx context.Context, input *EmitEventInput) (*EmitEventResult, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return nil, mongo.ErrNoDocuments
	}

	now := time.Now().UnixMilli()
	eventID := utility.GenerateUID(utility.UIDPrefixEvent)

	doc := &aidecisionmodels.DecisionEvent{
		EventID:            eventID,
		EventType:          input.EventType,
		EventSource:        input.EventSource,
		EntityType:         input.EntityType,
		EntityID:           input.EntityID,
		OrgID:              input.OrgID,
		OwnerOrganizationID: input.OwnerOrgID,
		Priority:           input.Priority,
		Lane:               input.Lane,
		Status:             aidecisionmodels.EventStatusPending,
		TraceID:            input.TraceID,
		CorrelationID:      input.CorrelationID,
		Payload:            input.Payload,
		AttemptCount:       0,
		MaxAttempts:        5,
		CreatedAt:          now,
	}

	if _, err := coll.InsertOne(ctx, doc); err != nil {
		return nil, err
	}

	return &EmitEventResult{
		EventID: eventID,
		Status:  aidecisionmodels.EventStatusPending,
	}, nil
}

// LeaseOne lấy 1 event pending để xử lý (theo lane).
// Trả về nil nếu không có event.
func (s *AIDecisionService) LeaseOne(ctx context.Context, lane, workerID string, leaseDurationSec int) (*aidecisionmodels.DecisionEvent, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return nil, mongo.ErrNoDocuments
	}

	now := time.Now().UnixMilli()
	leasedUntil := now + int64(leaseDurationSec)*1000

	filter := bson.M{
		"status": aidecisionmodels.EventStatusPending,
		"lane":   lane,
		"$or": []bson.M{
			{"scheduledAt": nil},
			{"scheduledAt": bson.M{"$lte": now}},
		},
	}

	update := bson.M{
		"$set": bson.M{
			"status":      aidecisionmodels.EventStatusLeased,
			"leasedBy":    workerID,
			"leasedUntil": leasedUntil,
		},
	}

	opts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "priority", Value: 1}, {Key: "createdAt", Value: 1}}).
		SetReturnDocument(options.After)

	var doc aidecisionmodels.DecisionEvent
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &doc, nil
}

// LeaseOneByEventType lấy 1 event pending theo event_type (cho domain workers như CRM).
func (s *AIDecisionService) LeaseOneByEventType(ctx context.Context, eventType, lane, workerID string, leaseDurationSec int) (*aidecisionmodels.DecisionEvent, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return nil, mongo.ErrNoDocuments
	}

	now := time.Now().UnixMilli()
	leasedUntil := now + int64(leaseDurationSec)*1000

	filter := bson.M{
		"status":     aidecisionmodels.EventStatusPending,
		"eventType":  eventType,
		"lane":       lane,
		"$or": []bson.M{
			{"scheduledAt": nil},
			{"scheduledAt": bson.M{"$lte": now}},
		},
	}

	update := bson.M{
		"$set": bson.M{
			"status":      aidecisionmodels.EventStatusLeased,
			"leasedBy":    workerID,
			"leasedUntil": leasedUntil,
		},
	}

	opts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "priority", Value: 1}, {Key: "createdAt", Value: 1}}).
		SetReturnDocument(options.After)

	var doc aidecisionmodels.DecisionEvent
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &doc, nil
}

// CompleteEvent đánh dấu event đã xử lý xong.
func (s *AIDecisionService) CompleteEvent(ctx context.Context, eventID string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return mongo.ErrNoDocuments
	}
	_, err := coll.UpdateOne(ctx, bson.M{"eventId": eventID}, bson.M{
		"$set": bson.M{"status": aidecisionmodels.EventStatusCompleted},
	})
	return err
}

// FailEvent đánh dấu event thất bại. retryable=true → scheduled_at + backoff, status=pending.
func (s *AIDecisionService) FailEvent(ctx context.Context, eventID string, retryable bool, errMsg string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return mongo.ErrNoDocuments
	}
	now := time.Now().UnixMilli()
	update := bson.M{
		"$set": bson.M{
			"status": aidecisionmodels.EventStatusFailedTerminal,
			"error":  errMsg,
		},
		"$unset": bson.M{"leasedBy": "", "leasedUntil": ""},
	}
	if retryable {
		// Retry backoff: 1→5s, 2→30s, 3→2 phút, 4→10 phút
		delays := []int64{5000, 30000, 120000, 600000}
		var doc aidecisionmodels.DecisionEvent
		_ = coll.FindOne(ctx, bson.M{"eventId": eventID}).Decode(&doc)
		idx := doc.AttemptCount
		if idx >= len(delays) {
			idx = len(delays) - 1
		}
		scheduledAt := now + delays[idx]
		update = bson.M{
			"$set": bson.M{
				"status":      aidecisionmodels.EventStatusPending,
				"scheduledAt": scheduledAt,
				"error":       errMsg,
			},
			"$inc":  bson.M{"attemptCount": 1},
			"$unset": bson.M{"leasedBy": "", "leasedUntil": ""},
		}
	}
	_, err := coll.UpdateOne(ctx, bson.M{"eventId": eventID}, update)
	return err
}

// DefaultLaneForEventType map event_type → lane mặc định.
func DefaultLaneForEventType(eventType string) string {
	switch eventType {
	case "conversation.message_inserted", "message.batch_ready",
		"conversation.inserted", "conversation.updated", "message.inserted", "message.updated",
		"cix.analysis_requested", "cix.analysis_completed", "customer.context_requested", "customer.context_ready",
		"order.inserted", "order.updated", "order.recompute_requested", "order.flags_emitted",
		EventTypeAdsProposeRequested:
		return aidecisionmodels.EventLaneFast
	case "ads.updated", "ads.context_ready", "ads.context_requested":
		return aidecisionmodels.EventLaneBatch
	default:
		return aidecisionmodels.EventLaneNormal
	}
}
