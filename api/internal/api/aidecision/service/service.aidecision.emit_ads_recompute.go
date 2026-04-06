// Package aidecisionsvc — Emit ads.intelligence.recompute_requested (queue AI Decision).
package aidecisionsvc

import (
	"context"
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EventTypeAdsIntelligenceRecomputeRequested — consumer chỉ enqueue ads_intel_compute; worker domain ads gọi ApplyAdsIntelligenceRecomputeWithMode.
const EventTypeAdsIntelligenceRecomputeRequested = eventtypes.AdsIntelligenceRecomputeRequested

// EventTypeAdsIntelligenceRecalculateAllRequested — consumer chỉ enqueue ads_intel_compute; worker domain ads gọi RecalculateAllMetaAds.
const EventTypeAdsIntelligenceRecalculateAllRequested = eventtypes.AdsIntelligenceRecalculateAllRequested

// EmitAdsIntelligenceRecomputeRequested đưa yêu cầu tính lại metrics Ads vào decision_events_queue.
// recomputeMode: rỗng hoặc "source" (hook); "full" (API tính lại full entity).
func EmitAdsIntelligenceRecomputeRequested(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, source string, recomputeMode string) (eventID string, err error) {
	svc := NewAIDecisionService()
	payload := map[string]interface{}{
		"objectType":    objectType,
		"objectId":      objectId,
		"adAccountId":   adAccountId,
		"ownerOrgIdHex": ownerOrgID.Hex(),
		"source":        source,
	}
	if strings.TrimSpace(recomputeMode) != "" {
		payload["recomputeMode"] = recomputeMode
	}
	eventSource := eventtypes.EventSourceMetaHooks
	if strings.TrimSpace(strings.ToLower(recomputeMode)) == "full" {
		eventSource = eventtypes.EventSourceMetaAPI
	}
	res, err := svc.EmitEvent(ctx, &EmitEventInput{
		EventType:   EventTypeAdsIntelligenceRecomputeRequested,
		EventSource: eventSource,
		EntityType:  objectType,
		EntityID:    objectId,
		OrgID:       ownerOrgID.Hex(),
		OwnerOrgID:  ownerOrgID,
		Priority:    "high",
		Lane:        aidecisionmodels.EventLaneFast,
		Payload:     payload,
	})
	if err != nil {
		return "", err
	}
	return res.EventID, nil
}

// EmitAdsIntelligenceRecalculateAllRequested enqueue batch RecalculateAllMetaAds (lane batch).
func EmitAdsIntelligenceRecalculateAllRequested(ctx context.Context, ownerOrgID primitive.ObjectID, limit int) (eventID string, err error) {
	svc := NewAIDecisionService()
	res, err := svc.EmitEvent(ctx, &EmitEventInput{
		EventType:   EventTypeAdsIntelligenceRecalculateAllRequested,
		EventSource: eventtypes.EventSourceMetaAPI,
		EntityType:  "organization",
		EntityID:    ownerOrgID.Hex(),
		OrgID:       ownerOrgID.Hex(),
		OwnerOrgID:  ownerOrgID,
		Priority:    "normal",
		Lane:        aidecisionmodels.EventLaneBatch,
		Payload: map[string]interface{}{
			"ownerOrgIdHex": ownerOrgID.Hex(),
			"limit":         limit,
		},
	})
	if err != nil {
		return "", err
	}
	return res.EventID, nil
}
