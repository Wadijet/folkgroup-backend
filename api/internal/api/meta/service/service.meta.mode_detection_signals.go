// Package metasvc — Helpers cho Mode Detection S1–S5 (FolkForm v4.1 Section 3.1).
// Chạy 07:30 mỗi sáng. Các hàm trả về (value, ok); ok=false khi thiếu dữ liệu → signal bỏ qua (0 điểm).
package metasvc

import (
	"context"
	"time"

	adsconfig "meta_commerce/internal/api/ads/config"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetROASYesterday S1: ROAS Pancake hôm qua = revenue / spend. Nguồn: meta_ad_insights (spend) + pc_pos_orders (revenue).
func GetROASYesterday(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (roas float64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	yesterday := now.AddDate(0, 0, -1)
	startOfDay := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.Add(24*time.Hour - time.Second)
	startSec := startOfDay.Unix()
	endSec := endOfDay.Unix()

	// Spend từ meta_ad_insights (ad_account, yesterday)
	insightColl, okInsight := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !okInsight {
		return 0, false
	}
	dateStr := yesterday.Format("2006-01-02")
	var insightDoc struct {
		Spend string `bson:"spend"`
	}
	err := insightColl.FindOne(ctx, bson.M{
		"objectType":          "ad_account",
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"dateStart":           dateStr,
	}).Decode(&insightDoc)
	if err != nil {
		return 0, false
	}
	spend := parseFloat(insightDoc.Spend)
	if spend <= 0 {
		return 0, false
	}

	// Revenue từ pc_pos_orders — aggregate theo tất cả ad_ids thuộc account
	adIds, okAds := getAdIdsForAccount(ctx, adAccountId, ownerOrgID)
	if !okAds || len(adIds) == 0 {
		return 0, false
	}

	posColl, okPos := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !okPos {
		return 0, false
	}
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"posData.ad_id":       bson.M{"$in": adIds},
			"$or": []bson.M{
				{"posCreatedAt": bson.M{"$gte": startSec, "$lte": endSec}},
				{"insertedAt": bson.M{"$gte": startSec, "$lte": endSec}},
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":     nil,
			"revenue": bson.M{"$sum": bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$posData.total_price_after_sub_discount", 0}}, "to": "double", "onError": 0, "onNull": 0}}},
		}}},
	}
	cursor, err := posColl.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)
	var revDoc struct {
		Revenue float64 `bson:"revenue"`
	}
	if !cursor.Next(ctx) {
		return 0, false
	}
	if err := cursor.Decode(&revDoc); err != nil {
		return 0, false
	}
	revenue := revDoc.Revenue
	if revenue <= 0 {
		return 0, false
	}
	return revenue / spend, true
}

// GetCPMSang0730 S2: CPM khung 07:00–07:30 hôm nay. CPM = spend / (impressions/1000).
func GetCPMSang0730(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (cpm float64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), 7, 30, 0, 0, loc)
	spend, impressions, ok := GetSpendFor30pSlot(ctx, adAccountId, ownerOrgID, today, endSlot.UnixMilli())
	if !ok || impressions <= 0 {
		return 0, false
	}
	return spend * 1000 / impressions, true
}

// GetMess0730ForDate S3: Đếm mess (fb_conversations) khung 07:00–07:30 cho ngày date.
func GetMess0730ForDate(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, date time.Time) (mess int64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	startSlot := time.Date(date.Year(), date.Month(), date.Day(), 7, 0, 0, 0, loc)
	endSlot := time.Date(date.Year(), date.Month(), date.Day(), 7, 30, 0, 0, loc)
	startMs := startSlot.UnixMilli()
	endMs := endSlot.UnixMilli()

	adIds, okAds := getAdIdsForAccount(ctx, adAccountId, ownerOrgID)
	if !okAds || len(adIds) == 0 {
		return 0, false
	}

	coll, okColl := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !okColl {
		return 0, false
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
					"vars": bson.M{
						"parsed": bson.M{"$dateFromString": bson.M{
							"dateString": bson.M{"$substr": bson.A{"$panCakeData.inserted_at", 0, 19}},
							"format":    "%Y-%m-%dT%H:%M:%S",
							"onError":   nil,
							"onNull":    nil,
						}},
					},
					"in": bson.M{"$cond": bson.A{
						bson.M{"$eq": bson.A{"$$parsed", nil}},
						nil,
						bson.M{"$toLong": "$$parsed"},
					}},
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
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"$or": []bson.M{
				{"panCakeData.ad_ids": bson.M{"$in": adIds}},
				{"panCakeData.ads.ad_id": bson.M{"$in": adIds}},
			},
		}}},
		{{Key: "$addFields", Value: bson.M{"_convTsMs": convTsMs}}},
		{{Key: "$match", Value: bson.M{
			"_convTsMs": bson.M{"$ne": nil, "$gte": startMs, "$lte": endMs},
		}}},
		{{Key: "$count", Value: "n"}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)
	var doc struct {
		N int64 `bson:"n"`
	}
	if cursor.Next(ctx) && cursor.Decode(&doc) == nil {
		return doc.N, true
	}
	return 0, false
}

