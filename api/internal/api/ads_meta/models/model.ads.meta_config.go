// Package models — Cấu hình quản lý Meta Ads (FLAG_RULE, ACTION_RULE, automation).
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Level config — dùng cho API và backward compat. Theo FolkForm v4.1: campaign là level chính.
const (
	LevelCampaign  = "campaign"
	LevelAd        = "ad"
	LevelAdSet     = "adset"
	LevelAdAccount = "ad_account"
)

// AccountConfig cấu hình cấp ad account: mode, common, automation switches.
type AccountConfig struct {
	AccountMode      string           `json:"accountMode" bson:"accountMode,omitempty"` // BLITZ | NORMAL | EFFICIENCY | PROTECT
	CommonConfig    CommonConfig     `json:"commonConfig" bson:"commonConfig"`
	AutomationConfig AutomationConfig `json:"automationConfig" bson:"automationConfig"`
}

// CampaignConfig cấu hình cấp campaign: flag rules, action rules.
type CampaignConfig struct {
	FlagRuleConfig   FlagRuleConfig   `json:"flagRuleConfig" bson:"flagRuleConfig"`
	ActionRuleConfig ActionRuleConfig `json:"actionRuleConfig" bson:"actionRuleConfig"`
}

// AdSetConfig cấu hình cấp ad set. Dự phòng cho tương lai.
type AdSetConfig struct {
	// Reserved
}

// AdConfig cấu hình cấp ad. Dự phòng cho tương lai.
type AdConfig struct {
	// Reserved
}

// AdsMetaConfig 1 document per (adAccountId, ownerOrgID). Tất cả config gộp trong 1 document.
// Tập trung và thống nhất: account, campaign, adSet, ad.
type AdsMetaConfig struct {
	ID                  primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	AdAccountId         string             `json:"adAccountId" bson:"adAccountId" index:"single:1"`
	OwnerOrganizationID primitive.ObjectID `json:"ownerOrganizationId" bson:"ownerOrganizationId" index:"single:1"`
	Account             AccountConfig     `json:"account" bson:"account"`
	Campaign            CampaignConfig     `json:"campaign" bson:"campaign"`
	AdSet               AdSetConfig       `json:"adSet,omitempty" bson:"adSet,omitempty"`
	Ad                  AdConfig          `json:"ad,omitempty" bson:"ad,omitempty"`
	CreatedAt           int64             `json:"createdAt" bson:"createdAt"`
	UpdatedAt           int64             `json:"updatedAt" bson:"updatedAt"`
}

// CampaignConfigView view phẳng cho campaign — dùng bởi evaluation, engine, scheduler.
// Gộp từ Account (common, automation) + Campaign (flagRule, actionRule).
type CampaignConfigView struct {
	AccountMode      string
	CommonConfig    CommonConfig
	FlagRuleConfig  FlagRuleConfig
	ActionRuleConfig ActionRuleConfig
	AutomationConfig AutomationConfig
}

// ToCampaignView chuyển AdsMetaConfig sang CampaignConfigView.
func (c *AdsMetaConfig) ToCampaignView() CampaignConfigView {
	if c == nil {
		return CampaignConfigView{}
	}
	return CampaignConfigView{
		AccountMode:       c.Account.AccountMode,
		CommonConfig:      c.Account.CommonConfig,
		FlagRuleConfig:    c.Campaign.FlagRuleConfig,
		ActionRuleConfig:  c.Campaign.ActionRuleConfig,
		AutomationConfig:  c.Account.AutomationConfig,
	}
}

// CommonConfig các cấu hình chung (timezone, scheduler). Trim/base condition đã chuyển sang FlagRuleConfig.
type CommonConfig struct {
	Timezone string `json:"timezone" bson:"timezone"` // VD: Asia/Ho_Chi_Minh — dùng cho trim window, Night Off

	// Circuit Breaker (CB-4): orders_2h=0 VÀ mess_2h > X → trigger. Mặc định 50.
	Cb4MessThreshold int `json:"cb4MessThreshold" bson:"cb4MessThreshold"`

	// Night Off: giờ tắt theo mode. FolkForm R07.
	NightOffHourProtect    int `json:"nightOffHourProtect" bson:"nightOffHourProtect"`       // PROTECT: 21
	NightOffHourEfficiency int `json:"nightOffHourEfficiency" bson:"nightOffHourEfficiency"` // EFFICIENCY: 22
	NightOffHourNormal     int `json:"nightOffHourNormal" bson:"nightOffHourNormal"`         // NORMAL: 22
	NightOffMinuteNormal   int `json:"nightOffMinuteNormal" bson:"nightOffMinuteNormal"`     // NORMAL: 30 phút
	NightOffHourBlitz      int `json:"nightOffHourBlitz" bson:"nightOffHourBlitz"`            // BLITZ: 23

	// Reset Budget: Best_day logic (RULE 10). Enabled=false khi logic chưa implement.
	ResetBudgetEnabled    bool `json:"resetBudgetEnabled" bson:"resetBudgetEnabled"`
	BestDayWindowDays     int  `json:"bestDayWindowDays" bson:"bestDayWindowDays"`           // Số ngày dùng cho Best_day (mặc định 3)

	// Mode Detection S4: Monthly Revenue Target (triệu VNĐ). Pace = revenue_so_far / (target × days_elapsed/total_days).
	// 0 = bỏ qua S4. FolkForm v4.1 Section 3.1.
	MonthlyTarget float64 `json:"monthlyTarget" bson:"monthlyTarget"`
}

