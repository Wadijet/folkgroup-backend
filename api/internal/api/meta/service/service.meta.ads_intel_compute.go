// Package metasvc — Enqueue ads_intel_compute từ consumer AI Decision; worker domain ads poll (recompute / context_ready / recalculate_all).
//
// Luồng bước 4: ads.context_requested → EnqueueAdsIntelComputeContextReady → RunAdsIntelComputeJob (context_ready)
// → emitAdsContextReadyFromIntelJob → ads.context_ready (consumer xử lý bước 5).
package metasvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	adsmodels "meta_commerce/internal/api/ads_meta/models"
	"meta_commerce/internal/api/aidecision/eventemit"
	"meta_commerce/internal/api/aidecision/eventtypes"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EnqueueAdsIntelCompute đưa job recompute một entity vào ads_intel_compute (không tính toán tại đây).
// causalOrderingAtMs: mốc nghiệp vụ từ payload event; 0 = gán bằng thời điểm enqueue.
func EnqueueAdsIntelCompute(ctx context.Context, objectType, objectID, adAccountID string, ownerOrgID primitive.ObjectID, source, recomputeMode, parentDecisionEventID string, causalOrderingAtMs int64, parentTraceID, parentCorrelationID string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsIntelCompute)
	if !ok {
		return fmt.Errorf("collection AdsIntelCompute chưa đăng ký")
	}
	now := time.Now().UnixMilli()
	job := &adsmodels.AdsIntelComputeJob{
		ID:                    primitive.NewObjectID(),
		JobKind:               adsmodels.AdsIntelComputeKindRecomputeOne,
		ObjectType:            objectType,
		ObjectID:              objectID,
		AdAccountID:           adAccountID,
		Source:                source,
		RecomputeMode:         recomputeMode,
		OwnerOrganizationID:   ownerOrgID,
		ParentDecisionEventID: parentDecisionEventID,
		ParentTraceID:         strings.TrimSpace(parentTraceID),
		ParentCorrelationID:   strings.TrimSpace(parentCorrelationID),
		CausalOrderingAtMs:    normalizeAdsIntelCausalMs(causalOrderingAtMs),
		CreatedAt:             now,
	}
	_, err := coll.InsertOne(ctx, job)
	return err
}

// EnqueueAdsIntelComputeRecalculateAll đưa job batch RecalculateAll vào ads_intel_compute.
func EnqueueAdsIntelComputeRecalculateAll(ctx context.Context, ownerOrgID primitive.ObjectID, limit int, parentDecisionEventID string, causalOrderingAtMs int64, parentTraceID, parentCorrelationID string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsIntelCompute)
	if !ok {
		return fmt.Errorf("collection AdsIntelCompute chưa đăng ký")
	}
	now := time.Now().UnixMilli()
	job := &adsmodels.AdsIntelComputeJob{
		ID:                    primitive.NewObjectID(),
		JobKind:               adsmodels.AdsIntelComputeKindRecalculateAll,
		OwnerOrganizationID:   ownerOrgID,
		RecalculateAllLimit:   limit,
		ParentDecisionEventID: parentDecisionEventID,
		ParentTraceID:         strings.TrimSpace(parentTraceID),
		ParentCorrelationID:   strings.TrimSpace(parentCorrelationID),
		CausalOrderingAtMs:    normalizeAdsIntelCausalMs(causalOrderingAtMs),
		CreatedAt:             now,
	}
	_, err := coll.InsertOne(ctx, job)
	return err
}

