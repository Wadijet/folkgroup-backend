// Package adssvc — AutoPropose: hệ thống tự tạo đề xuất dựa trên alertFlags (FolkForm Master Rules v4.1).
package adssvc

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/approval"
	"meta_commerce/internal/global"
	metasvc "meta_commerce/internal/api/meta/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// ParseAlertFlags chuyển alertFlags (từ currentMetrics) sang []interface{} — BSON có thể decode thành primitive.A, []string, v.v.
func ParseAlertFlags(v interface{}) []interface{} {
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
	// primitive.A, []primitive.E hoặc slice khác — dùng reflect
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

// expandAdAccountIdsForFilter mở rộng adAccountIds để match cả "act_XXX" và "XXX" — meta_campaigns có thể lưu cả hai format.
func expandAdAccountIdsForFilter(adAccountIds []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, id := range adAccountIds {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
		if strings.HasPrefix(id, "act_") {
			trimmed := strings.TrimPrefix(id, "act_")
			if !seen[trimmed] {
				seen[trimmed] = true
				out = append(out, trimmed)
			}
		} else if id != "" {
			withAct := "act_" + id
			if !seen[withAct] {
				seen[withAct] = true
				out = append(out, withAct)
			}
		}
	}
	return out
}

// MinCampaignAgeDays số ngày tối thiểu để campaign được đưa vào auto propose (FolkForm v4.1: camp mới < 7 ngày bỏ qua).
const MinCampaignAgeDays = 7

// isCampaignNew kiểm tra campaign có phải camp mới (< 7 ngày) không. Camp mới bỏ qua auto propose (FolkForm v4.1).
// Ưu tiên lifecycle từ layer1; fallback dùng metaCreatedAt khi chưa có currentMetrics.
func isCampaignNew(currentMetrics map[string]interface{}, metaCreatedAt int64) bool {
	if layer1, ok := currentMetrics["layer1"].(map[string]interface{}); ok {
		if lc, ok := layer1["lifecycle"].(string); ok && lc == "NEW" {
			return true
		}
	}
	if metaCreatedAt > 0 {
		daysSinceCreated := (time.Now().UnixMilli() - metaCreatedAt) / (24 * 60 * 60 * 1000)
		return daysSinceCreated < MinCampaignAgeDays
	}
	return false
}

// GetCampaignsForAutoPropose lấy campaigns ACTIVE có currentMetrics.alertFlags, thuộc ad accounts có autoProposeEnabled.
// Bỏ qua campaign mới (lifecycle=NEW hoặc metaCreatedAt < 7 ngày) — theo FolkForm v4.1 Per-Camp Adaptive Threshold giai đoạn 0.
// Công tắc 1: autoProposeEnabled (account) — bật mới lấy campaigns. Công tắc 2: ActionRules[].autoPropose (từng rule).
func GetCampaignsForAutoPropose(ctx context.Context, limit int) ([]CampaignForEval, error) {
	if limit <= 0 {
		limit = 20
	}
	// 1. Lấy ad accounts có autoProposeEnabled = true (hoặc không set = mặc định true)
	configColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsMetaConfig)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection ads_meta_config")
	}
	cursor, err := configColl.Find(ctx, bson.M{
		"$or": []bson.M{
			{"account.automationConfig.autoProposeEnabled": true},
			{"account.automationConfig.autoProposeEnabled": bson.M{"$exists": false}},
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var configs []struct {
		AdAccountId         string              `bson:"adAccountId"`
		OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
	}
	if err := cursor.All(ctx, &configs); err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		return []CampaignForEval{}, nil
	}
	// 2. Build filter: adAccountId in [...], ownerOrgId in [...], status ACTIVE, có currentMetrics.alertFlags
	// meta_campaigns có thể lưu adAccountId "act_XXX" hoặc "XXX" — cần match cả hai format (giống metasvc.adAccountIdFilterForMeta)
	rawIds := make([]string, 0, len(configs))
	for _, c := range configs {
		rawIds = append(rawIds, c.AdAccountId)
	}
	adAccountIds := expandAdAccountIdsForFilter(rawIds)
	orgIds := make([]primitive.ObjectID, 0, len(configs))
	for _, c := range configs {
		orgIds = append(orgIds, c.OwnerOrganizationID)
	}
	campaignsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_campaigns")
	}
	// PATCH 00: Chỉ lấy campaign Purchase Through Messaging (objective OUTCOME_SALES hoặc MESSAGES).
	scopeObj := adsconfig.ScopeFilterPurchaseMessaging()
	filter := bson.M{
		"adAccountId":                   bson.M{"$in": adAccountIds},
		"ownerOrganizationId":           bson.M{"$in": orgIds},
		"currentMetrics.alertFlags.0":    bson.M{"$exists": true},
		"$and": bson.A{
			bson.M{"$or": []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}}},
			scopeObj,
		},
	}
	opts := mongoopts.Find().SetLimit(int64(limit)).SetProjection(bson.M{
		"campaignId": 1, "adAccountId": 1, "name": 1, "ownerOrganizationId": 1,
		"currentMetrics.alertFlags": 1, "currentMetrics.layer1": 1, "metaCreatedAt": 1, "metaData": 1,
	})
	cur, err := campaignsColl.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []CampaignForEval
	for cur.Next(ctx) {
		var doc struct {
			CampaignId          string                 `bson:"campaignId"`
			AdAccountId         string                 `bson:"adAccountId"`
			Name                string                 `bson:"name"`
			OwnerOrganizationID primitive.ObjectID     `bson:"ownerOrganizationId"`
			MetaCreatedAt       int64                  `bson:"metaCreatedAt"`
			CurrentMetrics      map[string]interface{} `bson:"currentMetrics"`
		}
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		// Bỏ qua campaign mới (< 7 ngày) — FolkForm v4.1: giai đoạn 0 dùng global threshold, chưa đủ data để auto propose.
		if isCampaignNew(doc.CurrentMetrics, doc.MetaCreatedAt) {
			continue
		}
		alertFlags, _ := doc.CurrentMetrics["alertFlags"]
		out = append(out, CampaignForEval{
			CampaignId:          doc.CampaignId,
			AdAccountId:         doc.AdAccountId,
			CampaignName:        doc.Name,
			OwnerOrganizationID: doc.OwnerOrganizationID,
			AlertFlags:          alertFlags,
		})
	}
	return out, nil
}