// GetMessForCampaignSlot đếm mess (fb_conversations) cho campaign trong khung giờ [startHour, endHour) ngày date.
// Dùng cho PATCH 04 Window Shopping Pattern: Mess_07-12h, Mess_yesterday_07-12h.
func GetMessForCampaignSlot(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, date time.Time, startHour, endHour int) (mess int64, ok bool) {
	adIds, okAds := GetAdIdsForCampaign(ctx, campaignId, adAccountId, ownerOrgID)
	if !okAds || len(adIds) == 0 {
		return 0, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	startSlot := time.Date(date.Year(), date.Month(), date.Day(), startHour, 0, 0, 0, loc)
	endSlot := time.Date(date.Year(), date.Month(), date.Day(), endHour, 0, 0, 0, loc)
	startMs := startSlot.UnixMilli()
	endMs := endSlot.UnixMilli()

	coll, okColl := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !okColl {
		return 0, false
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
					"vars": bson.M{
						"parsed": bson.M{"$dateFromString": bson.M{
							"dateString": bson.M{"$substr": bson.A{"$panCakeData.inserted_at", 0, 19}},
							"format":    "%Y-%m-%dT%H:%M:%S",
							"onError":   nil,
							"onNull":    nil,
						}},
					},
					"in": bson.M{"$cond": bson.A{
						bson.M{"$eq": bson.A{"$$parsed", nil}},
						nil,
						bson.M{"$toLong": "$$parsed"},
					}},
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
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"$or": []bson.M{
				{"panCakeData.ad_ids": bson.M{"$in": adIds}},
				{"panCakeData.ads.ad_id": bson.M{"$in": adIds}},
			},
		}}},
		{{Key: "$addFields", Value: bson.M{"_convTsMs": convTsMs}}},
		{{Key: "$match", Value: bson.M{
			"_convTsMs": bson.M{"$ne": nil, "$gte": startMs, "$lt": endMs},
		}}},
		{{Key: "$count", Value: "n"}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)
	var doc struct {
		N int64 `bson:"n"`
	}
	if cursor.Next(ctx) && cursor.Decode(&doc) == nil {
		return doc.N, true
	}
	return 0, false
}

// GetOrdersForCampaignSlot đếm đơn Pancake cho campaign trong khung giờ [startHour, endHour) ngày date.
// Dùng cho PATCH 04 Window Shopping Pattern: CR_07-12h, CR_yesterday_12-22h.
func GetOrdersForCampaignSlot(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, date time.Time, startHour, endHour int) (orders int64, ok bool) {
	adIds, ok := GetAdIdsForCampaign(ctx, campaignId, adAccountId, ownerOrgID)
	if !ok || len(adIds) == 0 {
		return 0, false
	}
	posColl, okPos := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !okPos {
		return 0, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	startSlot := time.Date(date.Year(), date.Month(), date.Day(), startHour, 0, 0, 0, loc)
	endSlot := time.Date(date.Year(), date.Month(), date.Day(), endHour, 0, 0, 0, loc)
	startSec := startSlot.Unix()
	endSec := endSlot.Unix()

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"posData.ad_id":       bson.M{"$in": adIds},
			"$or": []bson.M{
				{"posCreatedAt": bson.M{"$gte": startSec, "$lt": endSec}},
				{"insertedAt": bson.M{"$gte": startSec, "$lt": endSec}},
			},
		}}},
		{{Key: "$count", Value: "n"}},
	}
	cursor, err := posColl.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)
	var doc struct {
		N int64 `bson:"n"`
	}
	if cursor.Next(ctx) && cursor.Decode(&doc) == nil {
		return doc.N, true
	}
	return 0, false
}

