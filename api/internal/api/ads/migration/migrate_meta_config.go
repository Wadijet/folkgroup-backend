// Package migration — Init ads_meta_config mặc định cho ad accounts có approval config.
package migration

import (
	"context"

	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BackfillAdsMetaConfigLevel gán level=campaign cho document ads_meta_config cũ (cấu trúc flat) chưa có level.
// Chỉ áp dụng cho doc có commonConfig ở top-level (cấu trúc cũ), không động vào doc mới (account, campaign).
func BackfillAdsMetaConfigLevel(ctx context.Context) (updated int, err error) {
	metaColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return 0, nil
	}
	res, err := metaColl.UpdateMany(ctx,
		bson.M{
			"level":       bson.M{"$exists": false},
			"commonConfig": bson.M{"$exists": true}, // Chỉ doc cũ (cấu trúc flat)
		},
		bson.M{"$set": bson.M{"level": adsmodels.LevelCampaign}},
	)
	if err != nil {
		return 0, err
	}
	return int(res.ModifiedCount), nil
}

// InitAdsMetaConfigDefaults tạo ads_meta_config mặc định (level=campaign) cho ad accounts chưa có meta config.
// Nguồn: meta_ad_accounts (ad accounts đã sync).
func InitAdsMetaConfigDefaults(ctx context.Context) (created int, err error) {
	metaColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return created, nil
	}
	cursor, err := metaColl.Find(ctx, bson.M{})
	if err != nil {
		return created, nil
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct {
			AdAccountId         string              `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&doc); err != nil || doc.AdAccountId == "" {
			continue
		}
		ok, err := adsconfig.InitDefaultConfig(ctx, doc.AdAccountId, doc.OwnerOrganizationID)
		if err != nil {
			continue
		}
		if ok {
			created++
		}
	}
	return created, nil
}
