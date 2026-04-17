// Package service — CixQueueService enqueue phân tích từ CIO event.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	cixmodels "meta_commerce/internal/api/cix/models"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	basesvc "meta_commerce/internal/api/base/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CixQueueService service hàng đợi phân tích CIX (collection cix_intel_compute).
type CixQueueService struct {
	*basesvc.BaseServiceMongoImpl[cixmodels.CixIntelComputeJob]
}

// NewCixQueueService tạo service mới.
func NewCixQueueService() (*CixQueueService, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.CixIntelCompute)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.CixIntelCompute, common.ErrNotFound)
	}
	return &CixQueueService{
		BaseServiceMongoImpl: basesvc.NewBaseServiceMongo[cixmodels.CixIntelComputeJob](coll),
	}, nil
}

// EnqueueAnalysisInput input để enqueue job phân tích.
type EnqueueAnalysisInput struct {
	ConversationID      string
	CustomerID          string
	Channel             string
	CioEventUid         string
	OwnerOrganizationID primitive.ObjectID
	TraceID             string
	CorrelationID       string
	CausalOrderingAtMs  int64
	DecisionEventID     string
	// BusEnvelope — bản sao eventType/eventSource/pipelineStage từ decision_events_queue (có thể nil).
	BusEnvelope *crmqueue.DomainQueueBusFields
}

// EnqueueAnalysis thêm job vào cix_intel_compute (upsert theo conversationId).
// Gọi từ CIO ingestion sau khi InsertOne cio_event (conversation_updated, message_updated).
func (s *CixQueueService) EnqueueAnalysis(ctx context.Context, input EnqueueAnalysisInput) error {
	if input.ConversationID == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	causal := input.CausalOrderingAtMs
	if causal <= 0 {
		causal = now
	}
	job := cixmodels.CixIntelComputeJob{
		ConversationID:      input.ConversationID,
		CustomerID:          input.CustomerID,
		Channel:             input.Channel,
		CioEventUid:         input.CioEventUid,
		OwnerOrganizationID: input.OwnerOrganizationID,
		TraceID:             strings.TrimSpace(input.TraceID),
		CorrelationID:       strings.TrimSpace(input.CorrelationID),
		CausalOrderingAtMs:  causal,
		DecisionEventID:     strings.TrimSpace(input.DecisionEventID),
		ProcessedAt:         nil,
		RetryCount:          0,
		CreatedAt:           now,
	}
	if input.BusEnvelope != nil {
		job.EventType = strings.TrimSpace(input.BusEnvelope.EventType)
		job.EventSource = strings.TrimSpace(input.BusEnvelope.EventSource)
		job.PipelineStage = strings.TrimSpace(input.BusEnvelope.PipelineStage)
		job.OwnerDomain = strings.TrimSpace(input.BusEnvelope.OwnerDomain)
		job.ProcessorDomain = strings.TrimSpace(input.BusEnvelope.ProcessorDomain)
		job.EnqueueSourceDomain = strings.TrimSpace(input.BusEnvelope.EnqueueSourceDomain)
		job.E2EStage = strings.TrimSpace(input.BusEnvelope.E2EStage)
		job.E2EStepID = strings.TrimSpace(input.BusEnvelope.E2EStepID)
	}
	filter := bson.M{
		"conversationId":      job.ConversationID,
		"ownerOrganizationId": job.OwnerOrganizationID,
	}
	setDoc := bson.M{
		"customerId":                          job.CustomerID,
		"channel":                             job.Channel,
		"cioEventUid":                         job.CioEventUid,
		crmqueue.PayloadKeyCausalOrderingAtMs: causal,
		"traceId":                             job.TraceID,
		"correlationId":                       job.CorrelationID,
		"decisionEventId":                     job.DecisionEventID,
		"processedAt":                         nil,
		"processError":                        "",
		"retryCount":                          job.RetryCount,
	}
	if input.BusEnvelope != nil {
		if v := strings.TrimSpace(input.BusEnvelope.EventType); v != "" {
			setDoc["eventType"] = v
		}
		if v := strings.TrimSpace(input.BusEnvelope.EventSource); v != "" {
			setDoc["eventSource"] = v
		}
		if v := strings.TrimSpace(input.BusEnvelope.PipelineStage); v != "" {
			setDoc["pipelineStage"] = v
		}
		if v := strings.TrimSpace(input.BusEnvelope.OwnerDomain); v != "" {
			setDoc["ownerDomain"] = v
		}
		if v := strings.TrimSpace(input.BusEnvelope.ProcessorDomain); v != "" {
			setDoc["processorDomain"] = v
		}
		if v := strings.TrimSpace(input.BusEnvelope.EnqueueSourceDomain); v != "" {
			setDoc["enqueueSourceDomain"] = v
		}
		if v := strings.TrimSpace(input.BusEnvelope.E2EStage); v != "" {
			setDoc["e2eStage"] = v
		}
		if v := strings.TrimSpace(input.BusEnvelope.E2EStepID); v != "" {
			setDoc["e2eStepId"] = v
		}
	}
	update := bson.M{
		"$set": setDoc,
		"$setOnInsert": bson.M{
			"createdAt": now,
		},
	}
	_, err := s.Collection().UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