// CampaignForEval campaign cần đánh giá.
type CampaignForEval struct {
	CampaignId          string
	AdAccountId         string
	CampaignName        string
	OwnerOrganizationID primitive.ObjectID
	AlertFlags          interface{}
}

// GetKillRulesEnabled đọc công tắc kill rules từ ads_meta_config. FALSE → skip SL-D, SL-E, CHS Kill, KO-B (vd: Pancake down).
func GetKillRulesEnabled(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) bool {
	return adsconfig.GetKillRulesEnabled(ctx, adAccountId, ownerOrgID)
}

// HasPendingProposalForCampaign kiểm tra đã có đề xuất pending cho campaign chưa.
func HasPendingProposalForCampaign(ctx context.Context, campaignId string, ownerOrgID primitive.ObjectID) (bool, error) {
	info, err := GetPendingProposalForCampaign(ctx, campaignId, ownerOrgID)
	return err == nil && info != nil, err
}

// PendingProposalInfo thông tin đề xuất đang chờ duyệt (để so sánh với kết quả đánh giá mới).
type PendingProposalInfo struct {
	ID         primitive.ObjectID
	ActionType string
	RuleCode   string
}

// GetChsFromYesterday lấy CHS từ ads_activity_history — bản ghi mới nhất của campaign trong ngày hôm qua (FolkForm v4.1 CHS Kill exception).
// Camp HEALTHY hôm qua (CHS >= 60) mà hôm nay CHS critical đột ngột → có thể data anomaly → chờ 1 checkpoint.
// Trả về (chs, ok). ok=false khi không có dữ liệu.
func GetChsFromYesterday(ctx context.Context, campaignId string, ownerOrgID primitive.ObjectID) (float64, bool) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsActivityHistory)
	if !ok {
		return 0, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	yesterdayStart := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, loc).UnixMilli()
	yesterdayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).UnixMilli()
	opts := mongoopts.FindOne().SetSort(bson.D{{Key: "activityAt", Value: -1}}).SetProjection(bson.M{"snapshot.metrics": 1})
	var doc struct {
		Snapshot struct {
			Metrics map[string]interface{} `bson:"metrics"`
		} `bson:"snapshot"`
	}
	err := coll.FindOne(ctx, bson.M{
		"objectType":          "campaign",
		"objectId":            campaignId,
		"ownerOrganizationId": ownerOrgID,
		"activityAt":          bson.M{"$gte": yesterdayStart, "$lt": yesterdayEnd},
	}, opts).Decode(&doc)
	if err != nil {
		return 0, false
	}
	layer3, _ := doc.Snapshot.Metrics["layer3"].(map[string]interface{})
	if layer3 == nil {
		return 0, false
	}
	chs := toFloat64FromInterface(layer3["chs"])
	return chs, true
}

