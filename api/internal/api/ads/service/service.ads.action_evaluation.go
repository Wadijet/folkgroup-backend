// Package adssvc — Logic tính action từ flags. Gọi từ worker.
// Server rollup chỉ tạo flags; worker gọi ComputeActionsFromMetrics → cập nhật currentMetrics.actions + ghi activity.
package adssvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	metasvc "meta_commerce/internal/api/meta/service"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// expandAdAccountIdForFilter trả về filter cho adAccountId (meta_campaigns có thể "act_XXX" hoặc "XXX").
func expandAdAccountIdForFilter(adAccountId string) interface{} {
	if adAccountId == "" {
		return adAccountId
	}
	if strings.HasPrefix(adAccountId, "act_") {
		return bson.M{"$in": bson.A{adAccountId, strings.TrimPrefix(adAccountId, "act_")}}
	}
	return bson.M{"$in": bson.A{adAccountId, "act_" + adAccountId}}
}

// ComputeActionsFromMetrics tính actions từ currentMetrics (raw, layer1, layer2, layer3, alertFlags).
// Gọi metasvc.ComputeFinalActions — logic đầy đủ lifecycle, rules, noon cut, dual-source, chs.
// Trả về (actions, actionDebugReport, nil) khi có actions; (nil, nil, nil) khi không.
func ComputeActionsFromMetrics(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, currentMetrics map[string]interface{}) (actions []map[string]interface{}, report map[string]interface{}) {
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
	flags := ParseAlertFlags(alertFlagsRaw)
	if len(flags) == 0 {
		return nil, nil
	}
	alertFlags := make([]string, len(flags))
	for i, f := range flags {
		if s, ok := f.(string); ok {
			alertFlags[i] = s
		}
	}
	cfg, _ := GetCampaignConfig(ctx, adAccountId, ownerOrgID)
	actions, report = metasvc.ComputeFinalActions(ctx, campaignId, adAccountId, ownerOrgID, alertFlags, raw, layer1, layer2, layer3, cfg)
	return actions, report
}

// UpdateCampaignActionsAndRecordActivity cập nhật campaign.currentMetrics.actions và ghi activity với actionDebugReport.
// Gọi sau khi ComputeActionsFromMetrics trả về actions.
func UpdateCampaignActionsAndRecordActivity(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, currentMetrics map[string]interface{}, actions []map[string]interface{}, actionDebugReport map[string]interface{}) error {
	if currentMetrics == nil || len(actions) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return fmt.Errorf("không tìm thấy collection meta_campaigns")
	}
	old := copyMap(currentMetrics)
	current := copyMap(currentMetrics)
	current["actions"] = actions

	filter := bson.M{
		"campaignId":          campaignId,
		"ownerOrganizationId": ownerOrgID,
	}
	if adAccountId != "" {
		filter["adAccountId"] = expandAdAccountIdForFilter(adAccountId)
	}

	// Ghi activity trước (trigger=auto_propose, có actionDebugReport)
	metadata := map[string]interface{}{"actionDebugReport": actionDebugReport}
	metasvc.RecordActivityForEntity(ctx, "campaign", campaignId, adAccountId, ownerOrgID, old, current, "auto_propose", metadata)

	// Cập nhật currentMetrics
	_, err := coll.UpdateOne(ctx, filter, bson.M{
		"$set": bson.M{
			"currentMetrics": current,
			"updatedAt":      time.Now().UnixMilli(),
		},
	})
	return err
}

// copyMap tạo bản sao nông của map (đủ cho diff).
func copyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

