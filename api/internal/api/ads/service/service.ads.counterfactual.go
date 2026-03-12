// Package adssvc — Counterfactual Kill Tracker (FolkForm v4.1 Section 2.3).
// B1: Snapshot khi kill. B2: Xác định siblings. B3: Đánh giá outcome 4h sau kill.
package adssvc

import (
	"context"
	"fmt"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"

	pkgapproval "meta_commerce/pkg/approval"
)

// Rule codes được coi là "kill" cho Counterfactual (Stop Loss, Kill Off, Trim, CHS).
var killRuleCodesForCounterfactual = map[string]bool{
	"sl_a": true, "sl_b": true, "sl_c": true, "sl_d": true, "sl_e": true,
	"chs_critical": true, "ko_a": true, "ko_b": true, "ko_c": true,
	"trim_eligible": true, "mess_trap_suspect": true,
}

// SaveKillSnapshotIfKill B1: Lưu snapshot khi kill campaign. Chỉ lưu khi ActionType=PAUSE/KILL, objectType=campaign, ruleCode là kill rule.
func SaveKillSnapshotIfKill(ctx context.Context, doc *pkgapproval.ActionPending) error {
	if doc == nil || doc.Payload == nil {
		return nil
	}
	if doc.ActionType != "KILL" && doc.ActionType != "PAUSE" {
		return nil
	}
	campaignId, _ := doc.Payload["campaignId"].(string)
	adAccountId, _ := doc.Payload["adAccountId"].(string)
	if campaignId == "" || adAccountId == "" {
		return nil
	}
	ruleCode, _ := doc.Payload["ruleCode"].(string)
	if ruleCode == "" || !killRuleCodesForCounterfactual[ruleCode] {
		return nil
	}
	// Chỉ snapshot khi kill campaign (không phải adset/ad)
	objectType, _ := doc.ExecuteResponse["objectType"].(string)
	if objectType != "" && objectType != "campaign" {
		return nil
	}

	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return nil
	}
	var camp struct {
		CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
	}
	err := campColl.FindOne(ctx, bson.M{
		"campaignId":          campaignId,
		"ownerOrganizationId": doc.OwnerOrganizationID,
	}, mongoopts.FindOne().SetProjection(bson.M{"currentMetrics": 1})).Decode(&camp)
	if err != nil {
		return nil
	}
	layer1, _ := camp.CurrentMetrics["layer1"].(map[string]interface{})
	layer2, _ := camp.CurrentMetrics["layer2"].(map[string]interface{})
	layer3, _ := camp.CurrentMetrics["layer3"].(map[string]interface{})
	raw, _ := camp.CurrentMetrics["raw"].(map[string]interface{})
	r7d, _ := raw["7d"].(map[string]interface{})
	meta7d, _ := r7d["meta"].(map[string]interface{})

	cpaMess := toFloatCf(layer1, "cpaMess_7d")
	convRate := toFloatCf(layer1, "convRate_7d")
	mess := toInt64Cf(meta7d, "mess")
	mqs := toFloatCf(layer1, "mqs_7d")
	chs := toFloatCf(layer3, "chs")
	spend := toFloatCf(meta7d, "spend")
	spendPct := toFloatCf(layer2, "spendPct_7d")

	// Lấy mode từ account
	cfg, _ := adsconfig.GetConfig(ctx, adAccountId, doc.OwnerOrganizationID)
	modeDay := "NORMAL"
	if cfg != nil && cfg.Account.AccountMode != "" {
		modeDay = cfg.Account.AccountMode
	}

	now := time.Now().UnixMilli()
	snap := &adsmodels.AdsKillSnapshot{
		CampaignId:          campaignId,
		AdAccountId:         adAccountId,
		OwnerOrganizationID: doc.OwnerOrganizationID,
		KillTime:            doc.ExecutedAt,
		TriggerRule:         ruleCode,
		ModeDay:             modeDay,
		CpaMess:             cpaMess,
		ConvRate:            convRate,
		Mess:                mess,
		Mqs:                 mqs,
		Chs:                 chs,
		Spend:               spend,
		SpendPct:            spendPct,
		SiblingCampIds:      []string{},
		ActionPendingId:     doc.ID,
		CreatedAt:           now,
	}

	// B2: Xác định siblings ngay
	siblings := identifySiblingCamps(ctx, snap)
	snap.SiblingCampIds = siblings

	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsKillSnapshots)
	if !ok {
		return fmt.Errorf("không tìm thấy collection ads_kill_snapshots")
	}
	_, err = coll.InsertOne(ctx, snap)
	if err != nil {
		logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
			"campaignId": campaignId, "ruleCode": ruleCode,
		}).Warn("🔍 [COUNTERFACTUAL] Lỗi lưu kill snapshot")
		return err
	}
	logger.GetAppLogger().WithFields(map[string]interface{}{
		"campaignId": campaignId, "ruleCode": ruleCode, "siblings": len(siblings),
	}).Info("🔍 [COUNTERFACTUAL] Đã lưu kill snapshot")
	return nil
}

