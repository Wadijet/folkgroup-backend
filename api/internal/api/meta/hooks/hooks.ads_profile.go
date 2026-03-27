// Package metahooks — Debounce + emit ads.intelligence.recompute_requested (currentMetrics 4 level).
// meta_ad_insights: chỉ số bất thường (IsUrgentMetaInsightDataChange) → recompute ngay; không thì gom 15 phút.
// Đơn/hội thoại: debounce ngắn (adsintel.DebounceMs). AI Decision consumer gọi ProcessDataChangeForAdsProfile sau datachanged.
package metahooks

import (
	"context"

	"meta_commerce/internal/adsintel"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// emitAdsIntelligenceRecomputeQueued đưa yêu cầu tính lại (mode source) vào decision_events_queue.
func emitAdsIntelligenceRecomputeQueued(bg context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, source string) {
	_, _ = aidecisionsvc.EmitAdsIntelligenceRecomputeRequested(bg, objectType, objectId, adAccountId, ownerOrgID, source, "")
}

// ProcessDataChangeForAdsProfile debounce rồi emit tính lại currentMetrics (meta_ad_insights, pc_pos_orders, fb_conversations).
func ProcessDataChangeForAdsProfile(ctx context.Context, e events.DataChangeEvent) {
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
		// Gấp: chỉ số bất thường → recompute ngay; không gấp → gom 15 phút (trailing debounce).
		debounceMs := adsintel.DebounceMsInsightBatch
		if IsUrgentMetaInsightDataChange(e) {
			debounceMs = 0
		}
		if objectType == "ad" {
			key := entityKey("ad", objectId, adAccountId, ownerOrgID, "meta")
			scheduleAdsRecompute(key, debounceMs, func() {
				bg := context.Background()
				emitAdsIntelligenceRecomputeQueued(bg, objectType, objectId, adAccountId, ownerOrgID, "meta")
			})
		} else {
			key := entityKey(objectType, objectId, adAccountId, ownerOrgID, "meta")
			scheduleAdsRecompute(key, debounceMs, func() {
				bg := context.Background()
				emitAdsIntelligenceRecomputeQueued(bg, objectType, objectId, adAccountId, ownerOrgID, "meta")
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
		scheduleAdsRecompute(key, adsintel.DebounceMs, func() {
			bg := context.Background()
			emitAdsIntelligenceRecomputeQueued(bg, "ad", adId, adAccountId, ownerOrgID, "pancake.pos")
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
			scheduleAdsRecompute(key, adsintel.DebounceMs, func() {
				bg := context.Background()
				emitAdsIntelligenceRecomputeQueued(bg, "ad", aid, acc, ownerOrgID, "pancake.conversation")
			})
		}
		return

	default:
		return
	}
}

// getAdAccountIdByAdId tìm adAccountId từ meta_ads theo adId.
func getAdAccountIdByAdId(ctx context.Context, adId string, ownerOrgID primitive.ObjectID) string {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return ""
	}
	filter := bson.M{
		"adId":                adId,
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
