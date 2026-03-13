// Package metasvc - FLAG_RULE: đánh giá metrics → tạo flag (sl_a, chs_critical, ...).
// Khác với ACTION_RULE (ads/rules): flag → action (PAUSE, DECREASE).
// Theo FolkForm v4.1 (WF-03). Lưu vào currentMetrics.alertFlags.
// Đọc ngưỡng từ ads/config khi cfg != nil.
// Dùng ads/rules evaluator driven bởi FlagDefinitions.
package metasvc

import (
	"context"
	"fmt"
	"time"

	adsadaptive "meta_commerce/internal/api/ads/adaptive"
	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
	adsrules "meta_commerce/internal/api/ads/rules"
	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	ruleintelsvc "meta_commerce/internal/api/ruleintel/service"

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

// hasFlag kiểm tra flag có trong danh sách không.
func hasFlag(flags []string, code string) bool {
	for _, f := range flags {
		if f == code {
			return true
		}
	}
	return false
}

// ruleMatchesFromFlags kiểm tra rule có match flags không (single Flag hoặc RequireFlags).
func ruleMatchesFromFlags(alertFlags []string, singleFlag string, requireFlags []string) bool {
	flags := make(map[string]bool)
	for _, f := range alertFlags {
		flags[f] = true
	}
	if len(requireFlags) > 0 {
		for _, rf := range requireFlags {
			if !flags[rf] {
				return false
			}
		}
		return true
	}
	return singleFlag != "" && flags[singleFlag]
}

func getKillRulesFromConfig(cfg *adsmodels.CampaignConfigView) []adsmodels.ActionRuleItem {
	if cfg != nil && len(cfg.ActionRuleConfig.KillRules) > 0 {
		return cfg.ActionRuleConfig.KillRules
	}
	return adsconfig.DefaultActionRuleConfig().KillRules
}

func getDecreaseRulesFromConfig(cfg *adsmodels.CampaignConfigView) []adsmodels.ActionRuleItem {
	if cfg != nil && len(cfg.ActionRuleConfig.DecreaseRules) > 0 {
		return cfg.ActionRuleConfig.DecreaseRules
	}
	return adsconfig.DefaultActionRuleConfig().DecreaseRules
}

func getIncreaseRulesFromConfig(cfg *adsmodels.CampaignConfigView) []adsmodels.ActionRuleItem {
	if cfg != nil && len(cfg.ActionRuleConfig.IncreaseRules) > 0 {
		return cfg.ActionRuleConfig.IncreaseRules
	}
	return adsconfig.DefaultActionRuleConfig().IncreaseRules
}

// ruleCodeToRuleID map rule_code → rule_id cho Rule Engine. Tất cả rules thuộc system (seed).
var ruleCodeToRuleID = map[string]string{
	"sl_a": "RULE_ADS_KILL_SL_A", "sl_b": "RULE_ADS_KILL_SL_B", "sl_c": "RULE_ADS_KILL_SL_C", "sl_d": "RULE_ADS_KILL_SL_D", "sl_e": "RULE_ADS_KILL_SL_E",
	"chs_critical": "RULE_ADS_KILL_CHS", "ko_a": "RULE_ADS_KILL_KO_A", "ko_b": "RULE_ADS_KILL_KO_B", "ko_c": "RULE_ADS_KILL_KO_C", "trim_eligible": "RULE_ADS_KILL_TRIM",
	"sl_a_decrease": "RULE_ADS_DECREASE_SL_A", "mess_trap_suspect": "RULE_ADS_DECREASE_MESS_TRAP", "trim_eligible_decrease": "RULE_ADS_DECREASE_TRIM", "chs_warning": "RULE_ADS_DECREASE_CHS",
	"increase_eligible": "RULE_ADS_INCREASE_ELIGIBLE", "increase_safety_net": "RULE_ADS_INCREASE_SAFETY",
}

// tryRuleEngine gọi Rule Engine cho rule. Trả về *RuleResult nếu match; nil nếu không.
func tryRuleEngine(ctx context.Context, ruleID, ruleCode string, layers map[string]interface{}, paramsOverride map[string]interface{}, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, label string) *adsrules.RuleResult {
	svc, err := ruleintelsvc.NewRuleEngineService()
	if err != nil {
		return nil
	}
	input := &ruleintelsvc.RunInput{
		RuleID:         ruleID,
		Domain:         "ads",
		EntityRef:      ruleintelmodels.EntityRef{Domain: "ads", ObjectType: "campaign", ObjectID: campaignId, OwnerOrganizationID: ownerOrgID.Hex()},
		Layers:         layers,
		ParamsOverride: paramsOverride,
	}
	result, err := svc.Run(ctx, input)
	if err != nil || result == nil || result.Result == nil {
		return nil
	}
	actionObj, ok := result.Result.(map[string]interface{})
	if !ok {
		return nil
	}
	actionCode, _ := actionObj["action_code"].(string)
	reason, _ := actionObj["reason"].(string)
	val := actionObj["value"]
	if actionCode == "" {
		return nil
	}
	if label == "" {
		label = ruleCode
	}
	return &adsrules.RuleResult{
		ShouldPropose: true,
		ActionType:    actionCode,
		Reason:        reason,
		RuleCode:      ruleCode,
		Label:         label,
		Value:         val,
	}
}

// computeSuggestedActions tính action đề xuất cho tình trạng hiện tại từ alertFlags.
// Ưu tiên: Kill → Decrease → Increase (theo FolkForm WF-03, WF-04).
// Dùng Rule Engine cho tất cả rules (Owner=system). Fallback sang adsrules nếu Rule Engine lỗi.
func computeSuggestedActions(ctx context.Context, alertFlags []string, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, cfg *adsmodels.CampaignConfigView, raw, layer1 map[string]interface{}) []map[string]interface{} {
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
	metaForCpm, _ := raw["meta"].(map[string]interface{})
	if metaForCpm != nil {
		opts.CpmVnd = toFloat(metaForCpm, "cpm")
	}

	// Build layers.flag (map) cho flag-based rules
	flagMap := make(map[string]interface{})
	for _, f := range alertFlags {
		flagMap[f] = true
	}
	r7d := raw
	if r7d == nil {
		r7d = map[string]interface{}{}
	}
	layersBase := map[string]interface{}{
		"layer1": layer1,
		"raw":    r7d,
		"flag":   flagMap,
	}
	if layer1 == nil {
		layersBase["layer1"] = map[string]interface{}{}
	}

	now := time.Now()
	exceptionKill := adsconfig.GetExceptionFlagsForKill(cfg)
	exceptionDecrease := adsconfig.GetExceptionFlagsForDecrease(cfg)
	windowShopping := hasFlag(alertFlags, "window_shopping_pattern")
	before1400 := adsconfig.IsBefore1400Vietnam(now)

	var result *adsrules.RuleResult

	// Rule Engine: Kill rules (theo thứ tự config)
	killRules := getKillRulesFromConfig(cfg)
	for _, r := range killRules {
		ruleCode := r.RuleCode
		if ruleCode == "" {
			ruleCode = r.Flag
		}
		if !ruleMatchesFromFlags(alertFlags, r.Flag, r.RequireFlags) {
			continue
		}
		ruleID := ruleCodeToRuleID[ruleCode]
		if ruleID == "" {
			continue
		}
		params := map[string]interface{}{
			"exceptionFlags":   exceptionKill,
			"killRulesEnabled": killEnabled,
			"freeze":           r.Freeze,
		}
		if ruleCode == "mess_trap_suspect" || ruleCode == "mess_trap_confirmed" {
			params["skipMessTrapWindowShopping"] = true
			params["windowShoppingPattern"] = windowShopping
			params["isBefore1400"] = before1400
			params["msgRateRatio"] = opts.MsgRateRatio
			params["cpmVnd"] = opts.CpmVnd
		}
		if ruleCode == "sl_a" {
			if campaignId != "" && adAccountId != "" {
				if th, ok := adsadaptive.GetAdaptiveThreshold(ctx, adsconfig.KeyCpaMessKill, campaignId, adAccountId, ownerOrgID, cfg, now); ok {
					params["th_cpaMessKill"] = th
				}
			}
			if _, has := params["th_cpaMessKill"]; !has {
				params["th_cpaMessKill"] = adsconfig.GetThresholdWithEventOverride(adsconfig.KeyCpaMessKill, cfg, now)
			}
		}
		if re := tryRuleEngine(ctx, ruleID, ruleCode, layersBase, params, campaignId, adAccountId, ownerOrgID, r.Label); re != nil {
			result = re
			break
		}
	}

	// Rule Engine: Decrease rules
	if result == nil && cfg != nil && cfg.AutomationConfig.EffectiveBudgetRulesEnabled() {
		decreaseRules := getDecreaseRulesFromConfig(cfg)
		for _, r := range decreaseRules {
			ruleCode := r.RuleCode
			if ruleCode == "" {
				ruleCode = r.Flag
			}
			if !ruleMatchesFromFlags(alertFlags, r.Flag, r.RequireFlags) {
				continue
			}
			ruleID := ruleCodeToRuleID[ruleCode]
			if ruleID == "" {
				continue
			}
			params := map[string]interface{}{
				"exceptionFlags": exceptionDecrease,
			}
			if re := tryRuleEngine(ctx, ruleID, ruleCode, layersBase, params, campaignId, adAccountId, ownerOrgID, r.Label); re != nil {
				result = re
				break
			}
		}
	}

	// Rule Engine: Increase rules
	if result == nil && cfg != nil && cfg.AutomationConfig.EffectiveBudgetRulesEnabled() {
		increaseRules := getIncreaseRulesFromConfig(cfg)
		for _, r := range increaseRules {
			ruleCode := r.RuleCode
			if ruleCode == "" {
				ruleCode = r.Flag
			}
			if !ruleMatchesFromFlags(alertFlags, r.Flag, r.RequireFlags) {
				continue
			}
			ruleID := ruleCodeToRuleID[ruleCode]
			if ruleID == "" {
				continue
			}
			params := map[string]interface{}{}
			if re := tryRuleEngine(ctx, ruleID, ruleCode, layersBase, params, campaignId, adAccountId, ownerOrgID, r.Label); re != nil {
				result = re
				break
			}
		}
	}

	// Fallback sang adsrules nếu Rule Engine không trả kết quả
	if result == nil {
		result = adsrules.EvaluateAlertFlagsWithConfig(flags, opts, cfg)
	}
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

// ComputeFinalActions tính action cuối cùng sau TẤT CẢ filter (FolkForm v4.1).
// Trả về (actions, actionDebugReport). Report dùng cho debug và kiểm tra lại quá trình tạo đề xuất.
// Gọi từ ads module (worker) — server rollup chỉ tạo flags, không tạo actions.
func ComputeFinalActions(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, alertFlags []string, raw map[string]interface{}, layer1, layer2, layer3 map[string]interface{}, cfg *adsmodels.CampaignConfigView) (actions []map[string]interface{}, report map[string]interface{}) {
	report = buildActionDebugReport(ctx, campaignId, adAccountId, ownerOrgID, alertFlags, raw, layer1, layer2, layer3, cfg)
	return report["finalActions"].([]map[string]interface{}), report
}

// MinCampaignAgeDays số ngày tối thiểu để campaign được đề xuất (FolkForm v4.1 Section 2.2: camp mới < 7 ngày bỏ qua).
const MinCampaignAgeDays = 7

// isLifecycleNew kiểm tra campaign có phải NEW (< 7 ngày) không. Dùng lifecycle từ layer1; fallback metaCreatedAt.
func isLifecycleNew(layer1 map[string]interface{}, metaCreatedAt int64) bool {
	if layer1 != nil {
		if lc, ok := layer1["lifecycle"].(string); ok && lc == "NEW" {
			return true
		}
	}
	if metaCreatedAt > 0 {
		daysSinceCreated := (time.Now().UnixMilli() - metaCreatedAt) / (24 * 60 * 60 * 1000)
		return daysSinceCreated < MinCampaignAgeDays
	}
	return false
}

// buildActionDebugReport xây dựng báo cáo chi tiết quá trình tạo đề xuất — dữ liệu, lý do, từng bước. Dùng cho debug.
func buildActionDebugReport(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, alertFlags []string, raw map[string]interface{}, layer1, layer2, layer3 map[string]interface{}, cfg *adsmodels.CampaignConfigView) map[string]interface{} {
	now := time.Now()
	r7d := getRaw7d(raw)
	steps := []map[string]interface{}{}

	// Input summary (raw, layer1, layer2, layer3 + orders_2h cho dual-source)
	inputSummary := buildInputSummary(raw, r7d, layer1, layer2, layer3)
	killEnabled := adsconfig.GetKillRulesEnabled(ctx, adAccountId, ownerOrgID)
	noonCut := adsconfig.IsNoonCutWindow(now)
	metaCreatedAt := toInt64(r7d, "metaCreatedAt")
	lifecycle := safeGet(layer1, "lifecycle")

	// Bước 0: Lifecycle filter — Campaign NEW (< 7 ngày) không đề xuất (FolkForm v4.1 Section 2.2)
	isNew := isLifecycleNew(layer1, metaCreatedAt)
	step0 := map[string]interface{}{
		"step":   0,
		"name":   "lifecycle_filter",
		"label":  "Campaign NEW (< 7 ngày) không đề xuất — FolkForm v4.1 Per-Camp Adaptive giai đoạn 0",
		"input":  map[string]interface{}{"lifecycle": lifecycle, "metaCreatedAt": metaCreatedAt},
		"result": map[bool]string{true: "filtered", false: "passed"}[isNew],
	}
	if isNew {
		step0["reason"] = "Campaign NEW (< 7 ngày) — chưa đủ data, không đề xuất"
		steps = append(steps, step0)
		return map[string]interface{}{
			"computedAt":   now.Format(time.RFC3339),
			"alertFlags":   alertFlags,
			"inputSummary": inputSummary,
			"steps":        steps,
			"finalActions": []map[string]interface{}{},
			"finalReason":  step0["reason"],
		}
	}
	step0["reason"] = "Lifecycle=" + fmt.Sprint(lifecycle) + " — đủ điều kiện đề xuất"
	steps = append(steps, step0)

	// Bước 1: Flag computation — metrics → alertFlags (EvaluateFlags + DetectWindowShoppingPattern)
	step1 := map[string]interface{}{
		"step":   1,
		"name":   "flag_computation",
		"label":  "Đánh giá metrics → alert flags (EvaluateFlags, window_shopping_pattern)",
		"input":  inputSummary,
		"output": alertFlags,
		"result": "computed",
	}
	if len(alertFlags) == 0 {
		step1["reason"] = "Không có flag nào trigger"
	} else {
		step1["reason"] = fmt.Sprintf("%d flags: %v", len(alertFlags), alertFlags)
	}
	steps = append(steps, step1)

	// Bước 2: Rule evaluation (Kill → Decrease → Increase)
	suggested := computeSuggestedActions(ctx, alertFlags, campaignId, adAccountId, ownerOrgID, cfg, r7d, layer1)
	metaForInput, _ := r7d["meta"].(map[string]interface{})
	step2 := map[string]interface{}{
		"step":   2,
		"name":   "rule_evaluation",
		"label":  "Đánh giá rules (Kill → Decrease → Increase)",
		"input": map[string]interface{}{
			"alertFlags":   alertFlags,
			"killEnabled":  killEnabled,
			"msgRateRatio": toFloat(layer1, "msgRate_7d"),
			"cpmVnd":       toFloat(metaForInput, "cpm"),
		},
	}
	if len(suggested) == 0 {
		step2["result"] = "no_match"
		step2["reason"] = "Không có rule nào match (hoặc alertFlags rỗng)"
		steps = append(steps, step2)
		return map[string]interface{}{
			"computedAt":     now.Format(time.RFC3339),
			"alertFlags":     alertFlags,
			"inputSummary":   inputSummary,
			"steps":          steps,
			"finalActions":   []map[string]interface{}{},
			"finalReason":    "Không có suggested action từ rule evaluation",
		}
	}
	action := suggested[0]
	actionType, _ := action["actionType"].(string)
	ruleCode, _ := action["ruleCode"].(string)
	step2["result"] = "match"
	step2["matchedRule"] = map[string]interface{}{"ruleCode": ruleCode, "actionType": actionType, "reason": action["reason"]}
	steps = append(steps, step2)

	// Bước 3: ShouldAutoPropose
	shouldPropose := adsconfig.GetShouldAutoPropose(ruleCode, cfg)
	step3 := map[string]interface{}{
		"step": 3, "name": "should_auto_propose", "label": "Rule có bật auto propose không",
		"ruleCode": ruleCode, "result": map[bool]string{true: "passed", false: "filtered"}[shouldPropose],
	}
	if !shouldPropose {
		step3["reason"] = "Rule " + ruleCode + " có autoPropose=false trong config"
		steps = append(steps, step3)
		return map[string]interface{}{
			"computedAt": now.Format(time.RFC3339), "alertFlags": alertFlags, "inputSummary": inputSummary,
			"steps": steps, "finalActions": []map[string]interface{}{}, "finalReason": step3["reason"],
		}
	}
	step3["reason"] = "Rule " + ruleCode + " có autoPropose=true"
	steps = append(steps, step3)

	// Bước 4: Noon cut (chỉ INCREASE)
	if actionType == "INCREASE" {
		step4 := map[string]interface{}{
			"step": 4, "name": "noon_cut", "label": "Cửa sổ Noon Cut (12:00–14:30)",
			"isNoonCut": noonCut, "result": map[bool]string{true: "filtered", false: "passed"}[noonCut],
		}
		if noonCut {
			step4["reason"] = "INCREASE không chạy trong cửa sổ noon cut"
			steps = append(steps, step4)
			return map[string]interface{}{
				"computedAt": now.Format(time.RFC3339), "alertFlags": alertFlags, "inputSummary": inputSummary,
				"steps": steps, "finalActions": []map[string]interface{}{}, "finalReason": step4["reason"],
			}
		}
		step4["reason"] = "Không trong cửa sổ noon cut"
		steps = append(steps, step4)
	} else {
		step4 := map[string]interface{}{
			"step": 4, "name": "noon_cut", "label": "Cửa sổ Noon Cut (12:00–14:30)",
			"result": "skipped", "reason": "Chỉ áp dụng cho INCREASE (actionType=" + actionType + ")",
		}
		steps = append(steps, step4)
	}

	// Bước 5: Dual-source confirm (chỉ Kill rules)
	if (actionType == "PAUSE" || actionType == "KILL") && adsconfig.RuleRequiresDualSourceConfirm(ruleCode) {
		orders2h := getPancakeOrders2hFromRaw(raw)
		fbPurchases, fbOk := GetFBPurchasesForCampaign(ctx, campaignId, adAccountId, ownerOrgID)
		step5 := map[string]interface{}{
			"step": 5, "name": "dual_source_confirm", "label": "Xác nhận dual-source (Pancake + FB)",
			"data": map[string]interface{}{"pancakeOrders2h": orders2h, "fbPurchases": fbPurchases, "fbOk": fbOk},
		}
		if orders2h > 0 {
			step5["result"] = "filtered"
			step5["reason"] = "Pancake có đơn 2h → attribution gap, chờ checkpoint"
			steps = append(steps, step5)
			return map[string]interface{}{
				"computedAt": now.Format(time.RFC3339), "alertFlags": alertFlags, "inputSummary": inputSummary,
				"steps": steps, "finalActions": []map[string]interface{}{}, "finalReason": step5["reason"],
			}
		}
		if fbOk && fbPurchases > 0 {
			step5["result"] = "filtered"
			step5["reason"] = "FB có purchase, Pancake chưa → attribution gap, chờ checkpoint"
			steps = append(steps, step5)
			return map[string]interface{}{
				"computedAt": now.Format(time.RFC3339), "alertFlags": alertFlags, "inputSummary": inputSummary,
				"steps": steps, "finalActions": []map[string]interface{}{}, "finalReason": step5["reason"],
			}
		}
		step5["result"] = "passed"
		step5["reason"] = "Cả 2 nguồn xấu (Pancake=0, FB=0) → confirm kill"
		steps = append(steps, step5)
	} else {
		reason := "Chỉ áp dụng cho Kill/PAUSE rules có dual-source"
		if actionType != "PAUSE" && actionType != "KILL" {
			reason = "Chỉ áp dụng cho Kill/PAUSE (actionType=" + actionType + ")"
		} else if !adsconfig.RuleRequiresDualSourceConfirm(ruleCode) {
			reason = "Rule " + ruleCode + " không yêu cầu dual-source confirm"
		}
		step5 := map[string]interface{}{
			"step": 5, "name": "dual_source_confirm", "label": "Xác nhận dual-source (Pancake + FB)",
			"result": "skipped", "reason": reason,
		}
		steps = append(steps, step5)
	}

	// Bước 6: CHS exception (chỉ chs_critical)
	if ruleCode == "chs_critical" {
		chsYesterday, chsOk := GetChsFromYesterday(ctx, campaignId, ownerOrgID)
		step6 := map[string]interface{}{
			"step": 6, "name": "chs_exception", "label": "CHS hôm qua HEALTHY (>=60) → chờ checkpoint",
			"data": map[string]interface{}{"chsYesterday": chsYesterday, "chsOk": chsOk},
		}
		if chsOk && chsYesterday >= 60 {
			step6["result"] = "filtered"
			step6["reason"] = "Camp HEALTHY hôm qua (CHS=" + fmt.Sprintf("%.1f", chsYesterday) + ") → có thể data anomaly, chờ 1 checkpoint"
			steps = append(steps, step6)
			return map[string]interface{}{
				"computedAt": now.Format(time.RFC3339), "alertFlags": alertFlags, "inputSummary": inputSummary,
				"steps": steps, "finalActions": []map[string]interface{}{}, "finalReason": step6["reason"],
			}
		}
		step6["result"] = "passed"
		step6["reason"] = "Không có CHS yesterday hoặc CHS < 60"
		steps = append(steps, step6)
	} else {
		step6 := map[string]interface{}{
			"step": 6, "name": "chs_exception", "label": "CHS hôm qua HEALTHY (>=60) → chờ checkpoint",
			"result": "skipped", "reason": "Chỉ áp dụng cho chs_critical (ruleCode=" + ruleCode + ")",
		}
		steps = append(steps, step6)
	}

	return map[string]interface{}{
		"computedAt":   now.Format(time.RFC3339),
		"alertFlags":   alertFlags,
		"inputSummary": inputSummary,
		"steps":        steps,
		"finalActions": suggested,
		"finalReason":  "Tất cả filter passed",
	}
}

// buildInputSummary tóm tắt dữ liệu đầu vào cho debug. raw dùng để lấy orders_2h (dual-source).
func buildInputSummary(raw map[string]interface{}, r7d map[string]interface{}, layer1, layer2, layer3 map[string]interface{}) map[string]interface{} {
	meta, _ := r7d["meta"].(map[string]interface{})
	pancake, _ := r7d["pancake"].(map[string]interface{})
	pos, _ := mapOrNil(pancake, "pos").(map[string]interface{})
	orders2h := float64(0)
	if raw != nil {
		orders2h = getPancakeOrders2hFromRaw(raw)
	}
	sum := map[string]interface{}{
		"raw": map[string]interface{}{
			"spend": toFloat(meta, "spend"), "mess": toInt64(meta, "mess"), "inlineLinkClicks": toInt64(meta, "inlineLinkClicks"),
			"cpm": toFloat(meta, "cpm"), "ctr": toFloat(meta, "ctr"), "frequency": toFloat(meta, "frequency"),
			"orders": toInt64(pos, "orders"), "revenue": toFloat(pos, "revenue"),
			"orders_2h": orders2h, // Pancake orders 2h — cho dual-source confirm
		},
		"layer1": map[string]interface{}{
			"lifecycle": safeGet(layer1, "lifecycle"), "cpaMess_7d": toFloat(layer1, "cpaMess_7d"), "convRate_7d": toFloat(layer1, "convRate_7d"),
			"msgRate_7d": toFloat(layer1, "msgRate_7d"), "mqs_7d": toFloat(layer1, "mqs_7d"), "spendPct": toFloat(layer1, "spendPct"),
		},
		"layer2": map[string]interface{}{"currentMode": safeGet(layer2, "currentMode")},
		"layer3": map[string]interface{}{"chs": toFloat(layer3, "chs")},
	}
	return sum
}

func safeGet(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	return m[key]
}


// getPancakeOrders2hFromRaw trích orders 2h từ raw (raw.2h.orders hoặc raw.7d.pancake cho 2h).
func getPancakeOrders2hFromRaw(raw map[string]interface{}) float64 {
	r2h := getRaw2h(raw)
	if r2h != nil {
		return toFloat(r2h, "orders")
	}
	return 0
}
