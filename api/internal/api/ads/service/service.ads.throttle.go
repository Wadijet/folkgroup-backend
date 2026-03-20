// Package adssvc — Rule 13 Throttle (1-2-6 only): Kiềm chế Ad Set tệ trong CBO 2 Ad Sets.
// Chỉ campaign có đúng 2 ad sets. Ad Set A CPA_Mess > adaptive×0.9 và nhận >60% budget → cap 15%.
// Gỡ cap: CPA_Mess Ad Set A < adaptive×0.75x trong 2 checkpoint → Remove.
package adssvc

import (
	"context"
	"strconv"
	"time"

	adsadaptive "meta_commerce/internal/api/ads/adaptive"
	adsconfig "meta_commerce/internal/api/ads/config"
	adsmodels "meta_commerce/internal/api/ads/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const throttleRemoveMinHours = 3 // Gỡ cap chỉ khi đã cap ít nhất 3h (tránh flip-flop)

// RunThrottleCheck chạy mỗi 60p — Rule 13: cap 15% + Gỡ cap khi Ad Set cải thiện.
func RunThrottleCheck(ctx context.Context) (throttled int, err error) {
	log := logger.GetAppLogger()
	// Bước 1: Gỡ cap — xử lý các ad set đang bị cap, nếu CPA_Mess < adaptive×0.75x trong 2 checkpoint
	removed := runThrottleRemoveCheck(ctx)
	if removed > 0 {
		log.WithFields(map[string]interface{}{"removed": removed}).Info("⚙️ [THROTTLE] Đã gỡ cap Ad Set cải thiện")
	}

	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, nil
	}
	adsetColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdSets)
	if !ok {
		return 0, nil
	}
	insightColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return 0, nil
	}

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	dateEnd := now.Format("2006-01-02")
	dateStart := now.AddDate(0, 0, -7).Format("2006-01-02")

	filter := bson.M{
		"$or": []bson.M{{"effectiveStatus": "ACTIVE"}, {"status": "ACTIVE"}},
	}
	for k, v := range adsconfig.ScopeFilterPurchaseMessaging() {
		filter[k] = v
	}
	cursor, err := campColl.Find(ctx, filter, nil)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var camp struct {
			CampaignId          string                 `bson:"campaignId"`
			AdAccountId         string                 `bson:"adAccountId"`
			OwnerOrganizationID primitive.ObjectID     `bson:"ownerOrganizationId"`
			MetaData            map[string]interface{} `bson:"metaData"`
		}
		if cursor.Decode(&camp) != nil {
			continue
		}
		adsetCur, err := adsetColl.Find(ctx, bson.M{
			"campaignId":          camp.CampaignId,
			"adAccountId":         adAccountIdFilterForMeta(camp.AdAccountId),
			"ownerOrganizationId": camp.OwnerOrganizationID,
		}, nil)
		if err != nil {
			continue
		}
		var adsetIds []string
		for adsetCur.Next(ctx) {
			var as struct {
				AdSetId string `bson:"adSetId"`
			}
			if adsetCur.Decode(&as) == nil && as.AdSetId != "" {
				adsetIds = append(adsetIds, as.AdSetId)
			}
		}
		adsetCur.Close(ctx)
		if len(adsetIds) != 2 {
			continue
		}

		// Lấy spend, mess per ad set từ meta_ad_insights 7 ngày
		adAccountFilter := adAccountIdFilterForMeta(camp.AdAccountId)
		extractMess := bson.M{
			"$reduce": bson.M{
				"input": bson.M{"$ifNull": bson.A{"$metaData.actions", bson.A{}}},
				"initialValue": int64(0),
				"in": bson.M{
					"$add": bson.A{
						"$$value",
						bson.M{
							"$cond": bson.M{
								"if": bson.M{"$regexMatch": bson.M{
									"input": bson.M{"$toLower": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$$this.action_type", ""}}, ""}}},
									"regex": "messaging_conversation_started",
								}},
								"then": bson.M{"$convert": bson.M{"input": "$$this.value", "to": "long", "onError": 0, "onNull": 0}},
								"else": int64(0),
							},
						},
					},
				},
			},
		}
		extractSpend := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$spend", "0"}}, "to": "double", "onError": 0, "onNull": 0}}
		pipe := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{
				"objectType":          "adset",
				"objectId":            bson.M{"$in": adsetIds},
				"adAccountId":         adAccountFilter,
				"ownerOrganizationId": camp.OwnerOrganizationID,
				"dateStart":           bson.M{"$gte": dateStart, "$lte": dateEnd},
			}}},
			{{Key: "$addFields", Value: bson.M{"_spend": extractSpend, "_mess": extractMess}}},
			{{Key: "$group", Value: bson.M{
				"_id":   "$objectId",
				"spend": bson.M{"$sum": "$_spend"},
				"mess":  bson.M{"$sum": "$_mess"},
			}}},
		}
		insightCur, err := insightColl.Aggregate(ctx, pipe)
		if err != nil {
			continue
		}
		metrics := make(map[string]struct{ Spend, Mess float64 })
		for insightCur.Next(ctx) {
			var doc struct {
				ID    string  `bson:"_id"`
				Spend float64 `bson:"spend"`
				Mess  float64 `bson:"mess"`
			}
			if insightCur.Decode(&doc) == nil {
				metrics[doc.ID] = struct{ Spend, Mess float64 }{doc.Spend, doc.Mess}
			}
		}
		insightCur.Close(ctx)
		if len(metrics) != 2 {
			continue
		}

		m0 := metrics[adsetIds[0]]
		m1 := metrics[adsetIds[1]]
		totalSpend := m0.Spend + m1.Spend
		if totalSpend <= 0 {
			continue
		}
		var badId string
		var badCpa, goodCpa float64
		if m0.Spend >= m1.Spend {
			badId = adsetIds[0]
			badCpa = 0
			if m0.Mess > 0 {
				badCpa = m0.Spend / m0.Mess * 1000
			}
			goodCpa = 0
			if m1.Mess > 0 {
				goodCpa = m1.Spend / m1.Mess * 1000
			}
		} else {
			badId = adsetIds[1]
			badCpa = 0
			if m1.Mess > 0 {
				badCpa = m1.Spend / m1.Mess * 1000
			}
			goodCpa = 0
			if m0.Mess > 0 {
				goodCpa = m0.Spend / m0.Mess * 1000
			}
		}
		badSpendPct := 0.0
		if badId == adsetIds[0] {
			badSpendPct = m0.Spend / totalSpend
		} else {
			badSpendPct = m1.Spend / totalSpend
		}
		if badSpendPct < 0.6 {
			continue
		}

		// Đã cap rồi → skip, chờ logic gỡ cap xử lý
		if isAdSetCapped(ctx, camp.CampaignId, badId) {
			continue
		}

		cfg, _ := adsconfig.GetConfigForCampaign(ctx, camp.AdAccountId, camp.OwnerOrganizationID)
		killThreshold := adsconfig.GetThreshold(adsconfig.KeyCpaMessKill, cfg)
		if th, ok := adsadaptive.GetAdaptiveThreshold(ctx, adsconfig.KeyCpaMessKill, camp.CampaignId, camp.AdAccountId, camp.OwnerOrganizationID, cfg, now); ok {
			killThreshold = th
		}
		if killThreshold <= 0 {
			killThreshold = 180000
		}
		if badCpa <= killThreshold*0.9 || goodCpa >= killThreshold*0.6 {
			continue
		}

		campaignBudget := toFloat64Throttle(camp.MetaData, "daily_budget")
		if campaignBudget <= 0 {
			continue
		}
		capBudget := campaignBudget * 0.15
		if capBudget < 100 {
			capBudget = 100
		}
		capCents := int64(capBudget)

		eventID, err := Propose(ctx, &ProposeInput{
			ActionType:   "SET_BUDGET",
			AdAccountId:  camp.AdAccountId,
			CampaignId:   camp.CampaignId,
			AdSetId:      badId,
			Reason:       "Throttle — Ad Set tệ nhận >60% budget, cap 15%",
			RuleCode:     "throttle",
			Value:        capCents,
		}, camp.OwnerOrganizationID, getProposeBaseURL())
		if err != nil {
			continue
		}
		if eventID != "" {
			throttled++
			saveThrottleState(ctx, camp.CampaignId, badId, camp.AdAccountId, camp.OwnerOrganizationID, campaignBudget)
			log.WithFields(map[string]interface{}{
				"campaignId": camp.CampaignId,
				"adSetId":    badId,
				"cpaMess":    badCpa,
				"spendPct":   badSpendPct * 100,
			}).Info("⚙️ [THROTTLE] Đã cap Ad Set tệ 15%")
		}
	}
	return throttled, nil
}