// identifySiblingCamps B2: Tìm camp active có pattern tương tự ±20% CPA_Mess, ±30% Conv_Rate, cùng mode_day.
func identifySiblingCamps(ctx context.Context, snap *adsmodels.AdsKillSnapshot) []string {
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return nil
	}
	// Camp active cùng account, khác campaign bị kill. Hỗ trợ adAccountId "act_123" và "123".
	adAccountIds := []string{snap.AdAccountId}
	if len(snap.AdAccountId) > 4 && snap.AdAccountId[:4] == "act_" {
		adAccountIds = append(adAccountIds, snap.AdAccountId[4:])
	} else if snap.AdAccountId != "" {
		adAccountIds = append(adAccountIds, "act_"+snap.AdAccountId)
	}
	filter := bson.M{
		"adAccountId":         bson.M{"$in": adAccountIds},
		"ownerOrganizationId": snap.OwnerOrganizationID,
		"campaignId":           bson.M{"$ne": snap.CampaignId},
		"$or": []bson.M{
			{"effectiveStatus": "ACTIVE"},
			{"status": "ACTIVE"},
		},
		"currentMetrics.layer1": bson.M{"$exists": true},
	}
	cursor, err := campColl.Find(ctx, filter, nil)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)

	var siblings []string
	cpaLo := snap.CpaMess * 0.80
	cpaHi := snap.CpaMess * 1.20
	if snap.CpaMess <= 0 {
		cpaLo, cpaHi = 0, 1e9
	}
	crLo := snap.ConvRate * 0.70
	crHi := snap.ConvRate * 1.30
	if snap.ConvRate <= 0 {
		crLo, crHi = 0, 1.0
	}

	for cursor.Next(ctx) {
		var c struct {
			CampaignId     string                 `bson:"campaignId"`
			CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		}
		if err := cursor.Decode(&c); err != nil {
			continue
		}
		layer1, _ := c.CurrentMetrics["layer1"].(map[string]interface{})
		if layer1 == nil {
			continue
		}
		cpa := toFloatCf(layer1, "cpaMess_7d")
		cr := toFloatCf(layer1, "convRate_7d")
		if cpa >= cpaLo && cpa <= cpaHi && cr >= crLo && cr <= crHi {
			siblings = append(siblings, c.CampaignId)
		}
	}
	return siblings
}

