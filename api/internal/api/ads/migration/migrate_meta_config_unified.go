// Package migration — Chuyển ads_meta_config từ cấu trúc cũ (nhiều doc theo level) sang cấu trúc mới (1 doc với account, campaign, adSet, ad).
package migration

import (
	"context"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MigrateAdsMetaConfigToUnified chuyển ads_meta_config sang cấu trúc 1 document per (adAccountId, ownerOrgID).
// Gộp: account (accountMode từ meta_ad_accounts, commonConfig, automationConfig) + campaign (flagRule, actionRule).
func MigrateAdsMetaConfigToUnified(ctx context.Context) (migrated int, err error) {
	metaColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return 0, nil
	}
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		accColl = nil
	}

	// Lấy tất cả doc cũ (có level)
	cursor, err := metaColl.Find(ctx, bson.M{"level": bson.M{"$exists": true}})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	// Gom theo (adAccountId, ownerOrgID)
	type key struct {
		AdAccountId string
		OwnerOrgID  primitive.ObjectID
	}
	merged := make(map[key]*adsmodels.AdsMetaConfig)

	for cursor.Next(ctx) {
		var doc struct {
			AdAccountId         string              `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
			Level               string              `bson:"level"`
			CommonConfig     adsmodels.CommonConfig     `bson:"commonConfig"`
			FlagRuleConfig   adsmodels.FlagRuleConfig   `bson:"flagRuleConfig"`
			ActionRuleConfig adsmodels.ActionRuleConfig `bson:"actionRuleConfig"`
			AutomationConfig adsmodels.AutomationConfig `bson:"automationConfig"`
			CreatedAt        int64                     `bson:"createdAt"`
			UpdatedAt        int64                     `bson:"updatedAt"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		k := key{AdAccountId: doc.AdAccountId, OwnerOrgID: doc.OwnerOrganizationID}
		if merged[k] == nil {
			merged[k] = &adsmodels.AdsMetaConfig{
				AdAccountId:         doc.AdAccountId,
				OwnerOrganizationID: doc.OwnerOrganizationID,
				Account: adsmodels.AccountConfig{
					CommonConfig:    adsconfig.DefaultCommonConfig(),
					AutomationConfig: adsconfig.DefaultAutomationConfig(),
				},
				Campaign: adsmodels.CampaignConfig{
					FlagRuleConfig:   adsconfig.DefaultFlagRuleConfig(),
					ActionRuleConfig: adsconfig.DefaultActionRuleConfig(),
				},
				CreatedAt: doc.CreatedAt,
				UpdatedAt: doc.UpdatedAt,
			}
		}
		cfg := merged[k]
		switch doc.Level {
		case adsmodels.LevelCampaign, adsmodels.LevelAdAccount:
			cfg.Account.CommonConfig = doc.CommonConfig
			cfg.Account.AutomationConfig = doc.AutomationConfig
		}
		if doc.Level == adsmodels.LevelCampaign {
			cfg.Campaign.FlagRuleConfig = doc.FlagRuleConfig
			cfg.Campaign.ActionRuleConfig = doc.ActionRuleConfig
			if doc.CreatedAt > 0 && cfg.CreatedAt == 0 {
				cfg.CreatedAt = doc.CreatedAt
			}
			if doc.UpdatedAt > cfg.UpdatedAt {
				cfg.UpdatedAt = doc.UpdatedAt
			}
		}
	}

	// Lấy accountMode từ meta_ad_accounts
	if accColl != nil {
		for k, cfg := range merged {
			var acc struct {
				AccountMode string `bson:"accountMode"`
			}
			_ = accColl.FindOne(ctx, bson.M{
				"adAccountId":         bson.M{"$regex": "^" + k.AdAccountId + "$", "$options": "i"},
				"ownerOrganizationId": k.OwnerOrgID,
			}).Decode(&acc)
			if acc.AccountMode != "" {
				cfg.Account.AccountMode = acc.AccountMode
			}
		}
	}

	// Xóa doc cũ và insert doc mới
	for k, cfg := range merged {
		// Xóa tất cả doc cũ của (adAccountId, ownerOrgID)
		_, _ = metaColl.DeleteMany(ctx, bson.M{
			"adAccountId":         k.AdAccountId,
			"ownerOrganizationId": k.OwnerOrgID,
		})
		now := time.Now().UnixMilli()
		cfg.UpdatedAt = now
		if cfg.CreatedAt == 0 {
			cfg.CreatedAt = now
		}
		if cfg.Campaign.FlagRuleConfig.Thresholds == nil {
			cfg.Campaign.FlagRuleConfig.Thresholds = make(map[string]float64)
		}
		_, err := metaColl.InsertOne(ctx, cfg)
		if err != nil {
			continue
		}
		migrated++
	}
	return migrated, nil
}