// FlagConditionItem một điều kiện đơn — fact + operator + value. Evaluator đọc từ đây để tính.
// Chuẩn json-rules-engine. ThresholdKey, Value, ValueStr, CompareToMetric, ThresholdKeyByMode, SpendPctFallback.
type FlagConditionItem struct {
	Fact               string   `json:"fact" bson:"fact"`
	MetricKey          string   `json:"metricKey,omitempty" bson:"metricKey,omitempty"`
	Operator           string   `json:"operator" bson:"operator"`
	ThresholdKey       string   `json:"thresholdKey,omitempty" bson:"thresholdKey,omitempty"`
	ThresholdKey2      string   `json:"thresholdKey2,omitempty" bson:"thresholdKey2,omitempty"`
	Value              *float64 `json:"value,omitempty" bson:"value,omitempty"`
	ValueStr           string   `json:"valueStr,omitempty" bson:"valueStr,omitempty"`
	CompareToMetric    string   `json:"compareToMetric,omitempty" bson:"compareToMetric,omitempty"`
	ThresholdKeyByMode string   `json:"thresholdKeyByMode,omitempty" bson:"thresholdKeyByMode,omitempty"`
	SpendPctFallback   bool     `json:"spendPctFallback,omitempty" bson:"spendPctFallback,omitempty"`
}

// FlagDefinition định nghĩa đầy đủ một cờ: metrics dùng, logic tính, conditionGroups. Evaluator CHỈ đọc từ đây.
// LogicText: công thức như trong FolkForm doc (vd: "spendPct > 20% AND runtimeMinutes > 90 AND cpaMess > 180k AND mess < 3 AND mqs < 1").
type FlagDefinition struct {
	Code             string                 `json:"code" bson:"code"`
	Label            string                 `json:"label" bson:"label"`
	Description      string                 `json:"description" bson:"description"`
	DocReference     string                 `json:"docReference" bson:"docReference"`
	MetricsUsed      []string               `json:"metricsUsed" bson:"metricsUsed"`             // Danh sách metrics: spendPct, runtimeMinutes, cpaMess, mess, mqs, ...
	LogicText        string                 `json:"logicText" bson:"logicText"`               // Công thức logic như doc: "A AND B AND C" hoặc "A OR B"
	ConditionGroups  [][]FlagConditionItem  `json:"conditionGroups" bson:"conditionGroups"`   // Nguồn cho evaluator — trong group: AND; giữa groups: OR
	Group            string                 `json:"group" bson:"group"`
	Order            int                    `json:"order" bson:"order"`
	Enabled          *bool                  `json:"enabled,omitempty" bson:"enabled,omitempty"` // nil = bật; true = bật; false = tắt
}

// FlagRuleConfig ngưỡng và định nghĩa cờ. Tất cả tham số dùng cho điều kiện flag đều ở đây.
type FlagRuleConfig struct {
	Thresholds map[string]float64 `json:"thresholds" bson:"thresholds"` // Ngưỡng dùng chung (cpaMessKill, spendPctBase, ...)

	// Trim window: fact inTrimWindow = true khi giờ hiện tại trong [TrimStartHour, TrimEndHour). Dùng cho trim_eligible.
	TrimStartHour int `json:"trimStartHour,omitempty" bson:"trimStartHour,omitempty"` // Mặc định 14
	TrimEndHour   int `json:"trimEndHour,omitempty" bson:"trimEndHour,omitempty"`     // Mặc định 20

	// FlagDefinitions: định nghĩa đầy đủ từng cờ (metrics, logic, conditions, enabled). Evaluator CHỈ đọc từ đây.
	// Rỗng/nil = dùng mặc định từ DefaultFlagDefinitions().
	FlagDefinitions []FlagDefinition `json:"flagDefinitions,omitempty" bson:"flagDefinitions,omitempty"`
}

