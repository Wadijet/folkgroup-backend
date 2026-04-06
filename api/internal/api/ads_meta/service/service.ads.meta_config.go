// Package adssvc — Service cấu hình Meta Ads (common, flagRule, actionRule, automation).
package adssvc

import (
	"context"
	"fmt"
	"time"

	adsconfig "meta_commerce/internal/api/ads_meta/config"
	adsmodels "meta_commerce/internal/api/ads_meta/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GetAdsMetaConfig lấy cấu hình Meta Ads theo ad account. 1 document per (adAccountId, ownerOrgID).
// Không có thì trả về config mặc định đầy đủ.
func GetAdsMetaConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (*adsmodels.AdsMetaConfig, error) {
	cfg, err := adsconfig.GetConfig(ctx, adAccountId, ownerOrgID)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		// Merge common với default nếu thiếu
		defCommon := adsconfig.DefaultCommonConfig()
		if cfg.Account.CommonConfig.Timezone == "" {
			cfg.Account.CommonConfig.Timezone = defCommon.Timezone
		}
		if cfg.Campaign.FlagRuleConfig.TrimStartHour == 0 && cfg.Campaign.FlagRuleConfig.TrimEndHour == 0 {
			cfg.Campaign.FlagRuleConfig.TrimStartHour = 14
			cfg.Campaign.FlagRuleConfig.TrimEndHour = 20
		}
		if cfg.Campaign.FlagRuleConfig.Thresholds == nil {
			cfg.Campaign.FlagRuleConfig.Thresholds = make(map[string]float64)
		}
		return cfg, nil
	}
	// Không có config trong DB → trả về config mặc định đầy đủ (FolkForm v4.1)
	return &adsmodels.AdsMetaConfig{
		AdAccountId:         adAccountId,
		OwnerOrganizationID: ownerOrgID,
		Account: adsmodels.AccountConfig{
			CommonConfig:    adsconfig.DefaultCommonConfig(),
			AutomationConfig: adsconfig.DefaultAutomationConfig(),
		},
		Campaign: adsmodels.CampaignConfig{
			FlagRuleConfig:   adsconfig.DefaultFlagRuleConfig(),
			ActionRuleConfig: adsconfig.DefaultActionRuleConfig(),
		},
	}, nil
}

// GetCampaignConfig lấy CampaignConfigView cho campaign. Dùng bởi evaluation, engine, scheduler.
func GetCampaignConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (*adsmodels.CampaignConfigView, error) {
	cfg, err := GetAdsMetaConfig(ctx, adAccountId, ownerOrgID)
	if err != nil {
		return nil, err
	}
	v := cfg.ToCampaignView()
	return &v, nil
}

// UpdateAdsMetaConfig cập nhật cấu hình Meta Ads. Upsert nếu chưa có. 1 document per (adAccountId, ownerOrgID).
func UpdateAdsMetaConfig(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, config *adsmodels.AdsMetaConfig) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return fmt.Errorf("không tìm thấy collection ads_meta_config")
	}
	now := time.Now().UnixMilli()
	config.AdAccountId = adAccountId
	config.OwnerOrganizationID = ownerOrgID
	config.UpdatedAt = now

	filter := bson.M{"adAccountId": adAccountId, "ownerOrganizationId": ownerOrgID}
	var existing adsmodels.AdsMetaConfig
	err := coll.FindOne(ctx, filter).Decode(&existing)
	if err != nil {
		// Insert mới
		config.CreatedAt = now
		_, err = coll.InsertOne(ctx, config)
		return err
	}
	config.CreatedAt = existing.CreatedAt
	_, err = coll.ReplaceOne(ctx, filter, config)
	return err
}

// getActionRulesFromConfig trả về danh sách rule (code, autoPropose, autoApprove).
// Ưu tiên ActionRuleConfig (đã gộp). Fallback mặc định từ definitions.
func getActionRulesFromConfig(metaCfg *adsmodels.CampaignConfigView) []struct{ code string; autoPropose, autoApprove bool } {
	if metaCfg == nil {
		return nil
	}
	var rules []struct{ code string; autoPropose, autoApprove bool }
	// 1. Ưu tiên ActionRuleConfig (cấu trúc mới — đã gộp AutoPropose, AutoApprove)
	arc := &metaCfg.ActionRuleConfig
	for _, r := range arc.KillRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		rules = append(rules, struct{ code string; autoPropose, autoApprove bool }{code, r.AutoPropose, r.AutoApprove})
	}
	for _, r := range arc.DecreaseRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		rules = append(rules, struct{ code string; autoPropose, autoApprove bool }{code, r.AutoPropose, r.AutoApprove})
	}
	if len(rules) > 0 {
		return rules
	}
	// 2. Mặc định từ definitions (actionRules đã bỏ, không còn fallback config cũ)
	def := adsconfig.DefaultActionRuleConfig()
	for _, r := range def.KillRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		rules = append(rules, struct{ code string; autoPropose, autoApprove bool }{code, r.AutoPropose, r.AutoApprove})
	}
	for _, r := range def.DecreaseRules {
		code := r.RuleCode
		if code == "" {
			code = r.Flag
		}
		rules = append(rules, struct{ code string; autoPropose, autoApprove bool }{code, r.AutoPropose, r.AutoApprove})
	}
	return rules
}

// ShouldAutoPropose kiểm tra ruleCode có được tự động đề xuất không.
// Đọc từ ActionRuleConfig (KillRules, DecreaseRules đã gộp AutoPropose). metaCfg nil = true.
func ShouldAutoPropose(ruleCode string, metaCfg *adsmodels.CampaignConfigView) bool {
	rules := getActionRulesFromConfig(metaCfg)
	for _, r := range rules {
		if r.code == ruleCode {
			return r.autoPropose
		}
	}
	return true
}

// ShouldAutoApprove kiểm tra ruleCode có được tự động phê duyệt không.
// Đọc từ ActionRuleConfig (đã gộp AutoApprove). metaCfg nil = false.
func ShouldAutoApprove(ruleCode string, metaCfg *adsmodels.CampaignConfigView) bool {
	rules := getActionRulesFromConfig(metaCfg)
	for _, r := range rules {
		if r.code == ruleCode {
			return r.autoApprove
		}
	}
	return false
}