// GetWindowShoppingInputs PATCH 04: Thu thập dữ liệu cho Rule Engine (Interpretation Rule window_shopping_pattern).
// Trả về params để truyền vào Rule; ok=false khi thiếu dữ liệu.
func GetWindowShoppingInputs(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) (params map[string]interface{}, ok bool) {
	if campaignId == "" || adAccountId == "" {
		return nil, false
	}
	inEvent, _, _ := adsconfig.IsEventWindow(time.Now())
	params = map[string]interface{}{
		"in_event_window": inEvent,
	}
	if !inEvent {
		return params, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	today := now
	yesterday := now.AddDate(0, 0, -1)

	messToday0712, ok1 := GetMessForCampaignSlot(ctx, campaignId, adAccountId, ownerOrgID, today, 7, 12)
	if !ok1 || messToday0712 == 0 {
		return params, false
	}
	messYesterday0712, ok2 := GetMessForCampaignSlot(ctx, campaignId, adAccountId, ownerOrgID, yesterday, 7, 12)
	if !ok2 || messYesterday0712 == 0 {
		return params, false
	}
	ordersToday0712, _ := GetOrdersForCampaignSlot(ctx, campaignId, adAccountId, ownerOrgID, today, 7, 12)
	messYesterday1222, ok3 := GetMessForCampaignSlot(ctx, campaignId, adAccountId, ownerOrgID, yesterday, 12, 22)
	if !ok3 || messYesterday1222 == 0 {
		return params, false
	}
	ordersYesterday1222, _ := GetOrdersForCampaignSlot(ctx, campaignId, adAccountId, ownerOrgID, yesterday, 12, 22)

	params["mess_07_12_today"] = float64(messToday0712)
	params["mess_07_12_yesterday"] = float64(messYesterday0712)
	params["orders_07_12_today"] = float64(ordersToday0712)
	params["mess_12_22_yesterday"] = float64(messYesterday1222)
	params["orders_12_22_yesterday"] = float64(ordersYesterday1222)
	return params, true
}

// GetMonthlyRevenuePace S4: Pace = revenue_so_far / (target × days_elapsed/total_days). target tính triệu VNĐ.
func GetMonthlyRevenuePace(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, monthlyTarget float64) (pace float64, ok bool) {
	if monthlyTarget <= 0 {
		return 0, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Second)
	totalDays := endOfMonth.Day()
	daysElapsed := now.Day()
	if now.Hour() < 7 || (now.Hour() == 7 && now.Minute() < 30) {
		daysElapsed-- // Trước 07:30 coi như chưa qua ngày hôm nay
	}
	if daysElapsed <= 0 {
		return 0, false
	}
	expectedRevenue := monthlyTarget * 1e6 * float64(daysElapsed) / float64(totalDays) // target triệu → VNĐ
	if expectedRevenue <= 0 {
		return 0, false
	}

	adIds, okAds := getAdIdsForAccount(ctx, adAccountId, ownerOrgID)
	if !okAds || len(adIds) == 0 {
		return 0, false
	}
	posColl, okPos := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !okPos {
		return 0, false
	}
	startSec := startOfMonth.Unix()
	endSec := now.Unix()
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"posData.ad_id":       bson.M{"$in": adIds},
			"$or": []bson.M{
				{"posCreatedAt": bson.M{"$gte": startSec, "$lte": endSec}},
				{"insertedAt": bson.M{"$gte": startSec, "$lte": endSec}},
			},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":     nil,
			"revenue": bson.M{"$sum": bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$posData.total_price_after_sub_discount", 0}}, "to": "double", "onError": 0, "onNull": 0}}},
		}}},
	}
	cursor, err := posColl.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)
	var revDoc struct {
		Revenue float64 `bson:"revenue"`
	}
	if !cursor.Next(ctx) {
		return 0, false
	}
	if err := cursor.Decode(&revDoc); err != nil {
		return 0, false
	}
	revenueSoFar := revDoc.Revenue
	return revenueSoFar / expectedRevenue, true
}

