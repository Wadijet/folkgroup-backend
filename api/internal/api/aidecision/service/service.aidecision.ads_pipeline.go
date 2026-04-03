// Package aidecisionsvc — Luồng Ads: insight → recompute Intelligence (worker) → campaign_intel_recomputed (meta_ads_intel) → ads.context_*.
// ACTION_RULE / đề xuất: adsautop + metasvc; ads.context_* trong consumer AID.
// Ads Intelligence (rollup) chỉ raw/layer/alertFlags — không tính action trên pipeline Intelligence.
//
// ProcessMetaCampaignDataChanged: đầu vào là campaign_intel_recomputed từ worker sau khi tính Intelligence xong; hoặc legacy meta_campaign.* từ hook datachanged (nếu bật emit Meta đầy đủ).
// Mỗi lần chạy: emit ads.context_requested → enqueue ads_intel_compute (context_ready) → worker emit ads.context_ready → RunAdsProposeFromContextReady.
// campaign_intel_recomputed / meta_campaign.updated dồn dập → cooldown (ADS_CONTEXT_REQUEST_COOLDOWN_SEC + lastAdsContextRequestedAt).
package aidecisionsvc

import (
	"context"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"github.com/sirupsen/logrus"
)

// EventTypeCampaignIntelRecomputed — worker ads_intel sau recompute Intelligence; tách tên khỏi meta_campaign.* (datachanged CRUD).
const EventTypeCampaignIntelRecomputed = eventtypes.CampaignIntelRecomputed

// ProcessMetaCampaignDataChanged — bước 3 trong luồng Ads:
// Đầu vào kỳ vọng: payload có campaignId + adAccountId (từ worker meta_ads_intel sau recompute, hoặc legacy datachanged).
// - ResolveOrCreate case ads_optimization; AcquireAdsContextRequestSlot (cooldown).
// - Emit ads.context_requested (chưa phải snapshot — chỉ xếp job bước 4).
// Bỏ qua nếu adsIntelligenceRollupOnly (rollup metrics qua CRUD, không chạy lại pipeline).
func (s *AIDecisionService) ProcessMetaCampaignDataChanged(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil {
		return nil
	}
	s.HydrateDatachangedPayload(ctx, evt)
	p := evt.Payload
	if p == nil {
		return nil
	}
	// Chỉ đổi currentMetrics từ roll-up Ads Intelligence — không emit lại ads.context_requested (tránh vòng lặp với Base UpdateOne).
	if skip, _ := p["adsIntelligenceRollupOnly"].(bool); skip {
		return nil
	}
	campaignID, _ := p["campaignId"].(string)
	adAccountID, _ := p["adAccountId"].(string)
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() || campaignID == "" {
		return nil
	}

	caseDoc, _, err := s.ResolveOrCreate(ctx, &ResolveOrCreateInput{
		EventID:       evt.EventID,
		EventType:     evt.EventType,
		OrgID:         evt.OrgID,
		OwnerOrgID:    ownerOrgID,
		EntityRefs:    aidecisionmodels.DecisionCaseEntityRefs{CampaignID: campaignID},
		CaseType:      aidecisionmodels.CaseTypeAdsOptimization,
		RequiredCtx:   s.RequiredContextsForCaseTypeFromRule(ctx, ownerOrgID, aidecisionmodels.CaseTypeAdsOptimization),
		Priority:      evt.Priority,
		Urgency:       "near_realtime",
		TraceID:       evt.TraceID,
		CorrelationID: evt.CorrelationID,
	})
	if err != nil {
		return err
	}
	decisionCaseID := ""
	if caseDoc != nil {
		decisionCaseID = caseDoc.DecisionCaseID
	}

	if decisionCaseID != "" {
		rollback, allow, err := s.AcquireAdsContextRequestSlot(ctx, decisionCaseID)
		if err != nil {
			return err
		}
		if !allow {
			logrus.WithFields(logrus.Fields{
				"decisionCaseId": decisionCaseID,
				"campaignId":     campaignID,
				"cooldownSec":    AdsContextRequestThrottleCooldownSec(),
			}).Debug("Luồng Ads: bỏ qua ads.context_requested — vẫn trong cooldown (giảm trùng queue khi campaign_intel_recomputed / meta_campaign.updated liên tục)")
			return nil
		}
		_, err = s.EmitEvent(ctx, &EmitEventInput{
			EventType:     eventtypes.AdsContextRequested,
			EventSource:   EventSourceAIDecision,
			EntityType:    "campaign",
			EntityID:      campaignID,
			OrgID:         evt.OrgID,
			OwnerOrgID:    ownerOrgID,
			Priority:      "normal",
			Lane:          aidecisionmodels.EventLaneBatch,
			TraceID:       evt.TraceID,
			CorrelationID: evt.CorrelationID,
			Payload: map[string]interface{}{
				"campaignId":       campaignID,
				"adAccountId":      adAccountID,
				"decisionCaseId":   decisionCaseID,
				"triggerEventType": evt.EventType,
			},
		})
		if err != nil {
			rollback()
			return err
		}
		return nil
	}

	_, err = s.EmitEvent(ctx, &EmitEventInput{
		EventType:     eventtypes.AdsContextRequested,
		EventSource:   EventSourceAIDecision,
		EntityType:    "campaign",
		EntityID:      campaignID,
		OrgID:         evt.OrgID,
		OwnerOrgID:    ownerOrgID,
		Priority:      "normal",
		Lane:          aidecisionmodels.EventLaneBatch,
		TraceID:       evt.TraceID,
		CorrelationID: evt.CorrelationID,
		Payload: map[string]interface{}{
			"campaignId":       campaignID,
			"adAccountId":      adAccountID,
			"decisionCaseId":   decisionCaseID,
			"triggerEventType": evt.EventType,
		},
	})
	return err
}