// RequireDualSourceConfirm trả về true nếu rule cần xác nhận dual-source (FB + Pancake) trước kill (FolkForm v4.1 PATCH 05).
// Cả 2 nguồn xấu → kill. Chỉ 1 xấu (Pancake có đơn) → chờ 1 checkpoint (attribution gap).
var dualSourceKillRules = map[string]bool{
	"sl_b": true, "sl_d": true, "ko_b": true,
	"mess_trap_suspect": true, "mess_trap_confirmed": true,
}

// getPancakeOrdersFromCurrentMetrics trích orders_2h, orders_7d từ currentMetrics.raw (Pancake).
// raw.2h.orders = orders 2h; raw.7d.pancake.pos.orders = orders 7d.
func getPancakeOrdersFromCurrentMetrics(currentMetrics map[string]interface{}) (orders2h, orders7d float64) {
	raw, _ := currentMetrics["raw"].(map[string]interface{})
	if raw == nil {
		return 0, 0
	}
	r2h, _ := raw["2h"].(map[string]interface{})
	if r2h != nil {
		orders2h = toFloat64FromInterface(r2h["orders"])
	}
	r7d, _ := raw["7d"].(map[string]interface{})
	if r7d != nil {
		pancake, _ := r7d["pancake"].(map[string]interface{})
		if pancake != nil {
			pos, _ := pancake["pos"].(map[string]interface{})
			if pos != nil {
				orders7d = toFloat64FromInterface(pos["orders"])
			}
		}
	}
	return orders2h, orders7d
}

