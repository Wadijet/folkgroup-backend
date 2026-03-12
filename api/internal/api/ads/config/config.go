// Package config — Cấu hình Meta Ads (common, flagRule, actionRule). Dùng chung bởi meta evaluation và ads.
package config

import (
	"context"
	"strings"
	"time"

	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// Objectives cho Purchase Through Messaging (FolkForm v4.1 PATCH 00). AI Agent CHỈ apply cho campaign loại này.
// OUTCOME_SALES = Sales objective; MESSAGES = legacy Messaging. Campaign khác (Website, Video, Reach...) bỏ qua.
var PurchaseThroughMessagingObjectives = []string{"OUTCOME_SALES", "MESSAGES"}

// IsPurchaseThroughMessagingCampaign kiểm tra campaign có phải Purchase Through Messaging không (PATCH 00).
// objective rỗng → cho phép (backward compat, có thể chưa sync). objective có giá trị → phải trong danh sách.
func IsPurchaseThroughMessagingCampaign(objective string) bool {
	if objective == "" {
		return true
	}
	for _, o := range PurchaseThroughMessagingObjectives {
		if objective == o {
			return true
		}
	}
	return false
}

// ScopeFilterPurchaseMessaging trả về filter MongoDB cho campaign Purchase Through Messaging (PATCH 00).
// Dùng trong scheduler, auto propose — chỉ xử lý campaign objective OUTCOME_SALES hoặc MESSAGES.
func ScopeFilterPurchaseMessaging() bson.M {
	return bson.M{
		"$or": []bson.M{
			{"objective": bson.M{"$in": PurchaseThroughMessagingObjectives}},
			{"objective": ""},
			{"objective": bson.M{"$exists": false}},
		},
	}
}

// Key ngưỡng FLAG_RULE — dùng trong flagRuleConfig.thresholds.
const (
	KeyCpaMessKill        = "cpaMessKill"
	KeyCpaPurchaseHardStop = "cpaPurchaseHardStop"
	KeyConvRateMessTrap   = "convRateMessTrap"
	KeyConvRateMessTrap6  = "convRateMessTrap6"
	KeyCtrKill            = "ctrKill"
	KeyMsgRateLow         = "msgRateLow"
	KeyCpmMessTrapLow     = "cpmMessTrapLow"
	KeyCpaMessTrapLow     = "cpaMessTrapLow"
	KeyCpmHigh            = "cpmHigh"
	KeyCpmKoCMultiplier   = "cpmKoCMultiplier"
	KeyFrequencyHigh      = "frequencyHigh"
	KeyFrequencyTrim      = "frequencyTrim"
	KeyMessTrapSuspectMin = "messTrapSuspectMin"
	KeyMessTrapSlDMin     = "messTrapSlDMin"
	KeyCtrTrafficRac      = "ctrTrafficRac"
	KeyChsWarningThreshold = "chsWarningThreshold"
	KeySafetyNetOrdersMin = "safetyNetOrdersMin"
	KeySafetyNetCrMin     = "safetyNetCrMin"
	KeySlEOrdersMin       = "slEOrdersMin"
	KeySlECrMax           = "slECrMax"
	KeyMqsSlAMax          = "mqsSlAMax"
	KeyMqsSlADecreaseMin  = "mqsSlADecreaseMin"
	KeyMqsSlEMax          = "mqsSlEMax"
	KeyMqsKoBMax          = "mqsKoBMax"
	KeyMqsChsKillMax      = "mqsChsKillMax"
	KeySpendPctSlB        = "spendPctSlB"
	KeySpendPctSlC        = "spendPctSlC"
	KeySpendPctSlD        = "spendPctSlD"
	KeySpendPctKoB        = "spendPctKoB"
	KeySpendPctKoC        = "spendPctKoC"
	KeySpendPctMessTrap   = "spendPctMessTrap"
	KeySpendPctKoAMax     = "spendPctKoAMax"
	KeyRuntimeMinutesKoA  = "runtimeMinutesKoA"
	KeyTrimOrdersMin      = "trimOrdersMin"
	KeySpendPctSlBBlitz   = "spendPctSlBBlitz"
	KeyRuntimeMinutesBase = "runtimeMinutesBase"
	KeySpendPctBase       = "spendPctBase"
	KeyConvRateStrong     = "convRateStrong"     // Exception: CR >= X → bảo vệ, không kill (doc: 20%)
	KeyCpaMessMoMax       = "cpaMessMoMax"       // Morning On: CPA_Mess < X (camp tốt)
	KeyCpaMessNoonCutMin  = "cpaMessNoonCutMin"  // Noon Cut: CPA_Mess > X (camp đắt)
	KeySpendPctNoonCutMax = "spendPctNoonCutMax" // Noon Cut: Spend < X%
)

// DefaultFlagRuleConfig config đầy đủ: thresholds + trim window + flag definitions. InitDefaultConfig dùng làm nguồn cho document mới.
func DefaultFlagRuleConfig() adsmodels.FlagRuleConfig {
	return adsmodels.FlagRuleConfig{
		Thresholds:      DefaultThresholds(),
		TrimStartHour:   14, // fact inTrimWindow: trim_eligible chỉ chạy trong khung 14h–20h
		TrimEndHour:     20,
		FlagDefinitions: DefaultFlagDefinitions(), // Định nghĩa từng cờ (sl_a, trim_eligible, mo_eligible, ...)
	}
}

// toActionRuleItems chuyển ActionRuleSpec sang ActionRuleItem (gộp Label, AutoPropose, AutoApprove).
func toActionRuleItems(specs []ActionRuleSpec) []adsmodels.ActionRuleItem {
	out := make([]adsmodels.ActionRuleItem, len(specs))
	for i, s := range specs {
		out[i] = adsmodels.ActionRuleItem{
			Flag:         s.Flag,
			RequireFlags: s.RequireFlags,
			RuleCode:     s.RuleCode,
			Action:       s.Action,
			Reason:       s.Reason,
			Value:        s.Value,
			Freeze:       s.Freeze,
			Priority:     s.Priority,
			Label:        s.Label,
			AutoPropose:  s.AutoPropose,
			AutoApprove:  s.AutoApprove,
		}
	}
	return out
}

// DefaultActionRuleConfig kill rules, decrease rules và increase rules từ FolkForm v4.1. Dùng ActionRuleSpecs làm nguồn duy nhất.
func DefaultActionRuleConfig() adsmodels.ActionRuleConfig {
	return adsmodels.ActionRuleConfig{
		KillRules:     toActionRuleItems(DefaultKillRuleSpecs()),
		DecreaseRules: toActionRuleItems(DefaultDecreaseRuleSpecs()),
		IncreaseRules: toActionRuleItems(DefaultIncreaseRuleSpecs()),
	}
}

// DefaultAutomationConfig từ FolkForm v4.1. ActionRules đã gộp vào ActionRuleConfig (KillRules, DecreaseRules).
func DefaultAutomationConfig() adsmodels.AutomationConfig {
	killTrue := true
	budgetTrue := true
	return adsmodels.AutomationConfig{
		AutoProposeEnabled: true,
		KillRulesEnabled:   &killTrue,
		BudgetRulesEnabled: &budgetTrue,
	}
}

// GetExceptionFlagsForKill trả về danh sách cờ khi có thì bỏ qua kill. Đọc từ ActionRuleConfig.
func GetExceptionFlagsForKill(cfg *adsmodels.CampaignConfigView) []string {
	if cfg != nil && len(cfg.ActionRuleConfig.ExceptionFlagsForKill) > 0 {
		return cfg.ActionRuleConfig.ExceptionFlagsForKill
	}
	return []string{"safety_net", "conv_rate_strong"}
}

// GetExceptionFlagsForDecrease trả về danh sách cờ khi có thì bỏ qua decrease. Đọc từ ActionRuleConfig.
func GetExceptionFlagsForDecrease(cfg *adsmodels.CampaignConfigView) []string {
	if cfg != nil && len(cfg.ActionRuleConfig.ExceptionFlagsForDecrease) > 0 {
		return cfg.ActionRuleConfig.ExceptionFlagsForDecrease
	}
	return []string{"conv_rate_strong"}
}

// OnboardingDays số ngày onboarding (FolkForm v4.1 PATCH 01). Sau 14 ngày siết dần theo /approve_thresholds.
const OnboardingDays = 14

// NoonCutStartHour, NoonCutEndHour khung giờ Noon Cut — không tăng budget (FolkForm: Increase skip 12–14:30).
const NoonCutStartHour = 12
const NoonCutEndHour = 14
const NoonCutEndMinute = 30

// IsBefore1400Vietnam trả về true nếu thời điểm t (Vietnam) trước 14:00. Dùng cho PATCH 04: suspend Mess Trap đến 14:00 khi WINDOW_SHOPPING_PATTERN.
func IsBefore1400Vietnam(t time.Time) bool {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := t.In(loc)
	return now.Hour() < 14
}

// IsNoonCutWindow trả về true nếu thời điểm t nằm trong khung 12:00–14:30 (Vietnam). Trong khung này không chạy Increase rules.
func IsNoonCutWindow(t time.Time) bool {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := t.In(loc)
	h, m := now.Hour(), now.Minute()
	if h < NoonCutStartHour {
		return false
	}
	if h > NoonCutEndHour {
		return false
	}
	if h == NoonCutEndHour && m >= NoonCutEndMinute {
		return false
	}
	return true
}

// IsOnboardingMode kiểm tra account đang trong giai đoạn onboarding không. Khi true: dùng threshold nới (CPA Kill 250k, CPA Pur 1.4M).
func IsOnboardingMode(cfg *adsmodels.CampaignConfigView, t time.Time) bool {
	if cfg == nil || !cfg.AutomationConfig.OnboardingMode {
		return false
	}
	deployedAt := cfg.AutomationConfig.OnboardingDeployedAt
	if deployedAt <= 0 {
		return true // Chưa set deployedAt — admin bật thủ công, giữ onboarding
	}
	daysSince := (t.UnixMilli() - deployedAt) / (24 * 60 * 60 * 1000)
	return daysSince < OnboardingDays
}

// onboardingThresholds trả về ngưỡng khi Onboarding (FolkForm v4.1 PATCH 01). CPA nới, CR/CTR thắt.
func onboardingThresholds() map[string]float64 {
	return map[string]float64{
		KeyCpaMessKill:        250_000,  // Nới: 180k → 250k
		KeyCpaPurchaseHardStop: 1_400_000, // Nới: 1.05M → 1.4M
		KeyConvRateMessTrap:    0.03,     // Thắt: 5% → 3%
		KeyConvRateMessTrap6:   0.04,     // Thắt: 6% → 4%
		KeyCtrKill:             0.0025,  // Thắt: 0.35% → 0.25%
		KeyMqsSlAMax:           0.5,     // Chờ lâu hơn: 1 → 0.5 trước khi kill
	}
}

// GetThreshold trả về giá trị từ config hoặc default. cfg nil = dùng DefaultThresholds().
func GetThreshold(key string, cfg *adsmodels.CampaignConfigView) float64 {
	if cfg != nil && cfg.FlagRuleConfig.Thresholds != nil {
		if v, ok := cfg.FlagRuleConfig.Thresholds[key]; ok {
			return v
		}
	}
	if v, ok := DefaultThresholds()[key]; ok {
		return v
	}
	return 0
}

// GetThresholdWithEventOverride — Onboarding + Mess Trap Event Override. Ưu tiên: Onboarding → Event → Default.
func GetThresholdWithEventOverride(key string, cfg *adsmodels.CampaignConfigView, t time.Time) float64 {
	// Onboarding 14 ngày: CPA nới, CR/CTR thắt (FolkForm v4.1 PATCH 01)
	if IsOnboardingMode(cfg, t) {
		if v, ok := onboardingThresholds()[key]; ok {
			return v
		}
	}
	// Event window: Mess Trap Event Override (FolkForm v4.1 PATCH 04). CR < 3% VÀ sau 40 mess → SUSPECT.
	// Thắt CR (3% thay vì 5%), tăng sample (40 mess thay vì 20) để tránh kill nhầm window shopping.
	if inEvent, _, _ := IsEventWindow(t); inEvent {
		if key == KeyConvRateMessTrap {
			return 0.03 // Thắt: 5% → 3% trong event
		}
		if key == KeyConvRateMessTrap6 {
			return 0.04 // Thắt: 6% → 4% trong event
		}
		if key == KeyMessTrapSlDMin || key == KeyMessTrapSuspectMin {
			return 40 // Tăng sample: 20 → 40 mess trong event
		}
	}
	return GetThreshold(key, cfg)
}

// GetTrimWindow trả về (trimStartHour, trimEndHour) cho fact inTrimWindow. Đọc từ FlagRuleConfig.
func GetTrimWindow(cfg *adsmodels.CampaignConfigView) (start, end int) {
	if cfg != nil {
		s, e := cfg.FlagRuleConfig.TrimStartHour, cfg.FlagRuleConfig.TrimEndHour
		if s != 0 || e != 0 {
			if s == 0 {
				s = 14
			}
			if e == 0 {
				e = 20
			}
			return s, e
		}
	}
	return 14, 20
}

// GetCommon trả về CommonConfig từ cfg hoặc default.
func GetCommon(cfg *adsmodels.CampaignConfigView) adsmodels.CommonConfig {
	def := DefaultCommonConfig()
	if cfg == nil {
		return def
	}
	c := cfg.CommonConfig
	if c.Timezone == "" {
		c.Timezone = def.Timezone
	}
	if c.Cb4MessThreshold == 0 {
		c.Cb4MessThreshold = def.Cb4MessThreshold
	}
	if c.NightOffHourProtect == 0 {
		c.NightOffHourProtect = def.NightOffHourProtect
	}
	if c.NightOffHourEfficiency == 0 {
		c.NightOffHourEfficiency = def.NightOffHourEfficiency
	}
	if c.NightOffHourNormal == 0 {
		c.NightOffHourNormal = def.NightOffHourNormal
	}
	if c.NightOffMinuteNormal == 0 {
		c.NightOffMinuteNormal = def.NightOffMinuteNormal
	}
	if c.NightOffHourBlitz == 0 {
		c.NightOffHourBlitz = def.NightOffHourBlitz
	}
	if c.BestDayWindowDays == 0 {
		c.BestDayWindowDays = def.BestDayWindowDays
	}
	return c
}

// adAccountIdFilterForConfig trả về filter cho adAccountId — ads_meta_config lưu "act_XXX", meta_campaigns có thể trả "XXX".
func adAccountIdFilterForConfig(adAccountId string) interface{} {
	if adAccountId == "" {
		return adAccountId
	}
	if strings.HasPrefix(adAccountId, "act_") {
		return bson.M{"$in": bson.A{adAccountId, strings.TrimPrefix(adAccountId, "act_")}}
	}
	return bson.M{"$in": bson.A{adAccountId, "act_" + adAccountId}}
}

// GetConfig lấy AdsMetaConfig theo ad account. 1 document per (adAccountId, ownerOrgID). Không có thì trả về nil.
// Hỗ trợ cả adAccountId "act_XXX" và "XXX" — meta_campaigns có thể lưu format khác ads_meta_config.
func GetConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (*adsmodels.AdsMetaConfig, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return nil, nil
	}
	var doc adsmodels.AdsMetaConfig
	err := coll.FindOne(ctx, bson.M{
		"adAccountId":         adAccountIdFilterForConfig(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil {
		return nil, nil
	}
	if doc.Campaign.FlagRuleConfig.Thresholds == nil {
		doc.Campaign.FlagRuleConfig.Thresholds = make(map[string]float64)
	}
	return &doc, nil
}

// GetConfigForCampaign lấy CampaignConfigView cho campaign. Dùng bởi evaluation, engine, scheduler.
func GetConfigForCampaign(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (*adsmodels.CampaignConfigView, error) {
	cfg, err := GetConfig(ctx, adAccountId, ownerOrgID)
	if err != nil || cfg == nil {
		return nil, err
	}
	v := cfg.ToCampaignView()
	return &v, nil
}

// GetKillRulesEnabled đọc công tắc kill rules từ ads_meta_config. FALSE → skip SL-D, SL-E, CHS Kill, KO-B (vd: Pancake down).
// Dùng bởi metasvc computeSuggestedActions (tránh import cycle với adssvc).
func GetKillRulesEnabled(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) bool {
	cfg, err := GetConfig(ctx, adAccountId, ownerOrgID)
	if err != nil || cfg == nil {
		return true // Mặc định bật
	}
	return cfg.Account.AutomationConfig.EffectiveKillRulesEnabled()
}

// DefaultAccountMode mode mặc định khi tạo config mới. FolkForm v4.1: base NORMAL.
const DefaultAccountMode = "NORMAL"

// InitDefaultConfig tạo ads_meta_config mặc định cho ad account nếu chưa có. Dùng trong init/migration.
func InitDefaultConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (created bool, err error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return false, nil
	}
	n, err := coll.CountDocuments(ctx, bson.M{
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
	})
	if err != nil || n > 0 {
		return false, err
	}
	now := time.Now().UnixMilli()
	doc := &adsmodels.AdsMetaConfig{
		AdAccountId:         adAccountId,
		OwnerOrganizationID: ownerOrgID,
		Account: adsmodels.AccountConfig{
			AccountMode:      DefaultAccountMode,
			CommonConfig:     DefaultCommonConfig(),
			AutomationConfig: DefaultAutomationConfig(),
		},
		Campaign: adsmodels.CampaignConfig{
			FlagRuleConfig:   DefaultFlagRuleConfig(),
			ActionRuleConfig: DefaultActionRuleConfig(),
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err = coll.InsertOne(ctx, doc)
	return err == nil, err
}

// GetWindowMsForCurrentMetrics trả về windowMs cho currentMetrics (7d).
// Ưu tiên từ ads_metric_definitions; fallback 7 ngày. Dùng bởi metasvc (tránh import cycle).
func GetWindowMsForCurrentMetrics(ctx context.Context) int64 {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetricDefinitions)
	if !ok {
		return 7 * 24 * 60 * 60 * 1000
	}
	filter := bson.M{"window": adsmodels.Window7d, "isActive": true}
	opts := mongoopts.FindOne().SetSort(bson.M{"order": 1})
	var doc struct {
		WindowMs int64 `bson:"windowMs"`
	}
	if err := coll.FindOne(ctx, filter, opts).Decode(&doc); err != nil {
		return 7 * 24 * 60 * 60 * 1000
	}
	if doc.WindowMs > 0 {
		return doc.WindowMs
	}
	return 7 * 24 * 60 * 60 * 1000
}