// ActionRuleItem một rule: flag(s) → action. Gộp logic (flag, action) và automation (autoPropose, autoApprove).
type ActionRuleItem struct {
	// Flag: single flag. Dùng khi RequireFlags rỗng (backward compat).
	Flag string `json:"flag" bson:"flag"`

	// RequireFlags: compound — TẤT CẢ flags phải có mới trigger. Ưu tiên hơn Flag khi len > 0.
	// VD: ["chs_warning", "cpa_mess_high"] → DECREASE 15%. RuleCode dùng làm identifier.
	RequireFlags []string `json:"requireFlags,omitempty" bson:"requireFlags,omitempty"`

	// RuleCode: mã rule cho automation (sl_a, chs_warning, ...). Khi dùng RequireFlags thì dùng RuleCode.
	// Khi single flag: RuleCode = Flag nếu để trống.
	RuleCode string `json:"ruleCode,omitempty" bson:"ruleCode,omitempty"`

	Action    string  `json:"action" bson:"action"`             // PAUSE, DECREASE, INCREASE
	Value     float64 `json:"value,omitempty" bson:"value,omitempty"` // % cho DECREASE/INCREASE (20, 30, ...)
	Reason    string  `json:"reason" bson:"reason"`              // Lý do hiển thị
	Freeze    bool    `json:"freeze" bson:"freeze"`             // Bỏ qua khi KillRulesEnabled=false
	Priority  int     `json:"priority" bson:"priority"`          // Thứ tự ưu tiên (nhỏ = trước)

	// Gộp từ AutomationConfig.ActionRules — mỗi rule có cài đặt đề xuất và phê duyệt.
	Label       string `json:"label,omitempty" bson:"label,omitempty"`             // Label hiển thị (SL-A, Mess Trap ↓, ...)
	AutoPropose bool   `json:"autoPropose" bson:"autoPropose"`                       // Hệ thống tự động đề xuất khi rule trigger
	AutoApprove bool   `json:"autoApprove" bson:"autoApprove"`                       // Hệ thống tự động phê duyệt sau khi đề xuất
}

// ActionRuleConfig cấu hình ACTION_RULE (flag → action). Kill rules, Decrease rules, Increase rules và exception flags.
type ActionRuleConfig struct {
	KillRules     []ActionRuleItem `json:"killRules" bson:"killRules"`           // Các rule trigger PAUSE
	DecreaseRules []ActionRuleItem `json:"decreaseRules" bson:"decreaseRules"`     // Các rule trigger DECREASE
	IncreaseRules []ActionRuleItem `json:"increaseRules,omitempty" bson:"increaseRules,omitempty"` // Các rule trigger INCREASE (R08)

	// ExceptionFlagsForKill: khi có cờ này trong alertFlags thì bỏ qua kill (bảo vệ camp). Rỗng = dùng mặc định.
	ExceptionFlagsForKill []string `json:"exceptionFlagsForKill,omitempty" bson:"exceptionFlagsForKill,omitempty"`
	// ExceptionFlagsForDecrease: khi có cờ này thì bỏ qua decrease. Rỗng = dùng mặc định.
	ExceptionFlagsForDecrease []string `json:"exceptionFlagsForDecrease,omitempty" bson:"exceptionFlagsForDecrease,omitempty"`
}

// AutomationActionRule cấu hình cho từng hành động: đề xuất và phê duyệt.
// Gộp theo hành động — mỗi rule có 2 cài đặt: autoPropose, autoApprove.
type AutomationActionRule struct {
	Code         string `json:"code" bson:"code"`                   // sl_a, sl_b, mess_trap_suspect, ...
	Label        string `json:"label" bson:"label"`                 // Label hiển thị (SL-A, Mess Trap ↓, ...)
	AutoPropose  bool   `json:"autoPropose" bson:"autoPropose"`     // Hệ thống tự động đề xuất khi rule trigger
	AutoApprove  bool   `json:"autoApprove" bson:"autoApprove"`      // Hệ thống tự động phê duyệt sau khi đề xuất
}

