// Package eventemit — ghi event vào decision_events_queue không phụ thuộc package aidecisionsvc đầy đủ (tránh import cycle với internal/worker).
package eventemit

import (
	"context"
	"strings"
	"time"

	"meta_commerce/internal/api/aidecision/eventtypes"
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
	PipelineStage string
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

	payload := clonePayloadMap(input.Payload)
	ref := eventtypes.ResolveE2EForQueueEnvelope(input.EventType, input.EventSource, input.PipelineStage)
	eventtypes.MergePayloadE2E(payload, ref)

	doc := &aidecisionmodels.DecisionEvent{
		EventID:             eventID,
		EventType:           input.EventType,
		EventSource:         input.EventSource,
		PipelineStage:       strings.TrimSpace(input.PipelineStage),
		E2EStage:            ref.Stage,
		E2EStepID:           ref.StepID,
		E2EStepLabelVi:      ref.LabelVi,
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
		Payload:             payload,
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

// clonePayloadMap sao chép payload để tránh sửa map gốc caller.
func clonePayloadMap(p map[string]interface{}) map[string]interface{} {
	if p == nil {
		return map[string]interface{}{}
	}
	out := make(map[string]interface{}, len(p))
	for k, v := range p {
		out[k] = v
	}
	return out
}