// runThrottleRemoveCheck xử lý gỡ cap: CPA_Mess < adaptive×0.75x trong 2 checkpoint → Remove.
func runThrottleRemoveCheck(ctx context.Context) int {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsThrottleState)
	if !ok {
		return 0
	}
	insightColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return 0
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	dateEnd := now.Format("2006-01-02")
	dateStart := now.AddDate(0, 0, -7).Format("2006-01-02")
	minCappedAt := now.Add(-throttleRemoveMinHours * time.Hour).UnixMilli()

	cursor, err := coll.Find(ctx, bson.M{}, nil)
	if err != nil {
		return 0
	}
	defer cursor.Close(ctx)
	var removed int
	for cursor.Next(ctx) {
		var doc adsmodels.AdsThrottleState
		if cursor.Decode(&doc) != nil {
			continue
		}
		// Lấy CPA_Mess 7d của ad set
		cpaMess := getAdSetCpaMess7d(ctx, insightColl, doc.AdSetId, doc.AdAccountId, doc.OwnerOrganizationID, dateStart, dateEnd)
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, doc.AdAccountId, doc.OwnerOrganizationID)
		killThreshold := adsconfig.GetThreshold(adsconfig.KeyCpaMessKill, cfg)
		if th, ok := adsadaptive.GetAdaptiveThreshold(ctx, adsconfig.KeyCpaMessKill, doc.CampaignId, doc.AdAccountId, doc.OwnerOrganizationID, cfg, now); ok {
			killThreshold = th
		}
		if killThreshold <= 0 {
			killThreshold = 180000
		}
		threshold075 := killThreshold * 0.75

		if cpaMess < threshold075 {
			doc.CheckpointOkCount++
			doc.LastCheckpointAt = now.UnixMilli()
			_, _ = coll.UpdateOne(ctx, bson.M{"_id": doc.ID}, bson.M{
				"$set": bson.M{
					"checkpointOkCount": doc.CheckpointOkCount,
					"lastCheckpointAt":  doc.LastCheckpointAt,
				},
			})
			if doc.CheckpointOkCount >= 2 && doc.CappedAt <= minCappedAt {
				// Gỡ cap: set ad set budget = 50% campaign để FB redistribute
				removeBudget := doc.CampaignBudget * 0.5
				if removeBudget < 100 {
					removeBudget = 100
				}
				eventID, err := Propose(ctx, &ProposeInput{
					ActionType:   "SET_BUDGET",
					AdAccountId:  doc.AdAccountId,
					CampaignId:   doc.CampaignId,
					AdSetId:      doc.AdSetId,
					Reason:       "Throttle gỡ cap — Ad Set cải thiện CPA_Mess < adaptive×0.75x 2 checkpoint",
					RuleCode:     "throttle_remove",
					Value:        int64(removeBudget),
				}, doc.OwnerOrganizationID, getProposeBaseURL())
				if err == nil && eventID != "" {
					_, _ = coll.DeleteOne(ctx, bson.M{"_id": doc.ID})
					removed++
				}
			}
		} else {
			_, _ = coll.UpdateOne(ctx, bson.M{"_id": doc.ID}, bson.M{
				"$set": bson.M{"checkpointOkCount": 0},
			})
		}
	}
	return removed
}

