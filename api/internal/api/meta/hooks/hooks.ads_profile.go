// Package metahooks - Hook tính ads profile (currentMetrics) cho 4 level: Account, Campaign, AdSet, Ad.
// Hook: meta_ad_insights, pc_pos_orders, fb_conversations — nguồn dữ liệu metrics.
// Khi thay đổi → UpdateRawFromSource (chỉ raw nguồn đó) → RecalculateForEntity → roll-up Ad→AdSet→Campaign→Account.
package metahooks

import (
	"context"

	"meta_commerce/internal/api/events"
	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func init() {
	events.OnDataChanged(handleAdsProfileDataChange)
}

// handleAdsProfileDataChange xử lý event để tính lại currentMetrics (ads profile) cho 4 level.
// Chỉ cập nhật raw từ nguồn phát sinh event, rồi tính lại layers.
func handleAdsProfileDataChange(ctx context.Context, e events.DataChangeEvent) {
	if e.Document == nil {
		return
	}
	ownerOrgID := events.GetOwnerOrganizationIDFromDocument(e.Document)
	if ownerOrgID.IsZero() {
		return
	}

	switch e.CollectionName {
	case global.MongoDB_ColNames.MetaAdInsights:
		objectType := events.GetStringField(e.Document, "ObjectType")
		objectId := events.GetStringField(e.Document, "ObjectId")
		adAccountId := events.GetStringField(e.Document, "AdAccountId")
		if objectType == "" || objectId == "" || adAccountId == "" {
			return
		}
		if objectType == "ad" {
			key := entityKey("ad", objectId, adAccountId, ownerOrgID, "meta")
			scheduleAdsRecompute(key, func() {
				bg := context.Background()
				_ = metasvc.UpdateRawFromSource(bg, objectType, objectId, adAccountId, ownerOrgID, "meta")
				triggerRollUpForAd(bg, objectId, adAccountId, ownerOrgID)
			})
		} else {
			key := entityKey(objectType, objectId, adAccountId, ownerOrgID, "meta")
			scheduleAdsRecompute(key, func() {
				bg := context.Background()
				_ = metasvc.RecalculateForEntity(bg, objectType, objectId, adAccountId, ownerOrgID)
			})
		}
		return

	case global.MongoDB_ColNames.PcPosOrders:
		adId := events.GetNestedStringField(e.Document, "posData", "ad_id")
		if adId == "" {
			adId = events.GetNestedStringField(e.Document, "PosData", "ad_id")
		}
		if adId == "" {
			return
		}
		adAccountId := getAdAccountIdByAdId(ctx, adId, ownerOrgID)
		if adAccountId == "" {
			return
		}
		key := entityKey("ad", adId, adAccountId, ownerOrgID, "pancake.pos")
		scheduleAdsRecompute(key, func() {
			bg := context.Background()
			_ = metasvc.UpdateRawFromSource(bg, "ad", adId, adAccountId, ownerOrgID, "pancake.pos")
			triggerRollUpForAd(bg, adId, adAccountId, ownerOrgID)
		})
		return

	case global.MongoDB_ColNames.FbConvesations:
		adIds := events.GetNestedStringSlice(e.Document, "panCakeData", "ad_ids")
		if len(adIds) == 0 {
			adIds = events.GetNestedStringSlice(e.Document, "PanCakeData", "ad_ids")
		}
		if len(adIds) == 0 {
			return
		}
		for _, adId := range adIds {
			if adId == "" {
				continue
			}
			adAccountId := getAdAccountIdByAdId(ctx, adId, ownerOrgID)
			if adAccountId == "" {
				continue
			}
			aid, acc := adId, adAccountId
			key := entityKey("ad", aid, acc, ownerOrgID, "pancake.conversation")
			scheduleAdsRecompute(key, func() {
				bg := context.Background()
				_ = metasvc.UpdateRawFromSource(bg, "ad", aid, acc, ownerOrgID, "pancake.conversation")
				triggerRollUpForAd(bg, aid, acc, ownerOrgID)
			})
		}
		return

	default:
		return
	}
}

// triggerRollUpForAd sau khi Ad cập nhật, gọi roll-up cho AdSet → Campaign → Account.
func triggerRollUpForAd(ctx context.Context, adId, adAccountId string, ownerOrgID primitive.ObjectID) {
	adSetId, campaignId := getAdSetAndCampaignByAdId(ctx, adId, ownerOrgID)
	if adSetId != "" {
		_ = metasvc.RecalculateForEntity(ctx, "adset", adSetId, adAccountId, ownerOrgID)
	}
	if campaignId != "" {
		_ = metasvc.RecalculateForEntity(ctx, "campaign", campaignId, adAccountId, ownerOrgID)
	}
	if adAccountId != "" {
		_ = metasvc.RecalculateForEntity(ctx, "ad_account", adAccountId, adAccountId, ownerOrgID)
	}
}

// getAdSetAndCampaignByAdId lấy adSetId, campaignId từ meta_ads theo adId.
func getAdSetAndCampaignByAdId(ctx context.Context, adId string, ownerOrgID primitive.ObjectID) (adSetId, campaignId string) {
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

// getAdAccountIdByAdId tìm adAccountId từ meta_ads theo adId.
func getAdAccountIdByAdId(ctx context.Context, adId string, ownerOrgID primitive.ObjectID) string {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return ""
	}
	filter := bson.M{
		"adId":                 adId,
		"ownerOrganizationId": ownerOrgID,
	}
	var doc struct {
		AdAccountId string `bson:"adAccountId"`
	}
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		return ""
	}
	return doc.AdAccountId
}
