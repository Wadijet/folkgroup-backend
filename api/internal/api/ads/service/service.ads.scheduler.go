// Package adssvc — Time-based scheduler jobs theo FolkForm v4.1 R02–R07, R10.
// Reset Budget 05:30, Morning On 06:00, Noon Cut 12:30/14:00, Noon Cut Resume 14:30, Night Off theo mode, Volume Push 16h/18h.
package adssvc

import (
	"context"
	"fmt"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	adsrules "meta_commerce/internal/api/ads/rules"
	"meta_commerce/internal/approval"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"
	metasvc "meta_commerce/internal/api/meta/service"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

// RunResetBudget chạy lúc 05:30 — set budget tối ưu theo Best_day (last N days). RULE 10.
// Best_day = ngày CPA_Purchase thấp nhất + Pancake_orders ≥ 1 (3 ngày). Event Prep × 1.2x. CHS adj (avg < 1.0 → ×1.1x, > 1.8 → ×0.9x).
func RunResetBudget(ctx context.Context) {
	log := logger.GetAppLogger()
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return
	}
	campColl, okCamp := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !okCamp {
		return
	}
	cursor, err := accColl.Find(ctx, bson.M{}, nil)
	if err != nil {
		log.WithError(err).Warn("[RESET_BUDGET] Lỗi query ad accounts")
		return
	}
	defer cursor.Close(ctx)
	var processed int
	for cursor.Next(ctx) {
		var acc struct {
			AdAccountId         string             `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&acc); err != nil {
			continue
		}
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
		common := adsconfig.GetCommon(cfg)
		if !common.ResetBudgetEnabled {
			continue
		}
		windowDays := common.BestDayWindowDays
		if windowDays <= 0 {
			windowDays = 3
		}
		// Lấy campaigns Purchase Messaging active
		campFilter := bson.M{
			"adAccountId":         acc.AdAccountId,
			"ownerOrganizationId": acc.OwnerOrganizationID,
			"$or":                 []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}},
		}
		for k, v := range adsconfig.ScopeFilterPurchaseMessaging() {
			campFilter[k] = v
		}
		cur, err := campColl.Find(ctx, campFilter, nil)
		if err != nil {
			continue
		}
		locVN, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
		nowVN := time.Now().In(locVN)
		for cur.Next(ctx) {
			var camp struct {
				CampaignId string `bson:"campaignId"`
			}
			if cur.Decode(&camp) != nil {
				continue
			}
			optimal := computeOptimalBudgetFromBestDay(ctx, camp.CampaignId, acc.AdAccountId, acc.OwnerOrganizationID, windowDays)
			if optimal <= 0 {
				continue
			}
			// Mode multiplier
			modeMult := 1.0
			if cfg != nil && cfg.AccountMode != "" {
				switch cfg.AccountMode {
				case ModePROTECT:
					modeMult = 0.7
				case ModeEFFICIENCY:
					modeMult = 0.85
				case ModeNORMAL:
					modeMult = 1.0
				case ModeBLITZ:
					modeMult = 1.3
				}
			}
			optimal *= modeMult
			// Event Prep × 1.2x (FolkForm v4.1 RULE 10) — dùng giờ VN để check đúng ngày
			if inEvent, bonus, evName := adsconfig.IsEventWindow(nowVN); inEvent {
				optimal *= 1.2
				log.WithFields(map[string]interface{}{
					"campaignId": camp.CampaignId,
					"event":      evName,
					"blitzBonus": bonus,
				}).Debug("🔄 [RESET_BUDGET] Event Prep × 1.2x áp dụng")
			}
			// CHS adj: avg < 1.0 → ×1.1, > 1.8 → ×0.9
			avgChs, _ := getAccountAvgCHS(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
			if avgChs > 0 && avgChs < 1.0 {
				optimal *= 1.1
			} else if avgChs > 1.8 {
				optimal *= 0.9
			}
			// Cap: Budget ≤ 28day_max × 1.1 (FolkForm v4.1 RULE 10)
			if max28 := getCampaignMaxSpend28Day(ctx, camp.CampaignId, acc.AdAccountId, acc.OwnerOrganizationID); max28 > 0 {
				cap := max28 * 1.1
				if optimal > cap {
					optimal = cap
				}
			}
			optimalTr := int64(optimal / 1e6) // triệu VNĐ
			if optimalTr < 1 {
				optimalTr = 1
			}
			if _, err := Propose(ctx, &ProposeInput{
				ActionType:   "SET_BUDGET",
				AdAccountId:  acc.AdAccountId,
				CampaignId:   camp.CampaignId,
				Reason:       "Reset Budget 05:30 — Best_day + Event × 1.2x + CHS adj",
				RuleCode:     "reset_budget",
				Value:        optimalTr * 1e6, // VNĐ
			}, acc.OwnerOrganizationID, ""); err == nil {
				processed++
			}
		}
		cur.Close(ctx)
	}
	log.WithFields(map[string]interface{}{"processed": processed}).Info("🔄 [RESET_BUDGET] Đã chạy Reset Budget 05:30")
}

// computeOptimalBudgetFromBestDay tính Best_day = ngày CPA_Purchase thấp nhất + orders ≥ 1 (3 ngày). Optimal = spend của Best_day.
func computeOptimalBudgetFromBestDay(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, windowDays int) float64 {
	rows, err := getCampaignDaySeriesForReset(ctx, campaignId, adAccountId, ownerOrgID, windowDays)
	if err != nil || len(rows) == 0 {
		return 0
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	dateEnd := now.Format("2006-01-02")
	dateStart := now.AddDate(0, 0, -windowDays).Format("2006-01-02")
	ordersMap, ok := metasvc.GetCampaignDailyOrdersMap(ctx, campaignId, adAccountId, ownerOrgID, dateStart, dateEnd)
	if !ok {
		ordersMap = make(map[string]int64)
	}
	var bestSpend float64
	var bestCpa float64 = 1e18
	for _, r := range rows {
		orders := ordersMap[r.Date]
		if orders < 1 {
			continue
		}
		cpa := r.Spend / float64(orders)
		if cpa < bestCpa {
			bestCpa = cpa
			bestSpend = r.Spend
		}
	}
	if bestSpend > 0 {
		return bestSpend
	}
	// Fallback: avg spend của các ngày có orders
	var sum float64
	var count int
	for _, r := range rows {
		if ordersMap[r.Date] >= 1 {
			sum += r.Spend
			count++
		}
	}
	if count > 0 {
		return sum / float64(count)
	}
	return 0
}

// daySpendRow dùng cho getCampaignDaySeriesForReset.
type daySpendRow struct {
	Date  string  `bson:"_id"`
	Spend float64 `bson:"spend"`
}

// getCampaignDaySeriesForReset lấy chuỗi N ngày (spend, date) cho Reset Budget.
func getCampaignDaySeriesForReset(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, days int) ([]daySpendRow, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return nil, nil
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	dateEnd := now.Format("2006-01-02")
	dateStart := now.AddDate(0, 0, -days).Format("2006-01-02")
	adAccountFilter := adAccountIdFilterForMeta(adAccountId)
	filter := bson.M{
		"objectType":          "campaign",
		"objectId":            campaignId,
		"adAccountId":         adAccountFilter,
		"ownerOrganizationId": ownerOrgID,
		"dateStart":           bson.M{"$gte": dateStart, "$lte": dateEnd},
	}
	extractSpend := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$spend", "0"}}, "to": "double", "onError": 0, "onNull": 0}}
	cursor, err := coll.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$addFields", Value: bson.M{"_spend": extractSpend}}},
		{{Key: "$group", Value: bson.M{"_id": "$dateStart", "spend": bson.M{"$sum": "$_spend"}}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var rows []daySpendRow
	for cursor.Next(ctx) {
		var doc daySpendRow
		if cursor.Decode(&doc) == nil {
			rows = append(rows, doc)
		}
	}
	return rows, nil
}

// getAccountAvgCHS trả về avg CHS của campaigns active trong account. 0 nếu không có.
func getAccountAvgCHS(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (float64, bool) {
	return metasvc.GetCHSAccountAvg(ctx, adAccountId, ownerOrgID)
}

// getCampaignMaxSpend28Day trả về spend cao nhất trong 28 ngày gần nhất. 0 nếu không có data.
func getCampaignMaxSpend28Day(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) float64 {
	rows, err := getCampaignDaySeriesForReset(ctx, campaignId, adAccountId, ownerOrgID, 28)
	if err != nil || len(rows) == 0 {
		return 0
	}
	var max float64
	for _, r := range rows {
		if r.Spend > max {
			max = r.Spend
		}
	}
	return max
}

// RunMorningOn chạy lúc 06:00 — bật lại camp tốt (MO-A, MO-B). RULE 02.
// Dùng flag mo_eligible: CPA_Mess < 216k, CR >= 8%, CHS healthy, orders >= 1, mess >= 3, freq < 3.0.
func RunMorningOn(ctx context.Context, baseURL string) {
	log := logger.GetAppLogger()
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return
	}
	filter := bson.M{
		"$or":                  []bson.M{{"effectiveStatus": "PAUSED"}, {"status": "PAUSED"}},
		"currentMetrics.raw":   bson.M{"$exists": true},
		"currentMetrics.layer1": bson.M{"$exists": true},
	}
	for k, v := range adsconfig.ScopeFilterPurchaseMessaging() {
		filter[k] = v
	}
	cursor, err := campColl.Find(ctx, filter, nil)
	if err != nil {
		log.WithError(err).Warn("[MORNING_ON] Lỗi query")
		return
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var doc struct {
			CampaignId          string                 `bson:"campaignId"`
			AdAccountId         string                 `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID     `bson:"ownerOrganizationId"`
			CurrentMetrics      map[string]interface{} `bson:"currentMetrics"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		cm := doc.CurrentMetrics
		if cm == nil {
			continue
		}
		raw, _ := cm["raw"].(map[string]interface{})
		layer1, _ := cm["layer1"].(map[string]interface{})
		layer2, _ := cm["layer2"].(map[string]interface{})
		layer3, _ := cm["layer3"].(map[string]interface{})
		if raw == nil || layer1 == nil {
			continue
		}
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, doc.AdAccountId, doc.OwnerOrganizationID)
		ctxFacts := adsrules.BuildFactsContext(getRaw7d(raw), layer1, layer2, layer3, cfg)
		campCtx := &adsrules.EvalCampaignContext{CampaignId: doc.CampaignId, AdAccountId: doc.AdAccountId, OwnerOrgID: doc.OwnerOrganizationID}
		flags := adsrules.EvaluateFlags(ctx, &ctxFacts, cfg, campCtx)
		flagsIf := make([]interface{}, len(flags))
		for i, f := range flags {
			flagsIf[i] = f
		}
		result := adsrules.EvaluateForResume(flagsIf, cfg)
		if result == nil || !result.ShouldPropose {
			continue
		}
		pending, err := Propose(ctx, &ProposeInput{
			ActionType:   "RESUME",
			AdAccountId:  doc.AdAccountId,
			CampaignId:   doc.CampaignId,
			Reason:       result.Reason,
			RuleCode:     result.RuleCode,
		}, doc.OwnerOrganizationID, baseURL)
		if err != nil {
			continue
		}
		if pending != nil {
			approval.Approve(ctx, pending.ID.Hex(), doc.OwnerOrganizationID)
			count++
		}
	}
	if count > 0 {
		log.WithFields(map[string]interface{}{"count": count}).Info("🟢 [MORNING_ON] Đã bật lại camp")
	}
}

// getRaw7d trả về raw.7d từ raw (hỗ trợ cấu trúc nested và phẳng).
func getRaw7d(raw map[string]interface{}) map[string]interface{} {
	if r, ok := raw["7d"].(map[string]interface{}); ok && r != nil {
		return r
	}
	return raw
}

// RunNoonCutOff chạy 12:30 và 14:00 — tắt camp chết buổi trưa. RULE 03.
// Dùng flag noon_cut_eligible: CPA_Mess > 144k, Spend < 55%, CHS yếu (warning/critical).
func RunNoonCutOff(ctx context.Context) {
	log := logger.GetAppLogger()
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return
	}
	filter := bson.M{
		"$or":                  []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}},
		"currentMetrics.raw":   bson.M{"$exists": true},
		"currentMetrics.layer1": bson.M{"$exists": true},
	}
	for k, v := range adsconfig.ScopeFilterPurchaseMessaging() {
		filter[k] = v
	}
	cursor, err := campColl.Find(ctx, filter, nil)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)

	count := 0
	for cursor.Next(ctx) {
		var doc struct {
			CampaignId          string                 `bson:"campaignId"`
			AdAccountId         string                 `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID     `bson:"ownerOrganizationId"`
			CurrentMetrics      map[string]interface{} `bson:"currentMetrics"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		cm := doc.CurrentMetrics
		if cm == nil {
			continue
		}
		raw, _ := cm["raw"].(map[string]interface{})
		layer1, _ := cm["layer1"].(map[string]interface{})
		layer2, _ := cm["layer2"].(map[string]interface{})
		layer3, _ := cm["layer3"].(map[string]interface{})
		if raw == nil || layer1 == nil {
			continue
		}
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, doc.AdAccountId, doc.OwnerOrganizationID)
		ctxFacts := adsrules.BuildFactsContext(getRaw7d(raw), layer1, layer2, layer3, cfg)
		campCtx := &adsrules.EvalCampaignContext{CampaignId: doc.CampaignId, AdAccountId: doc.AdAccountId, OwnerOrgID: doc.OwnerOrganizationID}
		flags := adsrules.EvaluateFlags(ctx, &ctxFacts, cfg, campCtx)
		hasNoonCut := false
		for _, f := range flags {
			if f == "noon_cut_eligible" {
				hasNoonCut = true
				break
			}
		}
		if !hasNoonCut {
			continue
		}
		// Bỏ qua nếu có safety_net
		for _, f := range flags {
			if f == "safety_net" {
				hasNoonCut = false
				break
			}
		}
		if !hasNoonCut {
			continue
		}
		pending, err := Propose(ctx, &ProposeInput{
			ActionType:   "PAUSE",
			AdAccountId:  doc.AdAccountId,
			CampaignId:   doc.CampaignId,
			Reason:       "Noon Cut — camp chết buổi trưa, bật lại 14:30",
			RuleCode:     "noon_cut",
		}, doc.OwnerOrganizationID, "")
		if err != nil {
			continue
		}
		if pending != nil {
			approval.Approve(ctx, pending.ID.Hex(), doc.OwnerOrganizationID)
			count++
		}
	}
	if count > 0 {
		log.WithFields(map[string]interface{}{"count": count}).Info("🟡 [NOON_CUT] Đã tắt camp")
	}
}

// RunNoonCutResume chạy 14:30 — bật lại camp đã tắt bởi Noon Cut.
// Chỉ resume campaign có ruleCode=noon_cut trong action_pending (executed trong 3h qua).
func RunNoonCutResume(ctx context.Context) {
	log := logger.GetAppLogger()
	actionColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.ActionPendingApproval)
	if !ok {
		return
	}
	cutoff := time.Now().Add(-3 * time.Hour).UnixMilli()
	cursor, err := actionColl.Find(ctx, bson.M{
		"domain":     "ads",
		"status":     "executed",
		"actionType": "PAUSE",
		"payload.ruleCode": "noon_cut",
		"executedAt": bson.M{"$gte": cutoff},
	}, nil)
	if err != nil {
		log.WithError(err).Warn("[NOON_CUT_RESUME] Lỗi query")
		return
	}
	defer cursor.Close(ctx)

	// Thu thập (campaignId, adAccountId, ownerOrgID) unique
	type campKey struct {
		CampaignId string
		AdAccountId string
		OwnerOrgID string
	}
	seen := make(map[campKey]bool)
	var toResume []struct {
		CampaignId          string
		AdAccountId         string
		OwnerOrganizationID primitive.ObjectID
	}
	for cursor.Next(ctx) {
		var doc struct {
			Payload             map[string]interface{} `bson:"payload"`
			OwnerOrganizationID primitive.ObjectID    `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		cid, _ := doc.Payload["campaignId"].(string)
		aid, _ := doc.Payload["adAccountId"].(string)
		if cid == "" || aid == "" {
			continue
		}
		k := campKey{CampaignId: cid, AdAccountId: aid, OwnerOrgID: doc.OwnerOrganizationID.Hex()}
		if seen[k] {
			continue
		}
		seen[k] = true
		toResume = append(toResume, struct {
			CampaignId          string
			AdAccountId         string
			OwnerOrganizationID primitive.ObjectID
		}{cid, aid, doc.OwnerOrganizationID})
	}

	count := 0
	for _, c := range toResume {
		pending, err := Propose(ctx, &ProposeInput{
			ActionType:   "RESUME",
			AdAccountId:  c.AdAccountId,
			CampaignId:   c.CampaignId,
			Reason:       "Noon Cut Resume 14:30 — bật lại camp đã tắt trưa",
			RuleCode:     "noon_cut_resume",
		}, c.OwnerOrganizationID, "")
		if err != nil {
			continue
		}
		if pending != nil {
			approval.Approve(ctx, pending.ID.Hex(), c.OwnerOrganizationID)
			count++
		}
	}
	if count > 0 {
		log.WithFields(map[string]interface{}{"count": count}).Info("🟡 [NOON_CUT_RESUME] Đã bật lại camp")
	}
}

// RunNightOff chạy theo giờ tắt của từng account (21h, 22h, 22:30, 23h). RULE 07.
func RunNightOff(ctx context.Context) {
	log := logger.GetAppLogger()
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return
	}
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	h, m := now.Hour(), now.Minute()

	cursor, err := accColl.Find(ctx, bson.M{}, nil)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var acc struct {
			AdAccountId         string              `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&acc); err != nil {
			continue
		}
		// accountMode: chỉ đọc từ ads_meta_config. Fallback NORMAL khi rỗng để Night Off vẫn chạy.
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
		accountMode := ModeNORMAL
		if cfg != nil && cfg.AccountMode != "" {
			accountMode = cfg.AccountMode
		}
		common := adsconfig.GetCommon(cfg)
		hp, he, hn, mn, hb := common.NightOffHourProtect, common.NightOffHourEfficiency,
			common.NightOffHourNormal, common.NightOffMinuteNormal, common.NightOffHourBlitz
		if hp == 0 {
			hp = 21
		}
		if he == 0 {
			he = 22
		}
		if hn == 0 {
			hn = 22
		}
		if mn == 0 {
			mn = 30
		}
		if hb == 0 {
			hb = 23
		}
		shouldOff := false
		if accountMode == ModePROTECT && h == hp && m == 0 {
			shouldOff = true
		}
		if accountMode == ModeEFFICIENCY && h == he && m == 0 {
			shouldOff = true
		}
		if accountMode == ModeNORMAL && h == hn && m == mn {
			shouldOff = true
		}
		if accountMode == ModeBLITZ && h == hb && m == 0 {
			shouldOff = true
		}
		if !shouldOff {
			continue
		}

		campFilter := bson.M{
			"adAccountId":         acc.AdAccountId,
			"ownerOrganizationId": acc.OwnerOrganizationID,
			"$or":                 []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}},
		}
		for k, v := range adsconfig.ScopeFilterPurchaseMessaging() {
			campFilter[k] = v
		}
		cur, err := campColl.Find(ctx, campFilter, nil)
		if err != nil {
			continue
		}
		paused := 0
		for cur.Next(ctx) {
			var camp struct {
				CampaignId string `bson:"campaignId"`
			}
			if err := cur.Decode(&camp); err != nil {
				continue
			}
			pending, err := Propose(ctx, &ProposeInput{
				ActionType:   "PAUSE",
				AdAccountId:  acc.AdAccountId,
				CampaignId:   camp.CampaignId,
				Reason:       fmt.Sprintf("Night Off — mode %s", accountMode),
				RuleCode:     "night_off",
			}, acc.OwnerOrganizationID, "")
			if err != nil {
				continue
			}
			if pending != nil {
				approval.Approve(ctx, pending.ID.Hex(), acc.OwnerOrganizationID)
				paused++
			}
		}
		cur.Close(ctx)
		if paused > 0 {
			log.WithFields(map[string]interface{}{
				"adAccountId": acc.AdAccountId,
				"mode":       accountMode,
				"paused":     paused,
			}).Info("🌙 [NIGHT_OFF] Đã tắt camp")
		}
	}
}

// RunWeeklyFeedbackLoop chạy Thứ 2 06:05 (FolkForm v4.1 Section 08). B4 Kill Accuracy, B5 đề xuất nới threshold.
// B5 Peak Profiles: tính peak/dead hours từ snapshots 7 ngày (Hourly Peak Matrix).
func RunWeeklyFeedbackLoop(ctx context.Context) {
	log := logger.GetAppLogger()
	if _, err := RunComputePeakProfilesFromSnapshots(ctx); err != nil {
		log.WithError(err).Warn("📊 [WEEKLY] Peak Profiles lỗi")
	}
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return
	}
	cursor, err := accColl.Find(ctx, bson.M{}, mongoopts.Find().SetProjection(bson.M{"adAccountId": 1, "ownerOrganizationId": 1}))
	if err != nil {
		log.WithError(err).Warn("📊 [WEEKLY] Lỗi query ad accounts")
		return
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var acc struct {
			AdAccountId         string              `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&acc); err != nil {
			continue
		}
		r, err := ComputeKillAccuracy(ctx, acc.OwnerOrganizationID, acc.AdAccountId, 7)
		if err != nil || r == nil {
			continue
		}
		log.WithFields(map[string]interface{}{
			"adAccountId": acc.AdAccountId,
			"rate":        r.Rate,
			"correct":     r.Correct,
			"falsePos":    r.FalsePositive,
			"inconclusive": r.Inconclusive,
			"total":       r.TotalEvaluated,
		}).Info("📊 [WEEKLY] Kill Accuracy tuần")
		// B5: Nếu Kill_Accuracy < 70% liên tục 2 tuần → đề xuất nới threshold
		if suggest, _, _ := ShouldSuggestThresholdAdjustment(ctx, acc.OwnerOrganizationID, acc.AdAccountId); suggest {
			log.WithFields(map[string]interface{}{
				"adAccountId": acc.AdAccountId,
			}).Warn("📊 [WEEKLY] Kill Accuracy < 70% 2 tuần — đề xuất nới threshold 10%")
		}
	}
	log.Info("📊 [WEEKLY] Đã chạy Weekly Feedback Loop")
}

