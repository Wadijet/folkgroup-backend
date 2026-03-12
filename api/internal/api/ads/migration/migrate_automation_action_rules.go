// Package migration — Backfill actionRuleConfig cho docs chưa có (actionRules đã bỏ).
package migration

import (
	"context"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
)

// BackfillAutomationActionRules backfill actionRuleConfig (killRules, decreaseRules) mặc định cho docs có config rỗng.
// Trước đây backfill automationConfig.actionRules; đã chuyển sang actionRuleConfig.
func BackfillAutomationActionRules(ctx context.Context) (updated int, err error) {
	metaColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return 0, nil
	}
	now := time.Now().UnixMilli()
	defaultARC := adsconfig.DefaultActionRuleConfig()
	res, err := metaColl.UpdateMany(ctx,
		bson.M{
			"$or": []bson.M{
				{"campaign.actionRuleConfig.killRules": bson.M{"$exists": false}},
				{"campaign.actionRuleConfig.killRules": nil},
				{"campaign.actionRuleConfig.killRules": bson.M{"$size": 0}},
			},
		},
		bson.M{
			"$set": bson.M{
				"campaign.actionRuleConfig.killRules":    defaultARC.KillRules,
				"campaign.actionRuleConfig.decreaseRules": defaultARC.DecreaseRules,
				"updatedAt":                               now,
			},
		},
	)
	if err != nil {
		return 0, err
	}
	return int(res.ModifiedCount), nil
}
