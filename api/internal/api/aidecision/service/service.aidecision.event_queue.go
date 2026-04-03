// Package aidecisionsvc — Event Queue Service cho decision_events_queue.
//
// Theo PLATFORM_L1_EVENT_DECISION_SUPPLEMENT §2. Emit event vào queue.
package aidecisionsvc

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/traceutil"
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
	EventID     string `json:"eventId"`
	Status      string `json:"status"`
	TraceID     string `json:"traceId,omitempty"`
	W3CTraceID  string `json:"w3cTraceId,omitempty"`
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
		PriorityRank:       aidecisionmodels.PriorityRankFromString(input.Priority),
		Lane:               input.Lane,
		Status:             aidecisionmodels.EventStatusPending,
		TraceID:            input.TraceID,
		CorrelationID:      input.CorrelationID,
		Payload:            input.Payload,
		AttemptCount:       0,
		MaxAttempts:        5,
		CreatedAt:          now,
	}
	if tid := strings.TrimSpace(input.TraceID); tid != "" {
		doc.W3CTraceID = traceutil.W3CTraceIDFromKey(tid)
	}

	if _, err := coll.InsertOne(ctx, doc); err != nil {
		return nil, err
	}

	decisionlive.RecordCommandCenterIntake(input.OwnerOrgID, input.EventType, input.EventSource)
	decisionlive.RefreshQueueDepthForOrg(ctx, input.OwnerOrgID)

	res := &EmitEventResult{
		EventID: eventID,
		Status:  aidecisionmodels.EventStatusPending,
		TraceID: input.TraceID,
	}
	if strings.TrimSpace(input.TraceID) != "" {
		res.W3CTraceID = traceutil.W3CTraceIDFromKey(strings.TrimSpace(input.TraceID))
	}
	return res, nil
}

// PersistDecisionEventTraceFields ghi traceId / correlationId / w3cTraceId lên document queue sau ensureDecisionEventTraceIDs
// (bản ghi cũ thiếu hoặc consumer vừa sinh trace mới) — để tra cứu Mongo khớp response API / OTel.
func (s *AIDecisionService) PersistDecisionEventTraceFields(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || strings.TrimSpace(evt.EventID) == "" {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return mongo.ErrNoDocuments
	}
	set := bson.M{}
	if t := strings.TrimSpace(evt.TraceID); t != "" {
		set["traceId"] = t
	}
	if c := strings.TrimSpace(evt.CorrelationID); c != "" {
		set["correlationId"] = c
	}
	if w := strings.TrimSpace(evt.W3CTraceID); w != "" {
		set["w3cTraceId"] = w
	}
	if len(set) == 0 {
		return nil
	}
	_, err := coll.UpdateOne(ctx, bson.M{"eventId": evt.EventID}, bson.M{"$set": set})
	return err
}

// LeaseOne lấy 1 event pending để xử lý (theo lane).
// Trả về nil nếu không có event.
func (s *AIDecisionService) LeaseOne(ctx context.Context, lane, workerID string, leaseDurationSec int) (*aidecisionmodels.DecisionEvent, error) {
	return s.leaseOneWithExtraFilter(ctx, lane, workerID, leaseDurationSec, nil)
}

// LeaseOneFair ưu tiên event của org không nằm trong preferNotOrgs (fair queue — supplement §2.8).
// preferNotOrgs thường là vài org vừa xử lý gần đây để tránh một tenant chiếm hết slot.
func (s *AIDecisionService) LeaseOneFair(ctx context.Context, lane, workerID string, leaseDurationSec int, preferNotOrgs []primitive.ObjectID) (*aidecisionmodels.DecisionEvent, error) {
	var exclude []interface{}
	for _, id := range preferNotOrgs {
		if !id.IsZero() {
			exclude = append(exclude, id)
		}
	}
	if len(exclude) > 0 {
		doc, err := s.leaseOneWithExtraFilter(ctx, lane, workerID, leaseDurationSec, bson.M{
			"ownerOrganizationId": bson.M{"$nin": exclude},
		})
		if err != nil {
			return nil, err
		}
		if doc != nil {
			return doc, nil
		}
	}
	return s.LeaseOne(ctx, lane, workerID, leaseDurationSec)
}

func (s *AIDecisionService) leaseOneWithExtraFilter(ctx context.Context, lane, workerID string, leaseDurationSec int, extra bson.M) (*aidecisionmodels.DecisionEvent, error) {
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
	for k, v := range extra {
		filter[k] = v
	}

	update := bson.M{
		"$set": bson.M{
			"status":      aidecisionmodels.EventStatusLeased,
			"leasedBy":    workerID,
			"leasedUntil": leasedUntil,
		},
	}

	opts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "priorityRank", Value: 1}, {Key: "createdAt", Value: 1}}).
		SetReturnDocument(options.After)

	var doc aidecisionmodels.DecisionEvent
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	decisionlive.RefreshQueueDepthForOrg(ctx, doc.OwnerOrganizationID)
	return &doc, nil
}

