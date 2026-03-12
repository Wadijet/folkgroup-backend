// Package metasvc - FLAG_RULE: đánh giá metrics → tạo flag (sl_a, chs_critical, ...).
// Khác với ACTION_RULE (ads/rules): flag → action (PAUSE, DECREASE).
// Theo FolkForm v4.1 (WF-03). Lưu vào currentMetrics.alertFlags.
// Đọc ngưỡng từ ads/config khi cfg != nil.
// Dùng ads/rules evaluator driven bởi FlagDefinitions.
package metasvc

import (
	"context"

	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
	adsrules "meta_commerce/internal/api/ads/rules"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// computeAlertFlags tính danh sách flags cảnh báo từ metrics.
// cfg: config từ ads_meta_config (nil = dùng default). Trả về []string — các mã flag đang trigger.
// Dùng rules.EvaluateFlags với FlagDefinitions từ config.
// Khi campaignId != "": dùng Per-Camp Adaptive Threshold (FolkForm v4.1 Section 2.2).
// PATCH 04: DetectWindowShoppingPattern → thêm window_shopping_pattern để suspend Mess Trap đến 14:00.
func computeAlertFlags(ctx context.Context, raw, layer1, layer2, layer3 map[string]interface{}, cfg *adsmodels.CampaignConfigView, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) []string {
	factsCtx := adsrules.BuildFactsContext(raw, layer1, layer2, layer3, cfg)
	var campCtx *adsrules.EvalCampaignContext
	if campaignId != "" && adAccountId != "" {
		campCtx = &adsrules.EvalCampaignContext{
			CampaignId:  campaignId,
			AdAccountId: adAccountId,
			OwnerOrgID:  ownerOrgID,
		}
	}
	flags := adsrules.EvaluateFlags(ctx, &factsCtx, cfg, campCtx)
	if DetectWindowShoppingPattern(ctx, campaignId, adAccountId, ownerOrgID) {
		flags = append(flags, "window_shopping_pattern")
	}
	return flags
}

// computeSuggestedActions tính action đề xuất cho tình trạng hiện tại từ alertFlags.
// Ưu tiên: Kill → Decrease → Increase (theo FolkForm WF-03, WF-04).
// raw, layer1: optional — dùng cho PATCH 04 Safety guard (Msg_Rate < 1%, CPM < 40k vẫn kill dù window_shopping_pattern).
func computeSuggestedActions(ctx context.Context, alertFlags []string, adAccountId string, ownerOrgID primitive.ObjectID, cfg *adsmodels.CampaignConfigView, raw, layer1 map[string]interface{}) []map[string]interface{} {
	if len(alertFlags) == 0 {
		return nil
	}
	flags := make([]interface{}, len(alertFlags))
	for i, f := range alertFlags {
		flags[i] = f
	}
	killEnabled := adsconfig.GetKillRulesEnabled(ctx, adAccountId, ownerOrgID)
	opts := &adsrules.EvalOptions{KillRulesEnabled: killEnabled}
	if layer1 != nil {
		opts.MsgRateRatio = toFloat(layer1, "msgRate_7d")
	}
	if raw != nil {
		meta, _ := raw["meta"].(map[string]interface{})
		if meta != nil {
			opts.CpmVnd = toFloat(meta, "cpm")
		}
	}

	var result *adsrules.RuleResult
	result = adsrules.EvaluateAlertFlagsWithConfig(flags, opts, cfg)
	if result == nil {
		result = adsrules.EvaluateForDecreaseWithConfig(flags, cfg)
	}
	if result == nil {
		result = adsrules.EvaluateForIncrease(flags, cfg)
	}
	if result == nil || !result.ShouldPropose {
		return nil
	}

	label := result.Label
	if label == "" {
		label = result.RuleCode
	}
	action := map[string]interface{}{
		"actionType": result.ActionType,
		"ruleCode":   result.RuleCode,
		"reason":     result.Reason,
		"label":      label,
	}
	if result.Value != nil {
		action["value"] = result.Value
	}
	return []map[string]interface{}{action}
}
