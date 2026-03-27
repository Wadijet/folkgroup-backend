// Package eventemit — ghi event vào decision_events_queue không phụ thuộc package aidecisionsvc đầy đủ (tránh import cycle với internal/worker).
package eventemit

import (
	"context"
	"time"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// EmitInput tham số ghi một event (tương đương aidecisionsvc.EmitEventInput).
type EmitInput struct {
	EventType     string
	EventSource   string
	EntityType    string
	EntityID      string
	OrgID         string
	OwnerOrgID    primitive.ObjectID
	Priority      string
	Lane          string
	TraceID       string
	CorrelationID string
	Payload       map[string]interface{}
}

// EmitResult kết quả ghi event.
type EmitResult struct {
	EventID string
	Status  string
}

// EmitDecisionEvent ghi một bản ghi vào decision_events_queue.
func EmitDecisionEvent(ctx context.Context, input *EmitInput) (*EmitResult, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.DecisionEventsQueue)
	if !ok {
		return nil, mongo.ErrNoDocuments
	}

	now := time.Now().UnixMilli()
	eventID := utility.GenerateUID(utility.UIDPrefixEvent)

	doc := &aidecisionmodels.DecisionEvent{
		EventID:             eventID,
		EventType:           input.EventType,
		EventSource:         input.EventSource,
		EntityType:          input.EntityType,
		EntityID:            input.EntityID,
		OrgID:               input.OrgID,
		OwnerOrganizationID: input.OwnerOrgID,
		Priority:            input.Priority,
		PriorityRank:        aidecisionmodels.PriorityRankFromString(input.Priority),
		Lane:                input.Lane,
		Status:              aidecisionmodels.EventStatusPending,
		TraceID:             input.TraceID,
		CorrelationID:       input.CorrelationID,
		Payload:             input.Payload,
		AttemptCount:        0,
		MaxAttempts:         5,
		CreatedAt:           now,
	}

	if _, err := coll.InsertOne(ctx, doc); err != nil {
		return nil, err
	}

	return &EmitResult{
		EventID: eventID,
		Status:  aidecisionmodels.EventStatusPending,
	}, nil
}
