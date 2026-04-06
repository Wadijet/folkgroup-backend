// Package adsautop — Pipeline tự động đề xuất (auto-propose) do AI Decision điều phối:
// emit ads.propose_requested; ACTION_RULE qua metasvc.ComputeFinalActionsFromCurrentMetrics (Intelligence chỉ alertFlags).
package adsautop

import (
	"context"
	"fmt"
	"sort"

	adsmodels "meta_commerce/internal/api/ads_meta/models"
	adssvc "meta_commerce/internal/api/ads_meta/service"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	"meta_commerce/internal/approval"
	"meta_commerce/internal/global"
	metasvc "meta_commerce/internal/api/meta/service"
)

// increaseCandidate ứng viên tăng budget — dùng cho Anti Self-Competition (FolkForm v4.1 Section 06).
type increaseCandidate struct {
	c              adssvc.CampaignForEval
	action         map[string]interface{}
	actions        []map[string]interface{}
	report         map[string]interface{}
	currentMetrics map[string]interface{}
	metaCfg        *adsmodels.CampaignConfigView
	cpaPurchase    float64
	chs            float64
	mqs            float64
	frequency      float64
}

// RunAutoPropose đánh giá campaigns và enqueue đề xuất (ads.propose_requested) khi rule trigger.
// Dùng metasvc.ComputeFinalActionsFromCurrentMetrics (ACTION_RULE); persist qua metasvc.PersistCampaignEvaluatedActions.
// Anti Self-Competition: Increase chỉ cho top N camp/account (PROTECT:1, EFFICIENCY:1, NORMAL:2, BLITZ:3).
func RunAutoPropose(ctx context.Context, baseURL string) (proposed int, err error) {
	campaigns, err := adssvc.GetCampaignsForAutoPropose(ctx, 30)
	if err != nil {
		return 0, err
	}
	campaignsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy collection meta_campaigns")
	}
	campaignConfigByAccount := make(map[string]*adsmodels.CampaignConfigView)
	increaseCandidatesByAccount := make(map[string][]increaseCandidate)

	for _, c := range campaigns {
		cacheKey := c.AdAccountId + "|" + c.OwnerOrganizationID.Hex()
		if _, ok := campaignConfigByAccount[cacheKey]; !ok {
			cfg, _ := adssvc.GetCampaignConfig(ctx, c.AdAccountId, c.OwnerOrganizationID)
			campaignConfigByAccount[cacheKey] = cfg
		}
		metaCfg := campaignConfigByAccount[cacheKey]

		pendingInfo, pendingErr := adssvc.GetPendingProposalForCampaign(ctx, c.CampaignId, c.OwnerOrganizationID)
		if pendingErr != nil {
			continue
		}
		hasPending := pendingInfo != nil

		// Có pending: enqueue tính lại campaign qua AI Decision (không gọi metasvc.RecalculateForEntity trực tiếp).
		// Consumer sẽ cập nhật currentMetrics; vòng này vẫn đọc DB hiện tại (có thể chưa kịp fresh nếu consumer chưa xử lý).
		if hasPending {
			if _, emitErr := aidecisionsvc.EmitAdsIntelligenceRecomputeRequested(ctx, "campaign", c.CampaignId, c.AdAccountId, c.OwnerOrganizationID, "meta", metasvc.RecomputeModeFull); emitErr != nil {
				continue
			}
		}

		currentMetrics := adssvc.GetCampaignCurrentMetrics(ctx, campaignsColl, c.CampaignId, c.OwnerOrganizationID)
		if currentMetrics == nil {
			if hasPending {
				_, _ = approval.Cancel(ctx, pendingInfo.ID.Hex(), c.OwnerOrganizationID)
			}
			continue
		}

		actions, report := metasvc.ComputeFinalActionsFromCurrentMetrics(ctx, c.CampaignId, c.AdAccountId, c.OwnerOrganizationID, currentMetrics)

		if len(actions) == 0 {
			if hasPending {
				_, _ = approval.Cancel(ctx, pendingInfo.ID.Hex(), c.OwnerOrganizationID)
			}
			continue
		}
		action := actions[0]
		actionType, _ := action["actionType"].(string)
		ruleCode, _ := action["ruleCode"].(string)

		if actionType == "INCREASE" {
			if adssvc.IsSelfCompetitionSuspect(ctx, c.AdAccountId, c.OwnerOrganizationID) {
				continue
			}
			layer1, _ := currentMetrics["layer1"].(map[string]interface{})
			layer3, _ := currentMetrics["layer3"].(map[string]interface{})
			raw, _ := currentMetrics["raw"].(map[string]interface{})
			r7d, _ := raw["7d"].(map[string]interface{})
			meta7d, _ := r7d["meta"].(map[string]interface{})
			orders, spend := 0.0, 0.0
			if p, _ := r7d["pancake"].(map[string]interface{}); p != nil {
				if pos, _ := p["pos"].(map[string]interface{}); pos != nil {
					orders = adssvc.ToFloat64FromInterface(pos["orders"])
				}
			}
			if meta7d != nil {
				spend = adssvc.ToFloat64FromInterface(meta7d["spend"])
			}
			cpaPurchase := 0.0
			if orders > 0 {
				cpaPurchase = spend / orders
			}
			cand := increaseCandidate{
				c: c, action: action, actions: actions, report: report,
				currentMetrics: currentMetrics, metaCfg: metaCfg,
				cpaPurchase: cpaPurchase,
				chs:         adssvc.ToFloat64FromInterface(layer3["chs"]),
				mqs:         adssvc.ToFloat64FromInterface(layer1["mqs_7d"]),
				frequency:   adssvc.ToFloat64FromInterface(meta7d["frequency"]),
			}
			increaseCandidatesByAccount[cacheKey] = append(increaseCandidatesByAccount[cacheKey], cand)
			continue
		}

		if hasPending && pendingInfo.ActionType == actionType && pendingInfo.RuleCode == ruleCode {
			continue
		}
		if hasPending {
			_, _ = approval.Cancel(ctx, pendingInfo.ID.Hex(), c.OwnerOrganizationID)
		}
		if err := metasvc.PersistCampaignEvaluatedActions(ctx, c.CampaignId, c.AdAccountId, c.OwnerOrganizationID, currentMetrics, actions, report); err != nil {
			continue
		}
		metricsPayload := adssvc.BuildMetricsPayloadForNotification(ctx, campaignsColl, c.CampaignId, c.AdAccountId, c.OwnerOrganizationID, currentMetrics)
		if metricsPayload == nil {
			metricsPayload = make(map[string]interface{})
		}
		if rc := action["result_check"]; rc != nil {
			metricsPayload["result_check"] = rc
		}
		if traceId, _ := action["traceId"].(string); traceId != "" {
			metricsPayload["traceId"] = traceId
		}
		reason, _ := action["reason"].(string)
		value := action["value"]
		traceID, _ := action["traceId"].(string)
		inp, err := adssvc.BuildApprovalProposeInput(ctx, &adssvc.ProposeInput{
			ActionType:   actionType,
			AdAccountId:  c.AdAccountId,
			CampaignId:   c.CampaignId,
			CampaignName: c.CampaignName,
			Reason:       reason,
			Value:        value,
			RuleCode:     ruleCode,
			TraceID:      traceID,
			Payload:      metricsPayload,
		}, c.OwnerOrganizationID, true)
		if err != nil {
			return proposed, fmt.Errorf("chuẩn bị propose campaign %s: %w", c.CampaignId, err)
		}
		if _, err := aidecisionsvc.EmitAdsProposeRequest(ctx, inp, c.OwnerOrganizationID, baseURL); err != nil {
			return proposed, fmt.Errorf("emit propose campaign %s: %w", c.CampaignId, err)
		}
		proposed++
	}

	for cacheKey, candidates := range increaseCandidatesByAccount {
		metaCfg := campaignConfigByAccount[cacheKey]
		mode := adssvc.ModeNORMAL
		if metaCfg != nil && metaCfg.AccountMode != "" {
			mode = metaCfg.AccountMode
		}
		limit := adssvc.GetIncreaseLimit(mode)
		if len(candidates) == 0 {
			continue
		}
		sortIncreaseCandidates(candidates)
		topN := candidates
		if len(topN) > limit {
			topN = topN[:limit]
		}
		for _, cand := range topN {
			if hasPending, _ := adssvc.HasPendingProposalForCampaign(ctx, cand.c.CampaignId, cand.c.OwnerOrganizationID); hasPending {
				continue
			}
			if err := metasvc.PersistCampaignEvaluatedActions(ctx, cand.c.CampaignId, cand.c.AdAccountId, cand.c.OwnerOrganizationID, cand.currentMetrics, cand.actions, cand.report); err != nil {
				continue
			}
			metricsPayload := adssvc.BuildMetricsPayloadForNotification(ctx, campaignsColl, cand.c.CampaignId, cand.c.AdAccountId, cand.c.OwnerOrganizationID, cand.currentMetrics)
			if metricsPayload == nil {
				metricsPayload = make(map[string]interface{})
			}
			if rc := cand.action["result_check"]; rc != nil {
				metricsPayload["result_check"] = rc
			}
			if traceId, _ := cand.action["traceId"].(string); traceId != "" {
				metricsPayload["traceId"] = traceId
			}
			reason, _ := cand.action["reason"].(string)
			ruleCode, _ := cand.action["ruleCode"].(string)
			traceID, _ := cand.action["traceId"].(string)
			inp, err := adssvc.BuildApprovalProposeInput(ctx, &adssvc.ProposeInput{
				ActionType:   "INCREASE",
				AdAccountId:  cand.c.AdAccountId,
				CampaignId:   cand.c.CampaignId,
				CampaignName: cand.c.CampaignName,
				Reason:       reason,
				Value:        cand.action["value"],
				RuleCode:     ruleCode,
				TraceID:      traceID,
				Payload:      metricsPayload,
			}, cand.c.OwnerOrganizationID, true)
			if err != nil {
				return proposed, fmt.Errorf("chuẩn bị propose campaign %s: %w", cand.c.CampaignId, err)
			}
			if _, err := aidecisionsvc.EmitAdsProposeRequest(ctx, inp, cand.c.OwnerOrganizationID, baseURL); err != nil {
				return proposed, fmt.Errorf("emit propose campaign %s: %w", cand.c.CampaignId, err)
			}
			proposed++
		}
	}
	return proposed, nil
}

func sortIncreaseCandidates(candidates []increaseCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		return increaseCandidateLess(candidates[i], candidates[j])
	})
}

func increaseCandidateLess(a, b increaseCandidate) bool {
	if a.cpaPurchase != b.cpaPurchase {
		return a.cpaPurchase < b.cpaPurchase
	}
	aHealthy := a.chs >= 60
	bHealthy := b.chs >= 60
	if aHealthy != bHealthy {
		return aHealthy
	}
	if a.mqs != b.mqs {
		return a.mqs > b.mqs
	}
	return a.frequency < b.frequency
}