// EvaluatePendingKills B3: Đánh giá các kill đã qua 4h, tạo counterfactual_outcome.
func EvaluatePendingKills(ctx context.Context) (evaluated int, err error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsKillSnapshots)
	if !ok {
		return 0, nil
	}
	outcomeColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsCounterfactualOutcomes)
	if !ok {
		return 0, nil
	}
	now := time.Now().UnixMilli()
	cutoff4h := now - 4*60*60*1000

	cursor, err := coll.Find(ctx, bson.M{
		"killTime": bson.M{"$lte": cutoff4h},
	}, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var snap adsmodels.AdsKillSnapshot
		if err := cursor.Decode(&snap); err != nil {
			continue
		}
		// Đã có outcome chưa?
		var existing struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		_ = outcomeColl.FindOne(ctx, bson.M{"killSnapshotId": snap.ID}).Decode(&existing)
		if existing.ID != primitive.NilObjectID {
			continue
		}
		outcome := evaluateOutcome4h(ctx, &snap)
		if outcome != nil {
			_, _ = outcomeColl.InsertOne(ctx, outcome)
			evaluated++
		}
	}
	return evaluated, nil
}

// evaluateOutcome4h tạo AdsCounterfactualOutcome từ snapshot. Siblings tốt (CR>12%, có đơn) → false_positive.
func evaluateOutcome4h(ctx context.Context, snap *adsmodels.AdsKillSnapshot) *adsmodels.AdsCounterfactualOutcome {
	if len(snap.SiblingCampIds) == 0 {
		return &adsmodels.AdsCounterfactualOutcome{
			KillSnapshotId:      snap.ID,
			CampaignId:          snap.CampaignId,
			AdAccountId:         snap.AdAccountId,
			OwnerOrganizationID: snap.OwnerOrganizationID,
			KillTime:            snap.KillTime,
			EvaluatedAt:         time.Now().UnixMilli(),
			SiblingCount:        0,
			Outcome:             adsmodels.OutcomeInconclusive,
			CreatedAt:           time.Now().UnixMilli(),
		}
	}
	// Lấy orders + mess của siblings trong 4h sau kill
	startMs := snap.KillTime
	endMs := snap.KillTime + 4*60*60*1000
	orders4h, mess4h, cr4h := getSiblingsMetrics4h(ctx, snap.AdAccountId, snap.OwnerOrganizationID, snap.SiblingCampIds, startMs, endMs)
	revenue4h := getSiblingsRevenue4h(ctx, snap.OwnerOrganizationID, snap.SiblingCampIds, startMs, endMs)

	outcome := adsmodels.OutcomeCorrect
	if mess4h > 0 && cr4h > 0.12 && orders4h >= 1 {
		outcome = adsmodels.OutcomeFalsePositive
	}
	if mess4h == 0 && orders4h == 0 {
		outcome = adsmodels.OutcomeInconclusive
	}

	return &adsmodels.AdsCounterfactualOutcome{
		KillSnapshotId:      snap.ID,
		CampaignId:          snap.CampaignId,
		AdAccountId:         snap.AdAccountId,
		OwnerOrganizationID: snap.OwnerOrganizationID,
		KillTime:            snap.KillTime,
		EvaluatedAt:         time.Now().UnixMilli(),
		SiblingCr4h:         cr4h,
		SiblingOrders4h:     orders4h,
		SiblingCount:        len(snap.SiblingCampIds),
		Outcome:             outcome,
		RevenueMissEst:      revenue4h,
		CreatedAt:           time.Now().UnixMilli(),
	}
}