// EnqueueAdsIntelComputeContextReady đưa job đọc snapshot Intelligence + emit ads.context_ready vào ads_intel_compute (consumer không đọc meta_campaigns).
func EnqueueAdsIntelComputeContextReady(ctx context.Context, parentDecisionEventID, orgID, traceID, correlationID, campaignID, adAccountID string, ownerOrgID primitive.ObjectID, causalOrderingAtMs int64) error {
	campaignID = strings.TrimSpace(campaignID)
	if campaignID == "" || ownerOrgID.IsZero() {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsIntelCompute)
	if !ok {
		return fmt.Errorf("collection AdsIntelCompute chưa đăng ký")
	}
	now := time.Now().UnixMilli()
	job := &adsmodels.AdsIntelComputeJob{
		ID:                       primitive.NewObjectID(),
		JobKind:                  adsmodels.AdsIntelComputeKindContextReady,
		ObjectID:                 campaignID,
		AdAccountID:              adAccountID,
		OwnerOrganizationID:      ownerOrgID,
		ParentDecisionEventID:    parentDecisionEventID,
		CausalOrderingAtMs:       normalizeAdsIntelCausalMs(causalOrderingAtMs),
		ContextEmitOrgID:         strings.TrimSpace(orgID),
		ContextEmitTraceID:       strings.TrimSpace(traceID),
		ContextEmitCorrelationID: strings.TrimSpace(correlationID),
		CreatedAt:                now,
	}
	if job.ContextEmitOrgID == "" {
		job.ContextEmitOrgID = ownerOrgID.Hex()
	}
	_, err := coll.InsertOne(ctx, job)
	return err
}

// RunAdsIntelComputeJob thực thi một job (gọi từ worker domain ads).
func RunAdsIntelComputeJob(ctx context.Context, job *adsmodels.AdsIntelComputeJob) error {
	if job == nil {
		return fmt.Errorf("job nil")
	}
	switch job.JobKind {
	case adsmodels.AdsIntelComputeKindRecomputeOne:
		nowMs := time.Now().UnixMilli()
		err := ApplyAdsIntelligenceRecomputeWithMode(ctx, job.ObjectType, job.ObjectID, job.AdAccountID, job.OwnerOrganizationID, job.Source, job.RecomputeMode)
		persistAdsMetaIntelAfterRecomputeOne(ctx, job, err, nowMs)
		if err != nil {
			return err
		}
		// Luồng mới: sau khi tính xong Intelligence mới emit campaign_intel_recomputed (meta_ads_intel) → AI Decision.
		return emitCampaignIntelRecomputedAfterRecomputeJob(ctx, job)
	case adsmodels.AdsIntelComputeKindRecalculateAll:
		nowMs := time.Now().UnixMilli()
		_, err := RecalculateAllMetaAds(ctx, job.OwnerOrganizationID, job.RecalculateAllLimit)
		persistAdsMetaIntelAfterRecalculateAll(ctx, job, err, nowMs)
		return err
	case adsmodels.AdsIntelComputeKindContextReady:
		nowMs := time.Now().UnixMilli()
		err := emitAdsContextReadyFromIntelJob(ctx, job)
		persistAdsMetaIntelAfterContextReady(ctx, job, err, nowMs)
		return err
	default:
		return fmt.Errorf("jobKind không hợp lệ: %q", job.JobKind)
	}
}

// emitAdsContextReadyFromIntelJob — bước 4 (phần worker): đọc DB, đóng gói ads → ghi ads.context_ready vào queue.
func emitAdsContextReadyFromIntelJob(ctx context.Context, job *adsmodels.AdsIntelComputeJob) error {
	if job == nil || strings.TrimSpace(job.ObjectID) == "" {
		return nil
	}
	adsPayload := BuildAdsIntelligenceContextPayloadFromDB(ctx, job.ObjectID, job.AdAccountID, job.OwnerOrganizationID)
	entityID := strings.TrimSpace(job.AdAccountID)
	if entityID == "" {
		entityID = job.ObjectID
	}
	orgID := strings.TrimSpace(job.ContextEmitOrgID)
	if orgID == "" && !job.OwnerOrganizationID.IsZero() {
		orgID = job.OwnerOrganizationID.Hex()
	}
	_, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:       eventtypes.AdsContextReady,
		EventSource:     eventtypes.EventSourceMetaAdsIntel,
		PipelineStage:   eventtypes.PipelineStageDomainIntel,
		EntityType:      "ad_account",
		EntityID:      entityID,
		OrgID:         orgID,
		OwnerOrgID:    job.OwnerOrganizationID,
		Priority:      "normal",
		Lane:          "batch",
		TraceID:       job.ContextEmitTraceID,
		CorrelationID: job.ContextEmitCorrelationID,
		Payload: map[string]interface{}{
			"adAccountId": job.AdAccountID,
			"campaignId":  job.ObjectID,
			"ads":         adsPayload,
		},
	})
	return err
}