// RunVolumePush chạy 16:00 (BLITZ) hoặc 18:00 (NORMAL). EFFICIENCY không có.
func RunVolumePush(ctx context.Context, baseURL string) {
	log := logger.GetAppLogger()
	accColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	h := now.Hour()

	cursor, err := accColl.Find(ctx, bson.M{}, nil)
	if err != nil {
		return
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var acc struct {
			AdAccountId         string              `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := cursor.Decode(&acc); err != nil {
			continue
		}
		// accountMode: chỉ đọc từ ads_meta_config. Fallback NORMAL.
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, acc.AdAccountId, acc.OwnerOrganizationID)
		accountMode := ModeNORMAL
		if cfg != nil && cfg.AccountMode != "" {
			accountMode = cfg.AccountMode
		}
		// BLITZ: 16h, NORMAL: 18h, EFFICIENCY: skip
		if accountMode == ModeEFFICIENCY {
			continue
		}
		targetHour := 18
		if accountMode == ModeBLITZ {
			targetHour = 16
		}
		if h != targetHour {
			continue
		}

		// Gọi RunAutoPropose — Increase rule sẽ chạy
		_, err := RunAutoPropose(ctx, baseURL)
		if err != nil {
			log.WithError(err).WithFields(map[string]interface{}{"adAccountId": acc.AdAccountId}).Warn("[VOLUME_PUSH] Lỗi")
		}
	}
	log.Info("📈 [VOLUME_PUSH] Đã chạy Volume Push")
}