func getSiblingsMetrics4h(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, campIds []string, startMs, endMs int64) (orders int64, mess int64, cr float64) {
	if len(campIds) == 0 {
		return 0, 0, 0
	}
	// Lấy từ meta_campaigns currentMetrics — raw.2h/1h có thể không đủ 4h. Cần aggregate từ pc_pos_orders + fb_conversations.
	// Đơn giản: dùng pc_pos_orders cho orders, fb_conversations cho mess trong window [startMs, endMs]
	posColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return 0, 0, 0
	}
	// Cần ad_ids của các campaign
	adIds := getAdIdsForCampaigns(ctx, campIds, ownerOrgID)
	if len(adIds) == 0 {
		return 0, 0, 0
	}
	startSec := startMs / 1000
	endSec := endMs / 1000
	var ordDoc struct {
		Orders int64 `bson:"orders"`
	}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"posData.ad_id":       bson.M{"$in": adIds},
			"$or": []bson.M{
				{"posCreatedAt": bson.M{"$gte": startSec, "$lte": endSec}},
				{"insertedAt": bson.M{"$gte": startSec, "$lte": endSec}},
				{"posCreatedAt": bson.M{"$gte": startMs, "$lte": endMs}},
				{"insertedAt": bson.M{"$gte": startMs, "$lte": endMs}},
			},
		}}},
		{{Key: "$group", Value: bson.M{"_id": nil, "orders": bson.M{"$sum": 1}}}},
	}
	cur, err := posColl.Aggregate(ctx, pipe)
	if err == nil && cur != nil {
		defer cur.Close(ctx)
		if cur.Next(ctx) {
			_ = cur.Decode(&ordDoc)
		}
	}
	orders = ordDoc.Orders

	convColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return orders, 0, 0
	}
	var messDoc struct {
		N int64 `bson:"n"`
	}
	convTsMs := bson.M{
		"$cond": bson.A{
			bson.M{"$and": bson.A{
				bson.M{"$ne": bson.A{"$panCakeData.inserted_at", nil}},
				bson.M{"$ne": bson.A{"$panCakeData.inserted_at", ""}},
			}},
			bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "string"}},
				bson.M{"$let": bson.M{
					"vars": bson.M{"p": bson.M{"$dateFromString": bson.M{"dateString": bson.M{"$substr": bson.A{"$panCakeData.inserted_at", 0, 19}}, "format": "%Y-%m-%dT%H:%M:%S", "onError": nil, "onNull": nil}}},
					"in": bson.M{"$cond": bson.A{bson.M{"$ne": bson.A{"$$p", nil}}, bson.M{"$toLong": "$$p"}, bson.M{"$multiply": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1}}}},
				}},
				bson.M{"$cond": bson.A{
					bson.M{"$gte": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1e12}},
					bson.M{"$toLong": "$panCakeData.inserted_at"},
					bson.M{"$multiply": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1000}},
				}},
			}},
			nil,
		},
	}
	convPipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"$or": []bson.M{
				{"panCakeData.ad_ids": bson.M{"$in": adIds}},
				{"panCakeData.ads.ad_id": bson.M{"$in": adIds}},
			},
		}}},
		{{Key: "$addFields", Value: bson.M{"_ts": convTsMs}}},
		{{Key: "$match", Value: bson.M{"_ts": bson.M{"$ne": nil, "$gte": startMs, "$lte": endMs}}}},
		{{Key: "$count", Value: "n"}},
	}
	convCur, _ := convColl.Aggregate(ctx, convPipe)
	if convCur != nil {
		defer convCur.Close(ctx)
		if convCur.Next(ctx) {
			_ = convCur.Decode(&messDoc)
		}
	}
	mess = messDoc.N
	if mess > 0 {
		cr = float64(orders) / float64(mess)
	}
	return orders, mess, cr
}

func getSiblingsRevenue4h(ctx context.Context, ownerOrgID primitive.ObjectID, campIds []string, startMs, endMs int64) float64 {
	adIds := getAdIdsForCampaigns(ctx, campIds, ownerOrgID)
	if len(adIds) == 0 {
		return 0
	}
	posColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return 0
	}
	startSec, endSec := startMs/1000, endMs/1000
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"posData.ad_id":       bson.M{"$in": adIds},
			"$or": []bson.M{
				{"posCreatedAt": bson.M{"$gte": startSec, "$lte": endSec}},
				{"insertedAt": bson.M{"$gte": startSec, "$lte": endSec}},
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":    nil,
			"revenue": bson.M{"$sum": bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$posData.total_price_after_sub_discount", 0}}, "to": "double", "onError": 0, "onNull": 0}}},
		}}},
	}
	cur, err := posColl.Aggregate(ctx, pipe)
	if err != nil {
		return 0
	}
	defer cur.Close(ctx)
	var doc struct {
		Revenue float64 `bson:"revenue"`
	}
	if cur.Next(ctx) {
		_ = cur.Decode(&doc)
	}
	return doc.Revenue
}

