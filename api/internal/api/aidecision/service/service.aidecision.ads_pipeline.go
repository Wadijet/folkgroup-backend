// Package aidecisionsvc — Khép luồng CIO → meta_campaign → ads.context_* (AI Decision).
// ACTION_RULE / đề xuất: adsautop + metasvc; ads.context_* trong consumer AID.
// Ads Intelligence (rollup) chỉ raw/layer/alertFlags — không tính action trên pipeline Intelligence.
//
// Mỗi lần ProcessMetaCampaignDataChanged chạy: emit một job ads.context_requested → worker emit ads.context_ready → RunAdsProposeFromContextReady.
// meta_campaign.updated có thể dồn dập (sync Meta) → nhiều job queue cho cùng một case. Cơ chế cooldown (ADS_CONTEXT_REQUEST_COOLDOWN_SEC + lastAdsContextRequestedAt) giảm trùng.
package aidecisionsvc

import (
	"context"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"github.com/sirupsen/logrus"
)

// ProcessMetaCampaignDataChanged: đồng bộ campaign (CIO/CRUD) → case ads_optimization → yêu cầu context Ads.
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
			}).Debug("Luồng Ads: bỏ qua ads.context_requested — vẫn trong cooldown (giảm trùng queue khi meta_campaign.updated liên tục)")
			return nil
		}
		_, err = s.EmitEvent(ctx, &EmitEventInput{
			EventType:     "ads.context_requested",
			EventSource:   "aidecision",
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
		EventType:     "ads.context_requested",
		EventSource:   "aidecision",
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
