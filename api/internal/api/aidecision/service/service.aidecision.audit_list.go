// Package aidecisionsvc — List case / queue cho màn trace & audit (read-only, theo org).
package aidecisionsvc

import (
	"context"
	"errors"
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	auditListDefaultLimit = 20
	auditListMaxLimit     = 100
)

// AuditDefaultListLimit page size mặc định cho API list audit (handler dùng khi query thiếu limit).
func AuditDefaultListLimit() int { return auditListDefaultLimit }

// ListDecisionCasesFilter bộ lọc + phân trang list decision_cases_runtime.
type ListDecisionCasesFilter struct {
	OwnerOrganizationID primitive.ObjectID
	Page                int
	Limit               int
	Status              string
	CaseType            string
	TraceID             string
	FromUpdatedMs       *int64
	ToUpdatedMs         *int64
}

// ListDecisionCases trả danh sách case theo org (updatedAt giảm dần).
func (s *AIDecisionService) ListDecisionCases(ctx context.Context, f ListDecisionCasesFilter) ([]aidecisionmodels.DecisionCase, int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionCasesRuntime)
	if !ok {
		return nil, 0, errors.New("không tìm thấy collection decision_cases_runtime")
	}
	if f.OwnerOrganizationID.IsZero() {
		return nil, 0, errors.New("ownerOrganizationId bắt buộc")
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 {
		limit = auditListDefaultLimit
	}
	if limit > auditListMaxLimit {
		limit = auditListMaxLimit
	}
	filter := bson.M{"ownerOrganizationId": f.OwnerOrganizationID}
	if t := strings.TrimSpace(f.Status); t != "" {
		filter["status"] = t
	}
	if t := strings.TrimSpace(f.CaseType); t != "" {
		filter["caseType"] = t
	}
	if t := strings.TrimSpace(f.TraceID); t != "" {
		filter["traceId"] = t
	}
	if f.FromUpdatedMs != nil || f.ToUpdatedMs != nil {
		rng := bson.M{}
		if f.FromUpdatedMs != nil {
			rng["$gte"] = *f.FromUpdatedMs
		}
		if f.ToUpdatedMs != nil {
			rng["$lte"] = *f.ToUpdatedMs
		}
		filter["updatedAt"] = rng
	}
	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []aidecisionmodels.DecisionCase{}, 0, nil
	}
	skip := int64(page-1) * int64(limit)
	if skip < 0 {
		skip = 0
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))
	cur, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []aidecisionmodels.DecisionCase
	for cur.Next(ctx) {
		var doc aidecisionmodels.DecisionCase
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, err
		}
		out = append(out, doc)
	}
	return out, total, cur.Err()
}

// ListQueueEventsFilter bộ lọc + phân trang decision_events_queue.
type ListQueueEventsFilter struct {
	OwnerOrganizationID primitive.ObjectID
	Page                int
	Limit               int
	Status              string
	EventType           string
	TraceID             string
	FromCreatedMs       *int64
	ToCreatedMs         *int64
	IncludePayload      bool
}

// ListQueueEvents trả danh sách envelope queue theo org (createdAt giảm dần).
// Payload có thể lớn — mặc định không trả payload (IncludePayload=false) để list nhẹ.
func (s *AIDecisionService) ListQueueEvents(ctx context.Context, f ListQueueEventsFilter) ([]aidecisionmodels.DecisionEvent, int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return nil, 0, errors.New("không tìm thấy collection decision_events_queue")
	}
	if f.OwnerOrganizationID.IsZero() {
		return nil, 0, errors.New("ownerOrganizationId bắt buộc")
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	limit := f.Limit
	if limit < 1 {
		limit = auditListDefaultLimit
	}
	if limit > auditListMaxLimit {
		limit = auditListMaxLimit
	}
	filter := bson.M{"ownerOrganizationId": f.OwnerOrganizationID}
	if t := strings.TrimSpace(f.Status); t != "" {
		filter["status"] = t
	}
	if t := strings.TrimSpace(f.EventType); t != "" {
		filter["eventType"] = t
	}
	if t := strings.TrimSpace(f.TraceID); t != "" {
		filter["traceId"] = t
	}
	if f.FromCreatedMs != nil || f.ToCreatedMs != nil {
		rng := bson.M{}
		if f.FromCreatedMs != nil {
			rng["$gte"] = *f.FromCreatedMs
		}
		if f.ToCreatedMs != nil {
			rng["$lte"] = *f.ToCreatedMs
		}
		filter["createdAt"] = rng
	}
	total, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []aidecisionmodels.DecisionEvent{}, 0, nil
	}
	skip := int64(page-1) * int64(limit)
	if skip < 0 {
		skip = 0
	}
	proj := bson.M{
		"eventId": 1, "eventType": 1, "eventSource": 1, "entityType": 1, "entityId": 1,
		"orgId": 1, "ownerOrganizationId": 1, "priority": 1, "priorityRank": 1, "lane": 1,
		"status": 1, "traceId": 1, "w3cTraceId": 1, "correlationId": 1,
		"scheduledAt": 1, "attemptCount": 1, "maxAttempts": 1,
		"leasedBy": 1, "leasedUntil": 1, "error": 1, "createdAt": 1,
		"parentEventId": 1, "rootEventId": 1, "causationEventId": 1,
	}
	if f.IncludePayload {
		proj["payload"] = 1
	}
	opts := options.Find().
		SetProjection(proj).
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))
	cur, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	var out []aidecisionmodels.DecisionEvent
	for cur.Next(ctx) {
		var doc aidecisionmodels.DecisionEvent
		if err := cur.Decode(&doc); err != nil {
			return nil, 0, err
		}
		out = append(out, doc)
	}
	return out, total, cur.Err()
}
