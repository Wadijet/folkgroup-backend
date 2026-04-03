// Package metahooks — Debounce + emit ads.intelligence.recompute_requested (currentMetrics 4 level).
// AI Decision consumer gọi ProcessDataChangeForAdsProfile sau datachanged (applyDatachangedSideEffects).
//
// Luồng 1 **nhóm Ads** theo tài liệu kiến trúc + filter L2: chỉ **meta_ad_insights** vào queue từ Meta (xem datachanged_emit_filter meta_insight_only).
// **Urgent** (bỏ debounce, emit recompute ngay): **chỉ** áp cho bản ghi insight — IsUrgentMetaInsightDataChange.
//
// Đơn POS / hội thoại có ad_id / ad_ids: vẫn có thể xếp cùng job recompute (attribution) qua đây, nhưng đó là **tín hiệu từ collection đơn/hội thoại**,
// không phải “datachanged Ads = insight”; luôn Urgent=false — chỉ debounce/throttle campaign như bình thường.
package metahooks

import (
	"context"
	"strings"

	"meta_commerce/internal/api/events"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ProcessDataChangeForAdsProfile — throttle theo campaign (tối thiểu 15 phút/lần) rồi emit recompute full.
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
		urgent := IsUrgentMetaInsightDataChange(e)
		recalcOT, recalcOID, acc, ok := resolveRecalcTargetForInsight(ctx, objectType, objectId, adAccountId, ownerOrgID)
		if !ok {
			return
		}
		enqueueCampaignIntelRecomputeDebounced(ctx, &campaignIntelDebounceRequest{
			OwnerOrgID:       ownerOrgID,
			AdAccountID:      acc,
			RecalcObjectType: recalcOT,
			RecalcObjectID:   recalcOID,
			SourceKind:       "meta_ad_insights",
			Urgent:           urgent,
		})
		return

	case global.MongoDB_ColNames.PcPosOrders:
		adId := events.GetNestedStringField(e.Document, "posData", "ad_id")
		if adId == "" {
			adId = events.GetNestedStringField(e.Document, "PosData", "ad_id")
		}
		if adId == "" {
			return
		}
		campaignId, adAccountId, ok := getCampaignAndAdAccountFromAdId(ctx, adId, ownerOrgID)
		if !ok {
			return
		}
		enqueueCampaignIntelRecomputeDebounced(ctx, &campaignIntelDebounceRequest{
			OwnerOrgID:       ownerOrgID,
			AdAccountID:      adAccountId,
			RecalcObjectType: "campaign",
			RecalcObjectID:   campaignId,
			SourceKind:       "pc_pos_orders",
			Urgent:           false,
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
		seen := make(map[string]struct{})
		for _, adId := range adIds {
			adId = strings.TrimSpace(adId)
			if adId == "" {
				continue
			}
			campaignId, adAccountId, ok := getCampaignAndAdAccountFromAdId(ctx, adId, ownerOrgID)
			if !ok {
				continue
			}
			sk := CampaignIntelThrottleKey(ownerOrgID, adAccountId, "campaign", campaignId)
			if _, dup := seen[sk]; dup {
				continue
			}
			seen[sk] = struct{}{}
			enqueueCampaignIntelRecomputeDebounced(ctx, &campaignIntelDebounceRequest{
				OwnerOrgID:       ownerOrgID,
				AdAccountID:      adAccountId,
				RecalcObjectType: "campaign",
				RecalcObjectID:   campaignId,
				SourceKind:       "fb_conversations",
				Urgent:           false,
			})
		}
		return

	default:
		return
	}
}

// resolveRecalcTargetForInsight map objectType insight → entity cho RecalculateForEntity + khóa throttle.
func resolveRecalcTargetForInsight(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID) (recalcOT, recalcOID, acc string, ok bool) {
	ot := strings.TrimSpace(strings.ToLower(objectType))
	switch ot {
	case "campaign":
		return "campaign", objectId, adAccountId, true
	case "ad":
		cid, acc2, ok2 := getCampaignAndAdAccountFromAdId(ctx, objectId, ownerOrgID)
		if !ok2 {
			return "", "", "", false
		}
		return "campaign", cid, acc2, true
	case "adset":
		cid, acc2, ok2 := getCampaignAndAdAccountFromAdSetId(ctx, objectId, ownerOrgID)
		if !ok2 {
			return "", "", "", false
		}
		return "campaign", cid, acc2, true
	case "ad_account":
		return "ad_account", adAccountId, adAccountId, true
	default:
		// Entity lạ: vẫn throttle theo cặp type|id, full recalc trực tiếp
		return objectType, objectId, adAccountId, true
	}
}

func getCampaignAndAdAccountFromAdId(ctx context.Context, adId string, ownerOrgID primitive.ObjectID) (campaignId, adAccountId string, ok bool) {
	coll, okc := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !okc {
		return "", "", false
	}
	filter := bson.M{
		"adId":                adId,
		"ownerOrganizationId": ownerOrgID,
	}
	var doc struct {
		CampaignId  string `bson:"campaignId"`
		AdAccountId string `bson:"adAccountId"`
	}
	if err := coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		return "", "", false
	}
	if strings.TrimSpace(doc.CampaignId) == "" || strings.TrimSpace(doc.AdAccountId) == "" {
		return "", "", false
	}
	return doc.CampaignId, doc.AdAccountId, true
}

func getCampaignAndAdAccountFromAdSetId(ctx context.Context, adSetId string, ownerOrgID primitive.ObjectID) (campaignId, adAccountId string, ok bool) {
	coll, okc := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdSets)
	if !okc {
		return "", "", false
	}
	filter := bson.M{
		"adSetId":             adSetId,
		"ownerOrganizationId": ownerOrgID,
	}
	var doc struct {
		CampaignId  string `bson:"campaignId"`
		AdAccountId string `bson:"adAccountId"`
	}
	if err := coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		return "", "", false
	}
	if strings.TrimSpace(doc.CampaignId) == "" || strings.TrimSpace(doc.AdAccountId) == "" {
		return "", "", false
	}
	return doc.CampaignId, doc.AdAccountId, true
}