func toFloat64FromInterface(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

// GetCampaignCurrentMetrics lấy currentMetrics từ campaign. Trả về nil nếu không tìm thấy. Export cho diagnose.
func GetCampaignCurrentMetrics(ctx context.Context, campaignsColl *mongo.Collection, campaignId string, ownerOrgID primitive.ObjectID) map[string]interface{} {
	var doc struct {
		CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
	}
	err := campaignsColl.FindOne(ctx, bson.M{
		"campaignId":          campaignId,
		"ownerOrganizationId": ownerOrgID,
	},
		mongoopts.FindOne().SetProjection(bson.M{"currentMetrics": 1}),
	).Decode(&doc)
	if err != nil {
		return nil
	}
	return doc.CurrentMetrics
}

// buildMetricsPayloadForNotification format currentMetrics thành payload cho notification.
// currentMetrics nil → fetch từ DB. Trả về map với keys: rawSummary, layer1Summary, flagsSummary, flagsDetail.
func buildMetricsPayloadForNotification(ctx context.Context, campaignsColl *mongo.Collection, campaignId string, adAccountId string, ownerOrgID primitive.ObjectID, currentMetrics map[string]interface{}) map[string]interface{} {
	if currentMetrics == nil {
		currentMetrics = GetCampaignCurrentMetrics(ctx, campaignsColl, campaignId, ownerOrgID)
	}
	if currentMetrics == nil {
		return nil
	}
	cfg, _ := GetCampaignConfig(ctx, adAccountId, ownerOrgID)
	summaries := FormatMetricsForNotificationWithConfig(ctx, currentMetrics, cfg)
	payload := make(map[string]interface{})
	for k, v := range summaries {
		payload[k] = v
	}
	return payload
}

// GetPendingProposalForCampaign lấy đề xuất pending cho campaign (nếu có). Dùng để hủy khi cần.
// Trả về (nil, nil) khi không có pending; (nil, err) khi lỗi; (info, nil) khi có pending.
func GetPendingProposalForCampaign(ctx context.Context, campaignId string, ownerOrgID primitive.ObjectID) (*PendingProposalInfo, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection action_pending_approval")
	}
	var doc struct {
		ID          primitive.ObjectID     `bson:"_id"`
		ActionType  string                 `bson:"actionType"`
		Payload     map[string]interface{} `bson:"payload"`
	}
	err := coll.FindOne(ctx, bson.M{
		"domain":               "ads",
		"status":               "pending",
		"ownerOrganizationId":  ownerOrgID,
		"payload.campaignId":   campaignId,
	}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	ruleCode, _ := doc.Payload["ruleCode"].(string)
	return &PendingProposalInfo{ID: doc.ID, ActionType: doc.ActionType, RuleCode: ruleCode}, nil
}

// increaseCandidate ứng viên tăng budget — dùng cho Anti Self-Competition (FolkForm v4.1 Section 06).
type increaseCandidate struct {
	c              CampaignForEval
	action         map[string]interface{}
	actions        []map[string]interface{}
	report         map[string]interface{}
	currentMetrics map[string]interface{}
	metaCfg        *adsmodels.CampaignConfigView
	cpaPurchase    float64
	chs            float64
	mqs            float64
	frequency      float64
}

// RunAutoPropose đánh giá campaigns và tạo đề xuất khi rule trigger.
// Dùng ComputeActionsFromMetrics (metasvc.ComputeFinalActions) — logic đầy đủ trong ads module.
// Anti Self-Competition: Increase chỉ cho top N camp/account (PROTECT:1, EFFICIENCY:1, NORMAL:2, BLITZ:3).
func RunAutoPropose(ctx context.Context, baseURL string) (proposed int, err error) {
	campaigns, err := GetCampaignsForAutoPropose(ctx, 30)
	if err != nil {
		return 0, err
	}
	campaignsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, fmt.Errorf("không tìm thấy collection meta_campaigns")
	}
	campaignConfigByAccount := make(map[string]*adsmodels.CampaignConfigView)
	increaseCandidatesByAccount := make(map[string][]increaseCandidate)

	for _, c := range campaigns {
		cacheKey := c.AdAccountId + "|" + c.OwnerOrganizationID.Hex()
		if _, ok := campaignConfigByAccount[cacheKey]; !ok {
			cfg, _ := GetCampaignConfig(ctx, c.AdAccountId, c.OwnerOrganizationID)
			campaignConfigByAccount[cacheKey] = cfg
		}
		metaCfg := campaignConfigByAccount[cacheKey]

		pendingInfo, pendingErr := GetPendingProposalForCampaign(ctx, c.CampaignId, c.OwnerOrganizationID)
		if pendingErr != nil {
			continue
		}
		hasPending := pendingInfo != nil

		// Recalculate nếu có pending (để lấy metrics mới nhất)
		if hasPending {
			if recalcErr := metasvc.RecalculateForEntity(ctx, "campaign", c.CampaignId, c.AdAccountId, c.OwnerOrganizationID); recalcErr != nil {
				continue
			}
		}

		currentMetrics := GetCampaignCurrentMetrics(ctx, campaignsColl, c.CampaignId, c.OwnerOrganizationID)
		if currentMetrics == nil {
			if hasPending {
				_, _ = approval.Cancel(ctx, pendingInfo.ID.Hex(), c.OwnerOrganizationID)
			}
			continue
		}

		// Gọi logic tính action (metasvc.ComputeFinalActions) — lifecycle, rules, noon cut, dual-source, chs
		actions, report := ComputeActionsFromMetrics(ctx, c.CampaignId, c.AdAccountId, c.OwnerOrganizationID, currentMetrics)

		if len(actions) == 0 {
			if hasPending {
				_, _ = approval.Cancel(ctx, pendingInfo.ID.Hex(), c.OwnerOrganizationID)
			}
			continue
		}
		action := actions[0]
		actionType, _ := action["actionType"].(string)
		ruleCode, _ := action["ruleCode"].(string)

		// Increase: thu thập ứng viên, xử lý sau (Anti Self-Competition — top N)
		if actionType == "INCREASE" {
			if IsSelfCompetitionSuspect(ctx, c.AdAccountId, c.OwnerOrganizationID) {
				continue
			}
			layer1, _ := currentMetrics["layer1"].(map[string]interface{})
			layer3, _ := currentMetrics["layer3"].(map[string]interface{})
			raw, _ := currentMetrics["raw"].(map[string]interface{})
			r7d, _ := raw["7d"].(map[string]interface{})
			meta7d, _ := r7d["meta"].(map[string]interface{})
			orders, spend := 0.0, 0.0
			if p, _ := r7d["pancake"].(map[string]interface{}); p != nil {
				if pos, _ := p["pos"].(map[string]interface{}); pos != nil {
					orders = toFloat64FromInterface(pos["orders"])
				}
			}
			if meta7d != nil {
				spend = toFloat64FromInterface(meta7d["spend"])
			}
			cpaPurchase := 0.0
			if orders > 0 {
				cpaPurchase = spend / orders
			}
			cand := increaseCandidate{
				c: c, action: action, actions: actions, report: report,
				currentMetrics: currentMetrics, metaCfg: metaCfg,
				cpaPurchase: cpaPurchase,
				chs:        toFloat64FromInterface(layer3["chs"]),
				mqs:        toFloat64FromInterface(layer1["mqs_7d"]),
				frequency:  toFloat64FromInterface(meta7d["frequency"]),
			}
			increaseCandidatesByAccount[cacheKey] = append(increaseCandidatesByAccount[cacheKey], cand)
			continue
		}

		// Kill/Decrease: cập nhật currentMetrics.actions, ghi activity, propose
		if hasPending && pendingInfo.ActionType == actionType && pendingInfo.RuleCode == ruleCode {
			continue
		}
		if hasPending {
			_, _ = approval.Cancel(ctx, pendingInfo.ID.Hex(), c.OwnerOrganizationID)
		}
		if err := UpdateCampaignActionsAndRecordActivity(ctx, c.CampaignId, c.AdAccountId, c.OwnerOrganizationID, currentMetrics, actions, report); err != nil {
			continue
		}
		metricsPayload := buildMetricsPayloadForNotification(ctx, campaignsColl, c.CampaignId, c.AdAccountId, c.OwnerOrganizationID, currentMetrics)
		if metricsPayload == nil {
			metricsPayload = make(map[string]interface{})
		}
		if rc := action["result_check"]; rc != nil {
			metricsPayload["result_check"] = rc
		}
		if traceId, _ := action["traceId"].(string); traceId != "" {
			metricsPayload["traceId"] = traceId
		}
		reason, _ := action["reason"].(string)
		value := action["value"]
		traceID, _ := action["traceId"].(string)
		// Vision 08: Luôn gọi Propose; ResolveImmediate (Executor) quyết định auto-approve theo ApprovalModeConfig.
		_, err = Propose(ctx, &ProposeInput{
			ActionType:   actionType,
			AdAccountId:  c.AdAccountId,
			CampaignId:   c.CampaignId,
			CampaignName: c.CampaignName,
			Reason:       reason,
			Value:        value,
			RuleCode:     ruleCode,
			TraceID:      traceID,
			Payload:      metricsPayload,
		}, c.OwnerOrganizationID, baseURL)
		if err != nil {
			return proposed, fmt.Errorf("emit propose campaign %s: %w", c.CampaignId, err)
		}
		proposed++
	}

	// Anti Self-Competition: xử lý Increase — chỉ top N camp/account
	for cacheKey, candidates := range increaseCandidatesByAccount {
		metaCfg := campaignConfigByAccount[cacheKey]
		mode := ModeNORMAL
		if metaCfg != nil && metaCfg.AccountMode != "" {
			mode = metaCfg.AccountMode
		}
		limit := GetIncreaseLimit(mode)
		if len(candidates) == 0 {
			continue
		}
		sortIncreaseCandidates(candidates)
		topN := candidates
		if len(topN) > limit {
			topN = topN[:limit]
		}
		for _, cand := range topN {
			if hasPending, _ := HasPendingProposalForCampaign(ctx, cand.c.CampaignId, cand.c.OwnerOrganizationID); hasPending {
				continue
			}
			if err := UpdateCampaignActionsAndRecordActivity(ctx, cand.c.CampaignId, cand.c.AdAccountId, cand.c.OwnerOrganizationID, cand.currentMetrics, cand.actions, cand.report); err != nil {
				continue
			}
			metricsPayload := buildMetricsPayloadForNotification(ctx, campaignsColl, cand.c.CampaignId, cand.c.AdAccountId, cand.c.OwnerOrganizationID, cand.currentMetrics)
			if metricsPayload == nil {
				metricsPayload = make(map[string]interface{})
			}
			if rc := cand.action["result_check"]; rc != nil {
				metricsPayload["result_check"] = rc
			}
			if traceId, _ := cand.action["traceId"].(string); traceId != "" {
				metricsPayload["traceId"] = traceId
			}
			reason, _ := cand.action["reason"].(string)
			ruleCode, _ := cand.action["ruleCode"].(string)
			traceID, _ := cand.action["traceId"].(string)
			// Vision 08: Luôn gọi Propose; ResolveImmediate (Executor) quyết định auto-approve theo ApprovalModeConfig.
			_, err = Propose(ctx, &ProposeInput{
				ActionType:   "INCREASE",
				AdAccountId:  cand.c.AdAccountId,
				CampaignId:   cand.c.CampaignId,
				CampaignName: cand.c.CampaignName,
				Reason:       reason,
				Value:        cand.action["value"],
				RuleCode:     ruleCode,
				TraceID:      traceID,
				Payload:      metricsPayload,
			}, cand.c.OwnerOrganizationID, baseURL)
			if err != nil {
				return proposed, fmt.Errorf("emit propose campaign %s: %w", cand.c.CampaignId, err)
			}
			proposed++
		}
	}
	return proposed, nil
}

// sortIncreaseCandidates sắp xếp theo priority: CPA_Purchase thấp, CHS healthy (>=60), MQS cao, Frequency thấp.
func sortIncreaseCandidates(candidates []increaseCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		return increaseCandidateLess(candidates[i], candidates[j])
	})
}

func increaseCandidateLess(a, b increaseCandidate) bool {
	if a.cpaPurchase != b.cpaPurchase {
		return a.cpaPurchase < b.cpaPurchase
	}
	aHealthy := a.chs >= 60
	bHealthy := b.chs >= 60
	if aHealthy != bHealthy {
		return aHealthy
	}
	if a.mqs != b.mqs {
		return a.mqs > b.mqs
	}
	return a.frequency < b.frequency
}
