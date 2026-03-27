// Package metasvc — ACTION_RULE từ snapshot currentMetrics (alertFlags + layers).
// Ads Intelligence (rollup) chỉ ghi raw/layer/alertFlags; không tính đề xuất hành động.
// AI Decision / worker adsautop gọi ComputeFinalActionsFromCurrentMetrics + PersistCampaignEvaluatedActions.
package metasvc

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// parseAlertFlagsFromCurrentMetrics chuẩn hóa alertFlags từ BSON (cùng logic adssvc.ParseAlertFlags, tránh import cycle).
func parseAlertFlagsFromCurrentMetrics(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []interface{}:
		return val
	case []string:
		out := make([]interface{}, len(val))
		for i, s := range val {
			out[i] = s
		}
		return out
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil
	}
	var out []interface{}
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i).Interface()
		if s, ok := elem.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// expandAdAccountIdForMetaFilter filter adAccountId (act_XXX / số) cho meta_campaigns.
func expandAdAccountIdForMetaFilter(adAccountId string) interface{} {
	if adAccountId == "" {
		return adAccountId
	}
	if strings.HasPrefix(adAccountId, "act_") {
		return bson.M{"$in": bson.A{adAccountId, strings.TrimPrefix(adAccountId, "act_")}}
	}
	return bson.M{"$in": bson.A{adAccountId, "act_" + adAccountId}}
}

func copyMetricsMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ComputeFinalActionsFromCurrentMetrics áp ACTION_RULE (ruleintel) lên snapshot Intelligence đã có alertFlags.
// Trả về đề xuất hành động sau lifecycle / dual-source / noon cut (FolkForm v4.1).
func ComputeFinalActionsFromCurrentMetrics(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, currentMetrics map[string]interface{}) (actions []map[string]interface{}, report map[string]interface{}) {
	if currentMetrics == nil {
		return nil, nil
	}
	raw, _ := currentMetrics["raw"].(map[string]interface{})
	layer1, _ := currentMetrics["layer1"].(map[string]interface{})
	layer2, _ := currentMetrics["layer2"].(map[string]interface{})
	layer3, _ := currentMetrics["layer3"].(map[string]interface{})
	alertFlagsRaw := currentMetrics["alertFlags"]
	if alertFlagsRaw == nil {
		return nil, nil
	}
	flags := parseAlertFlagsFromCurrentMetrics(alertFlagsRaw)
	if len(flags) == 0 {
		return nil, nil
	}
	alertFlags := make([]string, 0, len(flags))
	for _, f := range flags {
		if s, ok := f.(string); ok && s != "" {
			alertFlags = append(alertFlags, s)
		}
	}
	if len(alertFlags) == 0 {
		return nil, nil
	}
	cfg, _ := adsconfig.GetConfigForCampaign(ctx, adAccountId, ownerOrgID)
	return ComputeFinalActions(ctx, campaignId, adAccountId, ownerOrgID, alertFlags, raw, layer1, layer2, layer3, cfg)
}

// PersistCampaignEvaluatedActions cập nhật currentMetrics.actions và ghi ads_activity (trigger=auto_propose).
// Gọi sau ComputeFinalActionsFromCurrentMetrics khi có actions — lớp AI Decision / adsautop điều phối.
func PersistCampaignEvaluatedActions(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, currentMetrics map[string]interface{}, actions []map[string]interface{}, actionDebugReport map[string]interface{}) error {
	if currentMetrics == nil || len(actions) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return fmt.Errorf("không tìm thấy collection meta_campaigns")
	}
	old := copyMetricsMap(currentMetrics)
	current := copyMetricsMap(currentMetrics)
	current["actions"] = actions

	filter := bson.M{
		"campaignId":          campaignId,
		"ownerOrganizationId": ownerOrgID,
	}
	if adAccountId != "" {
		filter["adAccountId"] = expandAdAccountIdForMetaFilter(adAccountId)
	}

	metadata := map[string]interface{}{"actionDebugReport": actionDebugReport}
	RecordActivityForEntity(ctx, "campaign", campaignId, adAccountId, ownerOrgID, old, current, "auto_propose", metadata)

	_, err := coll.UpdateOne(ctx, filter, bson.M{
		"$set": bson.M{
			"currentMetrics": current,
			"updatedAt":      time.Now().UnixMilli(),
		},
	})
	return err
}
