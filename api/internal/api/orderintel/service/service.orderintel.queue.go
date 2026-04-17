// Package orderintelsvc — Hàng đợi domain order_intel_compute (Raw→L3→Flags tính ở worker domain, không trong consumer AI Decision).
package orderintelsvc

import (
	"context"
	"strings"
	"time"

	crmqueue "meta_commerce/internal/api/aidecision/crmqueue"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	orderintelmodels "meta_commerce/internal/api/orderintel/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnqueueOrderIntelligenceFromParent đưa job vào order_intel_compute sau order.inserted/updated (đã hydrate).
// enqueueSourceDomain rỗng → aidecision (consumer AID).
func EnqueueOrderIntelligenceFromParent(ctx context.Context, parent *aidecisionmodels.DecisionEvent, enqueueSourceDomain string) error {
	if parent == nil || parent.Payload == nil {
		return nil
	}
	ownerOrgID := parent.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	orderUid := strFromPayload(parent.Payload, "orderUid")
	if orderUid == "" {
		orderUid = strFromPayload(parent.Payload, "uid")
	}
	norm := ""
	if u, ok := parent.Payload["normalizedRecordUid"].(string); ok {
		norm = strings.TrimSpace(u)
	}
	mongoHex := ""
	if orderUid == "" {
		mongoHex = strings.TrimSpace(parent.EntityID)
		if norm != "" {
			mongoHex = norm
		}
	}
	job := &orderintelmodels.OrderIntelComputeJob{
		OrderUid:            orderUid,
		OwnerOrganizationID: ownerOrgID,
		NormalizedRecordUid: norm,
		OrgID:               parent.OrgID,
		TraceID:             parent.TraceID,
		CorrelationID:       parent.CorrelationID,
		ParentEventID:       parent.EventID,
		ParentEventType:     parent.EventType,
		Source:              "aidecision_order",
	}
	if parent.Payload != nil {
		job.CausalOrderingAtMs = crmqueue.ExtractCausalOrderingAtMs(parent.Payload)
	}
	if job.CausalOrderingAtMs <= 0 {
		job.CausalOrderingAtMs = time.Now().UnixMilli()
	}
	if orderUid == "" {
		job.MongoRecordIdHex = mongoHex
	}
	if job.OrderUid == "" && job.MongoRecordIdHex == "" {
		return nil
	}
	copyOrderIntelBusMeta(parent, job, enqueueSourceDomain)
	return upsertOrderIntelComputeJob(ctx, job)
}

func copyOrderIntelBusMeta(evt *aidecisionmodels.DecisionEvent, job *orderintelmodels.OrderIntelComputeJob, enqueueSourceDomain string) {
	if evt == nil || job == nil {
		return
	}
	job.EventType = strings.TrimSpace(evt.EventType)
	job.EventSource = strings.TrimSpace(evt.EventSource)
	job.PipelineStage = strings.TrimSpace(evt.PipelineStage)
	job.OwnerDomain = crmqueue.OwnerDomainFromDecisionPayload(evt.Payload)
	job.ProcessorDomain = crmqueue.ProcessorDomainOrder
	if es := strings.TrimSpace(enqueueSourceDomain); es != "" {
		job.EnqueueSourceDomain = es
	} else {
		job.EnqueueSourceDomain = crmqueue.EnqueueSourceAIDecision
	}
	job.E2EStage = strings.TrimSpace(evt.E2EStage)
	job.E2EStepID = strings.TrimSpace(evt.E2EStepID)
}

// EnqueueFromRecomputeDecisionEvent — consumer AI Decision chỉ chuyển job sang domain (không tính toán tại đây).
func EnqueueFromRecomputeDecisionEvent(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	return enqueueFromGenericAIDecisionPayload(ctx, evt, "recompute")
}

// EnqueueFromLegacyIntelligenceRequestedDecisionEvent — tương thích event order.intelligence_requested cũ trong queue: chỉ enqueue domain.
func EnqueueFromLegacyIntelligenceRequestedDecisionEvent(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	return enqueueFromGenericAIDecisionPayload(ctx, evt, "legacy_intelligence_requested")
}

func enqueueFromGenericAIDecisionPayload(ctx context.Context, evt *aidecisionmodels.DecisionEvent, source string) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
	normalizeOrderIntelligencePayload(evt)
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		if hex, ok := evt.Payload["ownerOrgIdHex"].(string); ok && hex != "" {
			if oid, err := primitive.ObjectIDFromHex(hex); err == nil {
				ownerOrgID = oid
			}
		}
	}
	if ownerOrgID.IsZero() {
		return nil
	}
	orderUid := strings.TrimSpace(strFromPayload(evt.Payload, "orderUid"))
	if orderUid == "" {
		orderUid = strings.TrimSpace(strFromPayload(evt.Payload, "uid"))
	}
	mongoHex := ""
	if orderUid == "" {
		mongoHex = strings.TrimSpace(evt.EntityID)
		if u, ok := evt.Payload["normalizedRecordUid"].(string); ok && strings.TrimSpace(u) != "" {
			mongoHex = strings.TrimSpace(u)
		}
	}
	parentEID, _ := evt.Payload["parentEventId"].(string)
	parentEType, _ := evt.Payload["parentEventType"].(string)
	if parentEID == "" {
		parentEID = evt.EventID
	}
	if parentEType == "" {
		parentEType = evt.EventType
	}
	norm := ""
	if u, ok := evt.Payload["normalizedRecordUid"].(string); ok {
		norm = strings.TrimSpace(u)
	}
	job := &orderintelmodels.OrderIntelComputeJob{
		OrderUid:            orderUid,
		OwnerOrganizationID: ownerOrgID,
		NormalizedRecordUid: norm,
		MongoRecordIdHex:    mongoHex,
		OrgID:               evt.OrgID,
		TraceID:             evt.TraceID,
		CorrelationID:       evt.CorrelationID,
		ParentEventID:       parentEID,
		ParentEventType:     parentEType,
		Source:              source,
	}
	if evt.Payload != nil {
		job.CausalOrderingAtMs = crmqueue.ExtractCausalOrderingAtMs(evt.Payload)
	}
	if job.CausalOrderingAtMs <= 0 {
		job.CausalOrderingAtMs = time.Now().UnixMilli()
	}
	if job.OrderUid == "" {
		job.MongoRecordIdHex = mongoHex
	}
	if job.OrderUid == "" && job.MongoRecordIdHex == "" {
		return nil
	}
	copyOrderIntelBusMeta(evt, job, crmqueue.EnqueueSourceAIDecision)
	return upsertOrderIntelComputeJob(ctx, job)
}