// GetCHSAccountAvg S5: Trung bình CHS của tất cả campaign active. CHS từ currentMetrics.layer3.chs.
func GetCHSAccountAvg(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (avgChs float64, ok bool) {
	coll, okColl := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !okColl {
		return 0, false
	}
	filter := bson.M{
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"effectiveStatus":     bson.M{"$in": bson.A{"ACTIVE", "CAMPAIGN_PAUSED"}}, // Đang chạy hoặc tạm dừng (vẫn có metrics)
	}
	cursor, err := coll.Find(ctx, filter, nil)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)

	var total float64
	var count int
	for cursor.Next(ctx) {
		var doc struct {
			CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		layer3, _ := doc.CurrentMetrics["layer3"].(map[string]interface{})
		if layer3 == nil {
			continue
		}
		chs := toFloat64FromMap(layer3, "chs")
		if chs > 0 {
			total += chs
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

// GetAdIdsForCampaign lấy danh sách ad_id thuộc campaign (dùng cho Predictive Trend CR Decay).
func GetAdIdsForCampaign(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) ([]string, bool) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return nil, false
	}
	cursor, err := coll.Find(ctx, bson.M{
		"campaignId":          campaignId,
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}, nil)
	if err != nil {
		return nil, false
	}
	defer cursor.Close(ctx)
	var ids []string
	for cursor.Next(ctx) {
		var doc struct {
			AdId string `bson:"adId"`
		}
		if err := cursor.Decode(&doc); err != nil || doc.AdId == "" {
			continue
		}
		ids = append(ids, doc.AdId)
	}
	return ids, len(ids) > 0
}

// GetCampaignDailyOrdersMap trả về map[date]orders — số đơn Pancake theo ngày cho campaign (7 ngày gần nhất).
// Dùng cho Predictive Trend Conv Rate Decay.
func GetCampaignDailyOrdersMap(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, dateStart, dateEnd string) (map[string]int64, bool) {
	adIds, ok := GetAdIdsForCampaign(ctx, campaignId, adAccountId, ownerOrgID)
	if !ok || len(adIds) == 0 {
		return nil, false
	}
	posColl, okPos := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !okPos {
		return nil, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	startT, _ := time.ParseInLocation("2006-01-02", dateStart, loc)
	endT, _ := time.ParseInLocation("2006-01-02", dateEnd, loc)
	startSec := startT.Unix()
	endSec := endT.Add(24*time.Hour - time.Second).Unix()
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"posData.ad_id":       bson.M{"$in": adIds},
			"$or": []bson.M{
				{"posCreatedAt": bson.M{"$gte": startSec, "$lte": endSec}},
				{"insertedAt": bson.M{"$gte": startSec, "$lte": endSec}},
			},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"_ts": bson.M{
				"$cond": bson.M{
					"if":   bson.M{"$gt": bson.A{bson.M{"$ifNull": bson.A{"$posCreatedAt", 0}}, 0}},
					"then": bson.M{"$multiply": bson.A{"$posCreatedAt", 1000}},
					"else": bson.M{"$multiply": bson.A{bson.M{"$ifNull": bson.A{"$insertedAt", 0}}, 1000}},
				},
			},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"_date": bson.M{"$dateToString": bson.M{"format": "%Y-%m-%d", "date": bson.M{"$toDate": "$_ts"}}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$_date",
			"count": bson.M{"$sum": 1},
		}}},
	}
	cursor, err := posColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, false
	}
	defer cursor.Close(ctx)
	out := make(map[string]int64)
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if cursor.Decode(&doc) == nil {
			out[doc.ID] = doc.Count
		}
	}
	return out, len(out) > 0
}

func getAdIdsForAccount(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) ([]string, bool) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return nil, false
	}
	cursor, err := coll.Find(ctx, bson.M{
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}, nil)
	if err != nil {
		return nil, false
	}
	defer cursor.Close(ctx)
	var ids []string
	for cursor.Next(ctx) {
		var doc struct {
			AdId string `bson:"adId"`
		}
		if err := cursor.Decode(&doc); err != nil || doc.AdId == "" {
			continue
		}
		ids = append(ids, doc.AdId)
	}
	return ids, len(ids) > 0
}