// MigrateDecisionEventsPriorityRank gán priorityRank cho bản ghi pending cũ (idempotent, mỗi lần khởi động).
func MigrateDecisionEventsPriorityRank(ctx context.Context) (int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return 0, mongo.ErrNoDocuments
	}
	var total int64
	for _, p := range []struct {
		pri  string
		rank int
	}{
		{"high", 1},
		{"normal", 2},
		{"low", 3},
	} {
		res, err := coll.UpdateMany(ctx, bson.M{
			"status":       aidecisionmodels.EventStatusPending,
			"priority":     p.pri,
			"priorityRank": bson.M{"$exists": false},
		}, bson.M{"$set": bson.M{"priorityRank": p.rank}})
		if err != nil {
			return total, err
		}
		total += res.ModifiedCount
	}
	res, err := coll.UpdateMany(ctx, bson.M{
		"status":       aidecisionmodels.EventStatusPending,
		"priorityRank": bson.M{"$exists": false},
	}, bson.M{"$set": bson.M{"priorityRank": aidecisionmodels.PriorityRankFromString("")}})
	if err != nil {
		return total, err
	}
	total += res.ModifiedCount
	return total, nil
}

// EscalateStalePendingEvents nâng priority lên high cho event pending quá lâu (mặc định 30 phút — supplement priority escalation).
// Trả về số document đã cập nhật.
func (s *AIDecisionService) EscalateStalePendingEvents(ctx context.Context) (int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return 0, mongo.ErrNoDocuments
	}
	staleSec := int64(1800)
	if v := strings.TrimSpace(os.Getenv("AI_DECISION_ESCALATE_STALE_SEC")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			staleSec = n
		}
	}
	now := time.Now().UnixMilli()
	cutoff := now - staleSec*1000
	filter := bson.M{
		"status": aidecisionmodels.EventStatusPending,
		"priority": bson.M{"$ne": "high"},
		"createdAt": bson.M{"$lt": cutoff},
		"$or": []bson.M{
			{"scheduledAt": nil},
			{"scheduledAt": bson.M{"$lte": now}},
		},
	}
	res, err := coll.UpdateMany(ctx, filter, bson.M{
		"$set": bson.M{"priority": "high", "priorityRank": aidecisionmodels.PriorityRankFromString("high")},
	})
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
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
		SetSort(bson.D{{Key: "priorityRank", Value: 1}, {Key: "createdAt", Value: 1}}).
		SetReturnDocument(options.After)

	var doc aidecisionmodels.DecisionEvent
	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	decisionlive.RefreshQueueDepthForOrg(ctx, doc.OwnerOrganizationID)
	return &doc, nil
}

// CompleteEvent đánh dấu event đã xử lý xong (handler đã chạy hoặc luồng tương đương).
func (s *AIDecisionService) CompleteEvent(ctx context.Context, eventID string) error {
	return s.CompleteEventWithStatus(ctx, eventID, aidecisionmodels.EventStatusCompleted)
}

// CompleteEventWithStatus đánh dấu đóng job thành công với trạng thái terminal tường minh (completed | completed_no_handler | completed_routing_skipped).
func (s *AIDecisionService) CompleteEventWithStatus(ctx context.Context, eventID string, status string) error {
	if !isValidDecisionQueueCompletedStatus(status) {
		status = aidecisionmodels.EventStatusCompleted
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return mongo.ErrNoDocuments
	}
	var pre aidecisionmodels.DecisionEvent
	_ = coll.FindOne(ctx, bson.M{"eventId": eventID}).Decode(&pre)
	res, err := coll.UpdateOne(ctx, bson.M{"eventId": eventID}, bson.M{
		"$set": bson.M{"status": status},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount > 0 && !pre.OwnerOrganizationID.IsZero() {
		decisionlive.RefreshQueueDepthForOrg(ctx, pre.OwnerOrganizationID)
	}
	return nil
}

func isValidDecisionQueueCompletedStatus(s string) bool {
	switch s {
	case aidecisionmodels.EventStatusCompleted,
		aidecisionmodels.EventStatusCompletedNoHandler,
		aidecisionmodels.EventStatusCompletedRoutingSkipped:
		return true
	default:
		return false
	}
}

// FailEvent đánh dấu event thất bại. retryable=true → scheduled_at + backoff, status=pending.
func (s *AIDecisionService) FailEvent(ctx context.Context, eventID string, retryable bool, errMsg string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return mongo.ErrNoDocuments
	}
	var pre aidecisionmodels.DecisionEvent
	_ = coll.FindOne(ctx, bson.M{"eventId": eventID}).Decode(&pre)
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
		idx := pre.AttemptCount
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
	res, err := coll.UpdateOne(ctx, bson.M{"eventId": eventID}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount > 0 && !pre.OwnerOrganizationID.IsZero() {
		decisionlive.RefreshQueueDepthForOrg(ctx, pre.OwnerOrganizationID)
	}
	return nil
}

// DefaultLaneForEventType map event_type → lane mặc định.
func DefaultLaneForEventType(eventType string) string {
	switch eventType {
	case eventtypes.ConversationMessageInserted, eventtypes.MessageBatchReady,
		eventtypes.ConversationInserted, eventtypes.ConversationUpdated, eventtypes.MessageInserted, eventtypes.MessageUpdated,
		eventtypes.CixAnalysisRequested, eventtypes.CustomerContextRequested, eventtypes.CustomerContextReady,
		eventtypes.OrderInserted, eventtypes.OrderUpdated, eventtypes.OrderRecomputeRequested,
		eventtypes.OrderIntelligenceRequested, // Order Intelligence — cùng lane fast với order.*
		eventtypes.OrderIntelRecomputed, eventtypes.CixIntelRecomputed,
		EventTypeExecutorProposeRequested, EventTypeAdsProposeRequested, EventTypeExecuteRequested:
		return aidecisionmodels.EventLaneFast
	case eventtypes.AdsUpdated, eventtypes.AdsContextReady, eventtypes.AdsContextRequested:
		return aidecisionmodels.EventLaneBatch
	default:
		return aidecisionmodels.EventLaneNormal
	}
}