func upsertOrderIntelComputeJob(ctx context.Context, job *orderintelmodels.OrderIntelComputeJob) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.OrderIntelCompute)
	if !ok {
		return nil
	}
	now := time.Now().UnixMilli()
	filter := bson.M{"ownerOrganizationId": job.OwnerOrganizationID}
	if job.OrderUid != "" {
		filter["orderUid"] = job.OrderUid
	} else {
		filter["mongoRecordIdHex"] = job.MongoRecordIdHex
	}
	set := bson.M{
		"orgId":               job.OrgID,
		"traceId":             job.TraceID,
		"correlationId":       job.CorrelationID,
		"parentEventId":       job.ParentEventID,
		"parentEventType":     job.ParentEventType,
		"source":              job.Source,
		"normalizedRecordUid": job.NormalizedRecordUid,
		"causalOrderingAtMs":  job.CausalOrderingAtMs,
		"eventType":             job.EventType,
		"eventSource":           job.EventSource,
		"pipelineStage":         job.PipelineStage,
		"ownerDomain":           job.OwnerDomain,
		"processorDomain":       job.ProcessorDomain,
		"enqueueSourceDomain":   job.EnqueueSourceDomain,
		"e2eStage":              job.E2EStage,
		"e2eStepId":             job.E2EStepID,
		"processedAt":           nil,
		"processError":        "",
		"retryCount":          0,
	}
	update := bson.M{
		"$set": set,
		"$setOnInsert": bson.M{
			"createdAt":           now,
			"orderUid":            job.OrderUid,
			"ownerOrganizationId": job.OwnerOrganizationID,
			"mongoRecordIdHex":    job.MongoRecordIdHex,
		},
	}
	_, err := coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
