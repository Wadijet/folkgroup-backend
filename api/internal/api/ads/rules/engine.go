// Package rules — ACTION_RULE: map flag (từ FLAG_RULE) sang action đề xuất (PAUSE, DECREASE).
// Khác với FLAG_RULE (meta/alert_flags): metrics → flag. Ở đây: flag → action.
// Theo FolkForm v4.1 (WF-03 Kill Engine, WF-04 Budget Engine).
// Đọc ActionRuleConfig từ cfg; khi rỗng dùng ActionRuleSpecs từ config (nguồn duy nhất).
package rules

import (
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
)

// RuleResult kết quả đánh giá: có đề xuất hay không.
type RuleResult struct {
	ShouldPropose bool        // Có nên tạo đề xuất không
	ActionType    string      // PAUSE, DECREASE, INCREASE, v.v.
	Reason        string      // Lý do tự động (hiển thị khi duyệt)
	RuleCode      string      // Mã rule (sl_a, sl_b, mess_trap_suspect, v.v.)
	Value         interface{} // Cho SET_BUDGET, INCREASE, DECREASE
	Label         string      // Label hiển thị (SL-A, Mess Trap ↓, ...)
}

// EvalOptions tùy chọn khi đánh giá (KillRulesEnabled từ automationConfig).
type EvalOptions struct {
	KillRulesEnabled bool // FALSE → skip SL-D, SL-E, CHS Kill, KO-B (vd: Pancake down)
	// PATCH 04 Safety guard: khi window_shopping_pattern, vẫn kill nếu Msg_Rate < 1% hoặc CPM < 40k.
	MsgRateRatio float64 // 0–1 (vd: 0.08 = 8%). Khi > 0 và < 0.01 → vẫn kill (traffic rác).
	CpmVnd       float64 // CPM VND. Khi > 0 và < 40000 → vẫn kill (Mess Trap rõ ràng).
}

// EvaluateAlertFlags đánh giá alertFlags từ currentMetrics, trả về RuleResult nếu cần đề xuất.
// opts: nil = default (KillRulesEnabled=true). cfg: config từ ads_meta_config (nil = dùng ActionRuleSpecs).
func EvaluateAlertFlags(alertFlags []interface{}, opts *EvalOptions) *RuleResult {
	return EvaluateAlertFlagsWithConfig(alertFlags, opts, nil)
}

// EvaluateAlertFlagsWithConfig đánh giá với config. Exception flags đọc từ config (GetExceptionFlagsForKill).
func EvaluateAlertFlagsWithConfig(alertFlags []interface{}, opts *EvalOptions, cfg *adsmodels.CampaignConfigView) *RuleResult {
	if len(alertFlags) == 0 {
		return nil
	}
	flags := make(map[string]bool)
	for _, f := range alertFlags {
		if s, ok := f.(string); ok && s != "" {
			flags[s] = true
		}
	}
	// Exception flags: khi có thì bỏ qua kill (đọc từ config)
	for _, code := range adsconfig.GetExceptionFlagsForKill(cfg) {
		if flags[code] {
			return nil
		}
	}
	skipFreezable := opts != nil && !opts.KillRulesEnabled
	rules := getKillRules(cfg)
	for _, r := range rules {
		if !ruleMatches(flags, r.Flag, r.RequireFlags) {
			continue
		}
		if skipFreezable && r.Freeze {
			continue
		}
		ruleCode := r.RuleCode
		if ruleCode == "" {
			ruleCode = r.Flag
		}
		// PATCH 04: Window Shopping Pattern — suspend Mess Trap Guard đến 14:00 (FolkForm v4.1).
		// Safety guard: vẫn kill nếu Msg_Rate < 1% (traffic rác) hoặc CPM < 40k (Mess Trap rõ ràng).
		if (ruleCode == "mess_trap_suspect" || ruleCode == "mess_trap_confirmed") &&
			flags["window_shopping_pattern"] &&
			adsconfig.IsBefore1400Vietnam(time.Now()) {
			safetyKill := opts != nil && ((opts.MsgRateRatio > 0 && opts.MsgRateRatio < 0.01) || (opts.CpmVnd > 0 && opts.CpmVnd < 40000))
			if !safetyKill {
				continue
			}
		}
		return &RuleResult{
			ShouldPropose: true,
			ActionType:    r.Action,
			Reason:        r.Reason,
			RuleCode:      ruleCode,
			Label:         r.Label,
		}
	}
	return nil
}

// getKillRules trả về kill rules: từ cfg nếu có; không thì từ DefaultKillRuleSpecs.
func getKillRules(cfg *adsmodels.CampaignConfigView) []adsmodels.ActionRuleItem {
	if cfg != nil && len(cfg.ActionRuleConfig.KillRules) > 0 {
		return cfg.ActionRuleConfig.KillRules
	}
	return adsconfig.DefaultActionRuleConfig().KillRules
}

// EvaluateForDecrease đánh giá có nên đề xuất DECREASE không (Budget Engine).
func EvaluateForDecrease(alertFlags []interface{}) *RuleResult {
	return EvaluateForDecreaseWithConfig(alertFlags, nil)
}

