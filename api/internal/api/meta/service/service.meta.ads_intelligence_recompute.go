// Package metasvc — Ads Intelligence: cập nhật raw + layer + roll-up sau khi nguồn (insight/POS/hội thoại) đổi.
// Được gọi từ worker AI Decision khi consume event ads.intelligence.recompute_requested (event-driven).
package metasvc

import (
	"context"
	"fmt"
	"strings"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RollUpFromAdToAccount sau khi Ad cập nhật raw/layers, tính lại AdSet → Campaign → Account.
func RollUpFromAdToAccount(ctx context.Context, adId, adAccountId string, ownerOrgID primitive.ObjectID) {
	adSetId, campaignId := getAdSetAndCampaignByAdIdForRollUp(ctx, adId, ownerOrgID)
	if adSetId != "" {
		_ = RecalculateForEntity(ctx, "adset", adSetId, adAccountId, ownerOrgID)
	}
	if campaignId != "" {
		_ = RecalculateForEntity(ctx, "campaign", campaignId, adAccountId, ownerOrgID)
	}
	if adAccountId != "" {
		_ = RecalculateForEntity(ctx, "ad_account", adAccountId, adAccountId, ownerOrgID)
	}
}

func getAdSetAndCampaignByAdIdForRollUp(ctx context.Context, adId string, ownerOrgID primitive.ObjectID) (adSetId, campaignId string) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return "", ""
	}
	filter := bson.M{"adId": adId, "ownerOrganizationId": ownerOrgID}
	var doc struct {
		AdSetId    string `bson:"adSetId"`
		CampaignId string `bson:"campaignId"`
	}
	if err := coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		return "", ""
	}
	return doc.AdSetId, doc.CampaignId
}

// RecomputeModeSource — cập nhật raw theo nguồn rồi layer/roll-up (hook insight/POS/hội thoại).
const RecomputeModeSource = "source"

// RecomputeModeFull — tính lại toàn bộ currentMetrics cho entity (giống RecalculateForEntity; dùng API / batch).
const RecomputeModeFull = "full"

// ApplyAdsIntelligenceRecompute tương đương ApplyAdsIntelligenceRecomputeWithMode(..., RecomputeModeSource).
func ApplyAdsIntelligenceRecompute(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, source string) error {
	return ApplyAdsIntelligenceRecomputeWithMode(ctx, objectType, objectId, adAccountId, ownerOrgID, source, RecomputeModeSource)
}

// ApplyAdsIntelligenceRecomputeWithMode: mode full → chỉ RecalculateForEntity; mode source → UpdateRawFromSource + roll-up (hook).
func ApplyAdsIntelligenceRecomputeWithMode(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, source string, recomputeMode string) error {
	mode := strings.TrimSpace(strings.ToLower(recomputeMode))
	if mode == "" || mode == RecomputeModeSource {
		return applyAdsIntelligenceRecomputeSource(ctx, objectType, objectId, adAccountId, ownerOrgID, source)
	}
	if mode == RecomputeModeFull {
		return RecalculateForEntity(ctx, objectType, objectId, adAccountId, ownerOrgID)
	}
	return fmt.Errorf("recomputeMode không hợp lệ: %q (chỉ %s|%s)", recomputeMode, RecomputeModeSource, RecomputeModeFull)
}

func applyAdsIntelligenceRecomputeSource(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, source string) error {
	src := strings.TrimSpace(strings.ToLower(source))
	switch src {
	case "meta":
		if objectType == "ad" {
			if err := UpdateRawFromSource(ctx, objectType, objectId, adAccountId, ownerOrgID, "meta"); err != nil {
				return err
			}
			RollUpFromAdToAccount(ctx, objectId, adAccountId, ownerOrgID)
			return nil
		}
		return RecalculateForEntity(ctx, objectType, objectId, adAccountId, ownerOrgID)
	case "pancake.pos", "pancake.conversation":
		if objectType != "ad" {
			return fmt.Errorf("nguồn %s chỉ hỗ trợ objectType=ad, nhận %s", src, objectType)
		}
		if err := UpdateRawFromSource(ctx, "ad", objectId, adAccountId, ownerOrgID, source); err != nil {
			return err
		}
		RollUpFromAdToAccount(ctx, objectId, adAccountId, ownerOrgID)
		return nil
	default:
		return fmt.Errorf("nguồn Ads Intelligence không hỗ trợ: %q", source)
	}
}
