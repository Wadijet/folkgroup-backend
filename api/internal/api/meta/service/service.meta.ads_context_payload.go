// Package metasvc — Đọc snapshot Intelligence Ads từ meta_campaigns (chỉ worker domain ads / ads_intel_compute).
package metasvc

import (
	"context"
	"strings"
	"time"

	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// expandAdAccountIDForMetaCampaignFilter meta_campaigns có thể lưu adAccountId dạng act_XXX hoặc số.
func expandAdAccountIDForMetaCampaignFilter(adAccountID string) bson.M {
	if adAccountID == "" {
		return bson.M{}
	}
	if strings.HasPrefix(adAccountID, "act_") {
		return bson.M{"$in": bson.A{adAccountID, strings.TrimPrefix(adAccountID, "act_")}}
	}
	return bson.M{"$in": bson.A{adAccountID, "act_" + adAccountID}}
}

// BuildAdsIntelligenceContextPayloadFromDB đọc currentMetrics từ meta_campaigns (nguồn sau rollup Intelligence).
// Chỉ gọi từ worker domain (RunAdsIntelComputeJob khi jobKind=context_ready), không gọi từ consumer AI Decision.
func BuildAdsIntelligenceContextPayloadFromDB(ctx context.Context, campaignID, adAccountID string, ownerOrgID primitive.ObjectID) map[string]interface{} {
	nowMs := time.Now().UnixMilli()
	out := map[string]interface{}{
		"campaignId":    campaignID,
		"adAccountId":   adAccountID,
		"source":        "ads_intelligence",
		"evaluatedAtMs": nowMs,
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		out["found"] = false
		out["reason"] = "không có collection meta_campaigns"
		return out
	}
	filter := bson.M{
		"campaignId":          campaignID,
		"ownerOrganizationId": ownerOrgID,
	}
	if adAccountID != "" {
		filter["adAccountId"] = expandAdAccountIDForMetaCampaignFilter(adAccountID)
	}
	var doc struct {
		AdAccountID    string                 `bson:"adAccountId"`
		Name           string                 `bson:"name"`
		CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
	}
	err := coll.FindOne(ctx, filter, mongoopts.FindOne().SetProjection(bson.M{
		"adAccountId": 1, "name": 1, "currentMetrics": 1,
	})).Decode(&doc)
	if err != nil || doc.CurrentMetrics == nil {
		out["found"] = false
		out["reason"] = "không tìm thấy campaign hoặc chưa có currentMetrics (Intelligence)"
		return out
	}
	cm := doc.CurrentMetrics
	raw, _ := cm["raw"].(map[string]interface{})
	layer1, _ := cm["layer1"].(map[string]interface{})
	layer2, _ := cm["layer2"].(map[string]interface{})
	layer3, _ := cm["layer3"].(map[string]interface{})
	out["found"] = true
	out["campaignName"] = doc.Name
	if doc.AdAccountID != "" {
		out["adAccountId"] = doc.AdAccountID
	}
	out["raw"] = raw
	out["layer1"] = layer1
	out["layer2"] = layer2
	out["layer3"] = layer3
	out["alertFlags"] = cm["alertFlags"]
	out["flags"] = alertFlagsAsFlagMaps(cm["alertFlags"])
	return out
}

func alertFlagsAsFlagMaps(v interface{}) []map[string]interface{} {
	var codes []string
	switch x := v.(type) {
	case []string:
		codes = x
	case []interface{}:
		for _, e := range x {
			if s, ok := e.(string); ok && s != "" {
				codes = append(codes, s)
			}
		}
	default:
		return nil
	}
	if len(codes) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(codes))
	for _, code := range codes {
		out = append(out, map[string]interface{}{
			"name":     code,
			"severity": "info",
			"source":   "alertFlags",
		})
	}
	return out
}