// EvaluateForDecreaseWithConfig đánh giá DECREASE với config. Khi cfg có DecreaseRules thì dùng; rỗng thì dùng DefaultDecreaseRuleSpecs.
func EvaluateForDecreaseWithConfig(alertFlags []interface{}, cfg *adsmodels.CampaignConfigView) *RuleResult {
	// Công tắc tổng: tắt nhóm tăng/giảm ngân sách → skip toàn bộ
	if cfg != nil && !cfg.AutomationConfig.EffectiveBudgetRulesEnabled() {
		return nil
	}
	flags := make(map[string]bool)
	for _, f := range alertFlags {
		if s, ok := f.(string); ok && s != "" {
			flags[s] = true
		}
	}
	// Exception flags: khi có thì bỏ qua decrease (đọc từ config)
	for _, code := range adsconfig.GetExceptionFlagsForDecrease(cfg) {
		if flags[code] {
			return nil
		}
	}
	rules := getDecreaseRules(cfg)
	for _, r := range rules {
		if !ruleMatches(flags, r.Flag, r.RequireFlags) {
			continue
		}
		ruleCode := r.RuleCode
		if ruleCode == "" {
			ruleCode = r.Flag
		}
		return &RuleResult{
			ShouldPropose: true,
			ActionType:    r.Action,
			Reason:        r.Reason,
			RuleCode:      ruleCode,
			Value:         r.Value,
			Label:         r.Label,
		}
	}
	return nil
}

// getDecreaseRules trả về decrease rules: từ cfg nếu có; không thì từ DefaultDecreaseRuleSpecs.
func getDecreaseRules(cfg *adsmodels.CampaignConfigView) []adsmodels.ActionRuleItem {
	if cfg != nil && len(cfg.ActionRuleConfig.DecreaseRules) > 0 {
		return cfg.ActionRuleConfig.DecreaseRules
	}
	return adsconfig.DefaultActionRuleConfig().DecreaseRules
}

// EvaluateForIncrease đánh giá có nên đề xuất INCREASE không (R08). Chỉ khi BudgetRulesEnabled.
func EvaluateForIncrease(alertFlags []interface{}, cfg *adsmodels.CampaignConfigView) *RuleResult {
	if cfg != nil && !cfg.AutomationConfig.EffectiveBudgetRulesEnabled() {
		return nil
	}
	flags := make(map[string]bool)
	for _, f := range alertFlags {
		if s, ok := f.(string); ok && s != "" {
			flags[s] = true
		}
	}
	rules := getIncreaseRules(cfg)
	for _, r := range rules {
		if !ruleMatches(flags, r.Flag, r.RequireFlags) {
			continue
		}
		ruleCode := r.RuleCode
		if ruleCode == "" {
			ruleCode = r.Flag
		}
		return &RuleResult{
			ShouldPropose: true,
			ActionType:    r.Action,
			Reason:        r.Reason,
			RuleCode:      ruleCode,
			Value:         r.Value,
			Label:         r.Label,
		}
	}
	return nil
}

func getIncreaseRules(cfg *adsmodels.CampaignConfigView) []adsmodels.ActionRuleItem {
	if cfg != nil && len(cfg.ActionRuleConfig.IncreaseRules) > 0 {
		return cfg.ActionRuleConfig.IncreaseRules
	}
	return adsconfig.DefaultActionRuleConfig().IncreaseRules
}

// ruleMatches kiểm tra rule có match với flags không.
// Nếu RequireFlags có phần tử: TẤT CẢ phải có. Nếu không: dùng Flag (single).
func ruleMatches(flags map[string]bool, singleFlag string, requireFlags []string) bool {
	if len(requireFlags) > 0 {
		for _, f := range requireFlags {
			if !flags[f] {
				return false
			}
		}
		return true
	}
	return singleFlag != "" && flags[singleFlag]
}

// EvaluateForResume đánh giá có nên đề xuất RESUME không (R02 Morning On).
// Chỉ khi mo_eligible trong flags VÀ không có kill flag (sl_*, chs_critical, ko_*, trim_eligible).
func EvaluateForResume(alertFlags []interface{}, cfg *adsmodels.CampaignConfigView) *RuleResult {
	flags := make(map[string]bool)
	for _, f := range alertFlags {
		if s, ok := f.(string); ok && s != "" {
			flags[s] = true
		}
	}
	if !flags["mo_eligible"] {
		return nil
	}
	// Không bật nếu có kill flag (camp bị kill vì lý do xấu)
	killFlags := []string{"sl_a", "sl_b", "sl_c", "sl_d", "sl_e", "chs_critical", "ko_a", "ko_b", "ko_c", "trim_eligible"}
	for _, kf := range killFlags {
		if flags[kf] {
			return nil
		}
	}
	return &RuleResult{
		ShouldPropose: true,
		ActionType:    "RESUME",
		Reason:        "Hệ thống đề xuất [Morning On]: Camp đủ điều kiện bật lại sáng (MO-A)",
		RuleCode:      "morning_on",
		Label:         "Morning On",
	}
}