// getAdSetCpaMess7d lấy CPA_Mess (spend/mess*1000) của ad set trong 7 ngày.
func getAdSetCpaMess7d(ctx context.Context, insightColl *mongo.Collection, adSetId, adAccountId string, ownerOrgID primitive.ObjectID, dateStart, dateEnd string) float64 {
	adAccountFilter := adAccountIdFilterForMeta(adAccountId)
	extractMess := bson.M{
		"$reduce": bson.M{
			"input": bson.M{"$ifNull": bson.A{"$metaData.actions", bson.A{}}},
			"initialValue": int64(0),
			"in": bson.M{
				"$add": bson.A{
					"$$value",
					bson.M{
						"$cond": bson.M{
							"if": bson.M{"$regexMatch": bson.M{
								"input": bson.M{"$toLower": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$$this.action_type", ""}}, ""}}},
								"regex": "messaging_conversation_started",
							}},
							"then": bson.M{"$convert": bson.M{"input": "$$this.value", "to": "long", "onError": 0, "onNull": 0}},
							"else": int64(0),
						},
					},
				},
			},
		},
	}
	extractSpend := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$spend", "0"}}, "to": "double", "onError": 0, "onNull": 0}}
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"objectType":          "adset",
			"objectId":            adSetId,
			"adAccountId":         adAccountFilter,
			"ownerOrganizationId": ownerOrgID,
			"dateStart":           bson.M{"$gte": dateStart, "$lte": dateEnd},
		}}},
		{{Key: "$addFields", Value: bson.M{"_spend": extractSpend, "_mess": extractMess}}},
		{{Key: "$group", Value: bson.M{
			"_id":   nil,
			"spend": bson.M{"$sum": "$_spend"},
			"mess":  bson.M{"$sum": "$_mess"},
		}}},
	}
	cur, err := insightColl.Aggregate(ctx, pipe)
	if err != nil {
		return 0
	}
	defer cur.Close(ctx)
	var doc struct {
		Spend float64 `bson:"spend"`
		Mess  float64 `bson:"mess"`
	}
	if cur.Next(ctx) && cur.Decode(&doc) == nil && doc.Mess > 0 {
		return doc.Spend / doc.Mess * 1000
	}
	return 0
}

// isAdSetCapped kiểm tra ad set đã bị cap chưa.
func isAdSetCapped(ctx context.Context, campaignId, adSetId string) bool {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsThrottleState)
	if !ok {
		return false
	}
	n, _ := coll.CountDocuments(ctx, bson.M{
		"campaignId": campaignId,
		"adSetId":    adSetId,
	})
	return n > 0
}

// saveThrottleState lưu trạng thái ad set đã cap.
func saveThrottleState(ctx context.Context, campaignId, adSetId, adAccountId string, ownerOrgID primitive.ObjectID, campaignBudget float64) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsThrottleState)
	if !ok {
		return
	}
	now := time.Now().UnixMilli()
	doc := adsmodels.AdsThrottleState{
		ID:                  primitive.NewObjectID(),
		CampaignId:          campaignId,
		AdSetId:             adSetId,
		AdAccountId:         adAccountId,
		OwnerOrganizationID: ownerOrgID,
		CappedAt:            now,
		CheckpointOkCount:   0,
		LastCheckpointAt:    0,
		CampaignBudget:      campaignBudget,
		CreatedAt:           now,
	}
	_, _ = coll.InsertOne(ctx, doc)
}

func toFloat64Throttle(m map[string]interface{}, k string) float64 {
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
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}