// AutomationConfig cấu hình tự động hóa: đề xuất và tự động duyệt.
type AutomationConfig struct {
	// AutoProposeEnabled: bật/tắt auto-propose cho ad account. Mặc định true.
	AutoProposeEnabled bool `json:"autoProposeEnabled" bson:"autoProposeEnabled"`

	// KillRulesEnabled: công tắc kill rules. TRUE = chạy bình thường; FALSE = skip SL-D, SL-E, CHS Kill, KO-B (vd: Pancake down). Mặc định true.
	// Dùng pointer để phân biệt "chưa set" (nil) với "false" — khi nil và có FreezeKillRules (legacy) thì dùng !FreezeKillRules.
	KillRulesEnabled *bool `json:"killRulesEnabled,omitempty" bson:"killRulesEnabled,omitempty"`

	// PancakeDownOverride: khi true (Pancake Heartbeat phát hiện không có order 2h) → EffectiveKillRulesEnabled = false.
	PancakeDownOverride bool `json:"pancakeDownOverride,omitempty" bson:"pancakeDownOverride,omitempty"`
	PancakeDownAt      int64 `json:"pancakeDownAt,omitempty" bson:"pancakeDownAt,omitempty"`

	// PancakeSuspectOverride: [HB-3] Divergence — FB_Mess_1h>100, Pancake_orders_1h=0, hôm qua cùng giờ có đơn → freeze 60p.
	PancakeSuspectOverride bool  `json:"pancakeSuspectOverride,omitempty" bson:"pancakeSuspectOverride,omitempty"`
	PancakeSuspectAt      int64  `json:"pancakeSuspectAt,omitempty" bson:"pancakeSuspectAt,omitempty"` // ms — để check 60p

	// BudgetRulesEnabled: công tắc nhóm tăng/giảm ngân sách. TRUE = chạy DECREASE/INCREASE; FALSE = skip (tắt khẩn cấp). Mặc định true.
	BudgetRulesEnabled *bool `json:"budgetRulesEnabled,omitempty" bson:"budgetRulesEnabled,omitempty"`

	// OnboardingMode: FolkForm v4.1 PATCH 01 — 14 ngày đầu deploy dùng threshold nới (CPA Kill 250k, CPA Pur 1.4M) tránh kill nhầm.
	OnboardingMode bool `json:"onboardingMode,omitempty" bson:"onboardingMode,omitempty"`
	// OnboardingDeployedAt: timestamp (ms) khi bật onboarding. Nếu > 0 và (now - DeployedAt) >= 14 ngày → tự coi như hết onboarding.
	OnboardingDeployedAt int64 `json:"onboardingDeployedAt,omitempty" bson:"onboardingDeployedAt,omitempty"`

	// Deprecated: dùng KillRulesEnabled. FreezeKillRules=true tương đương KillRulesEnabled=false.
	FreezeKillRules bool `json:"freezeKillRules,omitempty" bson:"freezeKillRules,omitempty"`

	// Bỏ: actionRules đã gộp vào ActionRuleConfig (KillRules, DecreaseRules có AutoPropose, AutoApprove).
	// BSON omitempty: doc cũ có field này vẫn decode được, bỏ qua.

	// Deprecated: dùng ActionRuleConfig. Giữ để tương thích khi đọc config cũ.
	AutoProposeRuleCodes []string `json:"autoProposeRuleCodes,omitempty" bson:"autoProposeRuleCodes,omitempty"`
	AutoApproveRuleCodes []string `json:"autoApproveRuleCodes,omitempty" bson:"autoApproveRuleCodes,omitempty"`
}

// EffectiveKillRulesEnabled trả về giá trị hiệu dụng của công tắc kill rules.
// PancakeDownOverride hoặc PancakeSuspectOverride (trong 60p) → false. Ưu tiên KillRulesEnabled khi có; nếu nil thì dùng !FreezeKillRules.
func (a *AutomationConfig) EffectiveKillRulesEnabled() bool {
	if a.PancakeDownOverride {
		return false
	}
	// [HB-3] Divergence: freeze 60p. Sau 60p tự gỡ (worker check) hoặc escalate sang PANCAKE_DOWN.
	if a.PancakeSuspectOverride && a.PancakeSuspectAt > 0 {
		if time.Since(time.UnixMilli(a.PancakeSuspectAt)) < 60*time.Minute {
			return false
		}
	}
	if a.KillRulesEnabled != nil {
		return *a.KillRulesEnabled
	}
	return !a.FreezeKillRules
}

// EffectiveBudgetRulesEnabled trả về giá trị hiệu dụng của công tắc budget rules (tăng/giảm ngân sách). Mặc định true.
func (a *AutomationConfig) EffectiveBudgetRulesEnabled() bool {
	if a.BudgetRulesEnabled != nil {
		return *a.BudgetRulesEnabled
	}
	return true
}