func getAdIdsForCampaigns(ctx context.Context, campaignIds []string, ownerOrgID primitive.ObjectID) []string {
	adColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return nil
	}
	cursor, err := adColl.Find(ctx, bson.M{
		"campaignId":          bson.M{"$in": campaignIds},
		"ownerOrganizationId": ownerOrgID,
	}, mongoopts.Find().SetProjection(bson.M{"adId": 1}))
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	var ids []string
	for cursor.Next(ctx) {
		var d struct {
			AdId string `bson:"adId"`
		}
		if cursor.Decode(&d) == nil && d.AdId != "" {
			ids = append(ids, d.AdId)
		}
	}
	return ids
}

func toFloatCf(m map[string]interface{}, k string) float64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok || v == nil {
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

// KillAccuracyResult B4: Kết quả Kill Accuracy Rate.
type KillAccuracyResult struct {
	Rate           float64 `json:"rate"`           // correct / (correct + false_positive), 0–1
	Correct        int     `json:"correct"`        // Số kill đúng
	FalsePositive  int     `json:"falsePositive"`   // Số kill nhầm
	Inconclusive   int     `json:"inconclusive"`   // Không xác định
	TotalEvaluated int     `json:"totalEvaluated"` // correct + false_positive + inconclusive
}

// ComputeKillAccuracy B4: Tính Kill Accuracy Rate từ counterfactual_outcomes trong window.
// Rate = correct / (correct + false_positive). Inconclusive không tính vào rate.
func ComputeKillAccuracy(ctx context.Context, ownerOrgID primitive.ObjectID, adAccountId string, windowDays int) (*KillAccuracyResult, error) {
	outcomeColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsCounterfactualOutcomes)
	if !ok {
		return nil, nil
	}
	cutoff := time.Now().AddDate(0, 0, -windowDays).UnixMilli()
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"evaluatedAt":         bson.M{"$gte": cutoff},
	}
	if adAccountId != "" {
		ids := []string{adAccountId}
		if len(adAccountId) > 4 && adAccountId[:4] == "act_" {
			ids = append(ids, adAccountId[4:])
		} else {
			ids = append(ids, "act_"+adAccountId)
		}
		filter["adAccountId"] = bson.M{"$in": ids}
	}
	cursor, err := outcomeColl.Find(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var correct, falsePos, inconclusive int
	for cursor.Next(ctx) {
		var o adsmodels.AdsCounterfactualOutcome
		if cursor.Decode(&o) != nil {
			continue
		}
		switch o.Outcome {
		case adsmodels.OutcomeCorrect:
			correct++
		case adsmodels.OutcomeFalsePositive:
			falsePos++
		case adsmodels.OutcomeInconclusive:
			inconclusive++
		}
	}
	total := correct + falsePos + inconclusive
	rate := 0.0
	if correct+falsePos > 0 {
		rate = float64(correct) / float64(correct+falsePos)
	}
	return &KillAccuracyResult{
		Rate:           rate,
		Correct:        correct,
		FalsePositive:  falsePos,
		Inconclusive:   inconclusive,
		TotalEvaluated: total,
	}, nil
}

// ShouldSuggestThresholdAdjustment B5: Đề xuất nới threshold khi Kill_Accuracy < 70% liên tục 2 tuần.
// Trả về true nếu nên đề xuất điều chỉnh.
func ShouldSuggestThresholdAdjustment(ctx context.Context, ownerOrgID primitive.ObjectID, adAccountId string) (bool, *KillAccuracyResult, error) {
	r, err := ComputeKillAccuracy(ctx, ownerOrgID, adAccountId, 14)
	if err != nil || r == nil {
		return false, nil, err
	}
	if r.TotalEvaluated < 3 {
		return false, r, nil // Cần ít nhất 3 outcomes để đánh giá
	}
	return r.Rate < 0.70, r, nil
}

func toInt64Cf(m map[string]interface{}, k string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}
