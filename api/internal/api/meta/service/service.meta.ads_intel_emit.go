// Package metasvc — Sau khi worker ads_intel_compute tính xong Intelligence (recompute_one),
// emit campaign_intel_recomputed (EventSource meta_ads_intel) để AI Decision chạy ProcessMetaCampaignDataChanged.
// Luồng mới: hook L2 chỉ insight (filter) → debounce recompute → job này → emit campaign_intel_recomputed → ads.context_*.
package metasvc

import (
	"context"
	"os"
	"strings"

	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/api/aidecision/eventemit"
	"meta_commerce/internal/api/aidecision/eventtypes"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// envKeyEmitMetaCampaignAfterIntel — "false" tắt emit campaign sau recompute (mặc định bật).
const envKeyEmitMetaCampaignAfterIntel = "AI_DECISION_WORKER_EMIT_META_CAMPAIGN_AFTER_INTEL_RECOMPUTE"

func envEmitMetaCampaignAfterIntelEnabled() bool {
	s := strings.TrimSpace(strings.ToLower(os.Getenv(envKeyEmitMetaCampaignAfterIntel)))
	if s == "" || s == "1" || s == "true" || s == "yes" {
		return true
	}
	return s != "false" && s != "0" && s != "no"
}

// emitCampaignIntelRecomputedAfterRecomputeJob — nối recompute → bước 3: ghi campaign_intel_recomputed (meta_ads_intel)
// để consumer gọi ProcessMetaCampaignDataChanged → ads.context_requested.
func emitCampaignIntelRecomputedAfterRecomputeJob(ctx context.Context, job *adsmodels.AdsIntelComputeJob) error {
	if !envEmitMetaCampaignAfterIntelEnabled() {
		return nil
	}
	if job == nil || job.OwnerOrganizationID.IsZero() {
		return nil
	}
	campaignID, adAccountID, ok := resolveCampaignIDAfterRecomputeJob(ctx, job)
	if !ok || campaignID == "" || adAccountID == "" {
		return nil
	}
	orgID := job.OwnerOrganizationID.Hex()
	traceID := utility.GenerateUID(utility.UIDPrefixTrace)
	correlationID := utility.GenerateUID(utility.UIDPrefixCorrelation)
	_, err := eventemit.EmitDecisionEvent(ctx, &eventemit.EmitInput{
		EventType:     eventtypes.CampaignIntelRecomputed,
		EventSource:   eventtypes.EventSourceMetaAdsIntel,
		EntityType:    "campaign",
		EntityID:      campaignID,
		OrgID:         orgID,
		OwnerOrgID:    job.OwnerOrganizationID,
		Priority:      "normal",
		Lane:          "batch",
		TraceID:       traceID,
		CorrelationID: correlationID,
		Payload: map[string]interface{}{
			"campaignId":                campaignID,
			"adAccountId":               adAccountID,
			"triggerFromIntelRecompute": true,
		},
	})
	return err
}

func resolveCampaignIDAfterRecomputeJob(ctx context.Context, job *adsmodels.AdsIntelComputeJob) (campaignID, adAccountID string, ok bool) {
	ot := strings.ToLower(strings.TrimSpace(job.ObjectType))
	oid := job.OwnerOrganizationID
	adAcc := strings.TrimSpace(job.AdAccountID)
	obj := strings.TrimSpace(job.ObjectID)
	if oid.IsZero() {
		return "", "", false
	}
	switch ot {
	case "campaign":
		if obj == "" || adAcc == "" {
			return "", "", false
		}
		return obj, adAcc, true
	case "ad":
		return lookupCampaignFromAdID(ctx, obj, oid)
	case "adset":
		return lookupCampaignFromAdSetID(ctx, obj, oid)
	case "ad_account":
		// Một account có nhiều campaign — không chọn một campaign đại diện.
		return "", "", false
	default:
		return "", "", false
	}
}

func lookupCampaignFromAdID(ctx context.Context, adID string, ownerOrgID primitive.ObjectID) (campaignID, adAccountID string, ok bool) {
	adID = strings.TrimSpace(adID)
	if adID == "" || ownerOrgID.IsZero() {
		return "", "", false
	}
	coll, okc := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !okc {
		return "", "", false
	}
	filter := bson.M{"adId": adID, "ownerOrganizationId": ownerOrgID}
	var doc struct {
		CampaignId  string `bson:"campaignId"`
		AdAccountId string `bson:"adAccountId"`
	}
	if err := coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		return "", "", false
	}
	if strings.TrimSpace(doc.CampaignId) == "" || strings.TrimSpace(doc.AdAccountId) == "" {
		return "", "", false
	}
	return doc.CampaignId, doc.AdAccountId, true
}

func lookupCampaignFromAdSetID(ctx context.Context, adSetID string, ownerOrgID primitive.ObjectID) (campaignID, adAccountID string, ok bool) {
	adSetID = strings.TrimSpace(adSetID)
	if adSetID == "" || ownerOrgID.IsZero() {
		return "", "", false
	}
	coll, okc := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdSets)
	if !okc {
		return "", "", false
	}
	filter := bson.M{"adSetId": adSetID, "ownerOrganizationId": ownerOrgID}
	var doc struct {
		CampaignId  string `bson:"campaignId"`
		AdAccountId string `bson:"adAccountId"`
	}
	if err := coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		return "", "", false
	}
	if strings.TrimSpace(doc.CampaignId) == "" || strings.TrimSpace(doc.AdAccountId) == "" {
		return "", "", false
	}
	return doc.CampaignId, doc.AdAccountId, true
}