// GetMessForAccountLast1h đếm mess (fb_conversations) cho account trong 1h gần nhất. Dùng cho PATCH 03 [HB-3] Divergence.
func GetMessForAccountLast1h(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (mess int64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
	startSlot := endSlot.Add(-1 * time.Hour)
	return getMessForAccountSlot(ctx, adAccountId, ownerOrgID, startSlot.UnixMilli(), endSlot.UnixMilli())
}

// GetOrdersForAccountLast1h đếm đơn Pancake cho account trong 1h gần nhất. Dùng cho PATCH 03 [HB-3] Divergence.
func GetOrdersForAccountLast1h(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (orders int64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
	startSlot := endSlot.Add(-1 * time.Hour)
	return getOrdersForAccountSlot(ctx, adAccountId, ownerOrgID, startSlot.Unix(), endSlot.Unix())
}

// GetOrdersForAccountYesterdaySameHour đếm đơn Pancake cho account trong 1h tương ứng hôm qua. Dùng cho [HB-3].
func GetOrdersForAccountYesterdaySameHour(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (orders int64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	yesterday := now.AddDate(0, 0, -1)
	// Cùng giờ: h-1 đến h hôm qua (vd: 9h-10h)
	startSlot := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), now.Hour()-1, 0, 0, 0, loc)
	if now.Hour() == 0 {
		startSlot = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 0, 0, 0, loc)
	}
	endSlot := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), now.Hour(), 0, 0, 0, loc)
	if now.Hour() == 0 {
		endSlot = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, loc).Add(24 * time.Hour)
	}
	return getOrdersForAccountSlot(ctx, adAccountId, ownerOrgID, startSlot.Unix(), endSlot.Unix())
}

func getMessForAccountSlot(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, startMs, endMs int64) (mess int64, ok bool) {
	adIds, okAds := getAdIdsForAccount(ctx, adAccountId, ownerOrgID)
	if !okAds || len(adIds) == 0 {
		return 0, false
	}
	coll, okColl := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !okColl {
		return 0, false
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
					"vars": bson.M{
						"parsed": bson.M{"$dateFromString": bson.M{
							"dateString": bson.M{"$substr": bson.A{"$panCakeData.inserted_at", 0, 19}},
							"format":    "%Y-%m-%dT%H:%M:%S",
							"onError":   nil,
							"onNull":    nil,
						}},
					},
					"in": bson.M{"$cond": bson.A{
						bson.M{"$eq": bson.A{"$$parsed", nil}},
						nil,
						bson.M{"$toLong": "$$parsed"},
					}},
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
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"ownerOrganizationId": ownerOrgID,
			"$or": []bson.M{
				{"panCakeData.ad_ids": bson.M{"$in": adIds}},
				{"panCakeData.ads.ad_id": bson.M{"$in": adIds}},
			},
		}}},
		{{Key: "$addFields", Value: bson.M{"_convTsMs": convTsMs}}},
		{{Key: "$match", Value: bson.M{
			"_convTsMs": bson.M{"$ne": nil, "$gte": startMs, "$lt": endMs},
		}}},
		{{Key: "$count", Value: "n"}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)
	var doc struct {
		N int64 `bson:"n"`
	}
	if cursor.Next(ctx) && cursor.Decode(&doc) == nil {
		return doc.N, true
	}
	return 0, false
}

func getOrdersForAccountSlot(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, startSec, endSec int64) (orders int64, ok bool) {
	adIds, ok := getAdIdsForAccount(ctx, adAccountId, ownerOrgID)
	if !ok || len(adIds) == 0 {
		return 0, false
	}
	posColl, okPos := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !okPos {
		return 0, false
	}
	n, err := posColl.CountDocuments(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"posData.ad_id":       bson.M{"$in": adIds},
		"$or": []bson.M{
			{"posCreatedAt": bson.M{"$gte": startSec, "$lt": endSec}},
			{"insertedAt": bson.M{"$gte": startSec, "$lt": endSec}},
		},
	})
	if err != nil {
		return 0, false
	}
	return n, true
}

func toFloat64FromMap(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}
