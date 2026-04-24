// Package adaptive — Per-Camp Adaptive Threshold (FolkForm v4.1 Section 2.2).
// Tính P25/P50/P75 từ dữ liệu lịch sử campaign 14 ngày, trả về ngưỡng adaptive theo giai đoạn.
// Package độc lập để tránh import cycle (meta, ads/config đều dùng).
package adaptive

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	adsconfig "meta_commerce/internal/api/ads_meta/config"
	adsmodels "meta_commerce/internal/api/ads_meta/models"
	canonicalquery "meta_commerce/internal/api/order/canonicalquery"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// CampStage0Days camp mới < 7 ngày — dùng global
	CampStage0Days = 7
	// CampStage1Days 7–14 ngày — blended 50% global + 50% camp
	CampStage1Days = 14
	// CampStage2Days 14–30 ngày — Camp_P75 × multiplier
	CampStage2Days = 30
	// CampStage2MinMess giai đoạn 2 yêu cầu ≥20 mess/ngày trung bình
	CampStage2MinMess = 20
	// AdaptiveClampPct giới hạn ±30% so với global (FolkForm 2.2)
	AdaptiveClampPct = 0.30
	// CpaMessKillMultiplier Camp_CPA_Mess_P75 × 1.3x
	CpaMessKillMultiplier = 1.3
	// CpaPurchaseMultiplier Camp_CPA_Pur_P75 × 1.25x
	CpaPurchaseMultiplier = 1.25
	// ConvRateMessTrapMultiplier Camp_CR_P25 × 0.6x
	ConvRateMessTrapMultiplier = 0.6
	// CtrKillMultiplier Camp_CTR_P25 × 0.7x
	CtrKillMultiplier = 0.7
	// CpaMessSafetyNetMultiplier Camp_CPA_Mess_P50 × 1.1x
	CpaMessSafetyNetMultiplier = 1.1
)

// adAccountIdFilter trả về filter cho adAccountId (act_XXX hoặc XXX).
func adAccountIdFilter(adAccountId string) interface{} {
	if adAccountId == "" {
		return adAccountId
	}
	if strings.HasPrefix(adAccountId, "act_") {
		return bson.M{"$in": bson.A{adAccountId, strings.TrimPrefix(adAccountId, "act_")}}
	}
	return bson.M{"$in": bson.A{adAccountId, "act_" + adAccountId}}
}

// dailyCampMetric metric theo ngày cho campaign (từ meta + pancake).
type dailyCampMetric struct {
	Date         string  `bson:"date"`
	Spend        float64 `bson:"spend"`
	Mess         int64   `bson:"mess"`
	Orders       int64   `bson:"orders"`
	Ctr          float64 `bson:"ctr"`
	CpaMess      float64 `bson:"cpaMess"`
	CpaPurchase  float64 `bson:"cpaPurchase"`
	ConvRate     float64 `bson:"convRate"`
}

// getAdIdsForCampaign lấy danh sách adId thuộc campaign.
func getAdIdsForCampaign(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) ([]string, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ads")
	}
	filter := bson.M{
		"campaignId":          campaignId,
		"adAccountId":         adAccountIdFilter(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}
	opts := mongoopts.Find().SetProjection(bson.M{"adId": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var adIds []string
	for cursor.Next(ctx) {
		var doc struct {
			AdId string `bson:"adId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.AdId != "" {
			adIds = append(adIds, doc.AdId)
		}
	}
	return adIds, nil
}

// fetchDailyMetricsForCampaign lấy metrics theo ngày cho campaign (14 ngày).
func fetchDailyMetricsForCampaign(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, days int) ([]dailyCampMetric, error) {
	adIds, err := getAdIdsForCampaign(ctx, campaignId, adAccountId, ownerOrgID)
	if err != nil || len(adIds) == 0 {
		return nil, err
	}

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := end.AddDate(0, 0, -days)
	dateStart := start.Format("2006-01-02")
	dateStop := end.Format("2006-01-02")

	insightsColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ad_insights")
	}

	insightsPipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"objectType":          "ad",
			"objectId":            bson.M{"$in": adIds},
			"adAccountId":         adAccountIdFilter(adAccountId),
			"ownerOrganizationId": ownerOrgID,
			"dateStart":           bson.M{"$gte": dateStart, "$lte": dateStop},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"_mess": bson.M{"$reduce": bson.M{
				"input":    bson.M{"$ifNull": bson.A{"$metaData.actions", bson.A{}}},
				"initialValue": int64(0),
				"in": bson.M{
					"$add": bson.A{
						"$$value",
						bson.M{
							"$cond": bson.M{
								"if": bson.M{"$regexMatch": bson.M{"input": bson.M{"$toLower": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$$this.action_type", ""}}, ""}}}, "regex": "messaging_conversation_started"}},
								"then": bson.M{"$convert": bson.M{"input": "$$this.value", "to": "long", "onError": 0, "onNull": 0}},
								"else": int64(0),
							},
						},
					},
				},
			}},
			"_spend": bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$spend", "0"}}, "to": "double", "onError": 0, "onNull": 0}},
			"_ctr":   bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$metaData.ctr", "$ctr"}}, "0"}}, "to": "double", "onError": 0, "onNull": 0}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$dateStart",
			"spend": bson.M{"$sum": "$_spend"},
			"mess":  bson.M{"$sum": "$_mess"},
			"ctr":   bson.M{"$avg": "$_ctr"},
		}}},
	}

	insightsCursor, err := insightsColl.Aggregate(ctx, insightsPipe)
	if err != nil {
		return nil, fmt.Errorf("aggregate insights: %w", err)
	}
	defer insightsCursor.Close(ctx)

	metaByDate := make(map[string]dailyCampMetric)
	for insightsCursor.Next(ctx) {
		var doc struct {
			Id    string  `bson:"_id"`
			Spend float64 `bson:"spend"`
			Mess  int64   `bson:"mess"`
			Ctr   float64 `bson:"ctr"`
		}
		if err := insightsCursor.Decode(&doc); err != nil {
			continue
		}
		metaByDate[doc.Id] = dailyCampMetric{Date: doc.Id, Spend: doc.Spend, Mess: doc.Mess, Ctr: doc.Ctr}
	}

	ordersColl, errColl := canonicalquery.CollOrderCanonical()
	if errColl != nil {
		return nil, errColl
	}
	startSec := start.Unix()
	endSec := end.Unix() + 86400
	tw := canonicalquery.MatchInsertedAtExclusiveSlotOr(startSec, endSec)
	match := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"posData.ad_id":       bson.M{"$in": adIds},
		"$or":                 tw["$or"],
	}

	ordersPipe := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$addFields", Value: bson.M{
			"_ts": bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{bson.M{"$ifNull": bson.A{"$insertedAt", 0}}, 1e12}},
				"$insertedAt",
				bson.M{"$multiply": bson.A{bson.M{"$toLong": bson.M{"$ifNull": bson.A{"$insertedAt", 0}}}, 1000}},
			}},
		}}},
		{{Key: "$addFields", Value: bson.M{
			"_day": bson.M{"$dateToString": bson.M{
				"format": "%Y-%m-%d",
				"date":   bson.M{"$toDate": "$_ts"},
			}},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":    "$_day",
			"orders": bson.M{"$sum": 1},
		}}},
	}

	ordersCursor, err := ordersColl.Aggregate(ctx, ordersPipe)
	if err != nil {
		return nil, fmt.Errorf("aggregate orders: %w", err)
	}
	defer ordersCursor.Close(ctx)

	ordersByDate := make(map[string]int64)
	for ordersCursor.Next(ctx) {
		var doc struct {
			Id     string `bson:"_id"`
			Orders int64  `bson:"orders"`
		}
		if err := ordersCursor.Decode(&doc); err != nil {
			continue
		}
		ordersByDate[doc.Id] = doc.Orders
	}

	var out []dailyCampMetric
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		m := metaByDate[ds]
		m.Date = ds
		m.Orders = ordersByDate[ds]
		if m.Mess > 0 {
			m.CpaMess = m.Spend / float64(m.Mess)
		}
		if m.Orders > 0 {
			m.CpaPurchase = m.Spend / float64(m.Orders)
		}
		if m.Mess > 0 {
			m.ConvRate = float64(m.Orders) / float64(m.Mess) * 100
		}
		out = append(out, m)
	}
	return out, nil
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo < 0 {
		lo = 0
	}
	if hi >= len(sorted) {
		hi = len(sorted) - 1
	}
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (sorted[hi]-sorted[lo])*(idx-float64(lo))
}

func collectValues(daily []dailyCampMetric, getter func(d dailyCampMetric) float64) []float64 {
	var out []float64
	for _, d := range daily {
		v := getter(d)
		if v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			out = append(out, v)
		}
	}
	return out
}

// ComputeCampThresholds tính P25/P50/P75 từ dữ liệu 14 ngày và lưu vào ads_camp_thresholds.
func ComputeCampThresholds(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, metaCreatedAt int64) error {
	daily, err := fetchDailyMetricsForCampaign(ctx, campaignId, adAccountId, ownerOrgID, 14)
	if err != nil {
		return err
	}
	if len(daily) == 0 {
		return nil
	}

	var totalMess int64
	for _, d := range daily {
		totalMess += d.Mess
	}
	avgDailyMess := float64(totalMess) / float64(len(daily))

	computeP := func(getter func(d dailyCampMetric) float64) (p25, p50, p75 float64) {
		vals := collectValues(daily, getter)
		if len(vals) == 0 {
			return 0, 0, 0
		}
		sort.Float64s(vals)
		return percentile(vals, 25), percentile(vals, 50), percentile(vals, 75)
	}

	cpaMessP25, cpaMessP50, cpaMessP75 := computeP(func(d dailyCampMetric) float64 { return d.CpaMess })
	cpaPurP25, cpaPurP50, cpaPurP75 := computeP(func(d dailyCampMetric) float64 { return d.CpaPurchase })
	crP25, crP50, crP75 := computeP(func(d dailyCampMetric) float64 { return d.ConvRate })
	ctrP25, ctrP50, ctrP75 := computeP(func(d dailyCampMetric) float64 { return d.Ctr })

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := end.AddDate(0, 0, -14)
	dateStart := start.Format("2006-01-02")
	dateStop := end.Format("2006-01-02")

	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsCampThresholds)
	if !ok {
		return fmt.Errorf("không tìm thấy collection ads_camp_thresholds")
	}
	filter := bson.M{
		"campaignId":          campaignId,
		"adAccountId":         adAccountIdFilter(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}
	setDoc := bson.M{
		"campaignId":          campaignId,
		"adAccountId":         adAccountId,
		"ownerOrganizationId": ownerOrgID,
		"metaCreatedAt":       metaCreatedAt,
		"windowDays":          14,
		"cpaMessP25":          cpaMessP25,
		"cpaMessP50":          cpaMessP50,
		"cpaMessP75":          cpaMessP75,
		"cpaPurchaseP25":      cpaPurP25,
		"cpaPurchaseP50":      cpaPurP50,
		"cpaPurchaseP75":      cpaPurP75,
		"convRateP25":         crP25,
		"convRateP50":         crP50,
		"convRateP75":         crP75,
		"ctrP25":              ctrP25,
		"ctrP50":              ctrP50,
		"ctrP75":              ctrP75,
		"avgDailyMess":        avgDailyMess,
		"dateStart":           dateStart,
		"dateStop":            dateStop,
		"updatedAt":           now.UnixMilli(),
	}
	opts := mongoopts.Update().SetUpsert(true)
	update := bson.M{
		"$set":         setDoc,
		"$setOnInsert": bson.M{"createdAt": now.UnixMilli()},
	}
	_, err = coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
			"campaignId": campaignId, "adAccountId": adAccountId,
		}).Warn("[ADS] ComputeCampThresholds upsert thất bại")
		return err
	}
	return nil
}

// GetCampThresholds lấy document ads_camp_thresholds cho campaign.
func GetCampThresholds(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) (*adsmodels.AdsCampThresholds, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsCampThresholds)
	if !ok {
		return nil, nil
	}
	var doc adsmodels.AdsCampThresholds
	err := coll.FindOne(ctx, bson.M{
		"campaignId":          campaignId,
		"adAccountId":         adAccountIdFilter(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

// GetCampaignAgeDays trả về số ngày từ metaCreatedAt đến hiện tại.
func GetCampaignAgeDays(metaCreatedAt int64) int64 {
	if metaCreatedAt <= 0 {
		return 0
	}
	return (time.Now().UnixMilli() - metaCreatedAt) / (24 * 60 * 60 * 1000)
}

func clampAdaptive(val, global float64, pct float64) float64 {
	lo := global * (1 - pct)
	hi := global * (1 + pct)
	if val < lo {
		return lo
	}
	if val > hi {
		return hi
	}
	return val
}

// GetAdaptiveThreshold trả về ngưỡng adaptive theo campaign age và stage.
func GetAdaptiveThreshold(ctx context.Context, key string, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, cfg *adsmodels.CampaignConfigView, t time.Time) (float64, bool) {
	base := adsconfig.GetThresholdWithEventOverride(key, cfg, t)
	if campaignId == "" || adAccountId == "" {
		return base, false
	}

	metaCreatedAt, th, err := getCampaignMetaAndThresholds(ctx, campaignId, adAccountId, ownerOrgID)
	if err != nil || metaCreatedAt <= 0 {
		return base, false
	}
	ageDays := GetCampaignAgeDays(metaCreatedAt)

	if ageDays < CampStage0Days {
		return base, false
	}

	switch key {
	case adsconfig.KeyCpaMessKill, adsconfig.KeyCpaPurchaseHardStop, adsconfig.KeyConvRateMessTrap, adsconfig.KeyConvRateMessTrap6, adsconfig.KeyCtrKill:
		break
	default:
		return base, false
	}

	if ageDays < CampStage1Days {
		return blendStage1(key, base, th), true
	}

	if ageDays < CampStage2Days {
		if th == nil || th.AvgDailyMess < CampStage2MinMess {
			return base, false
		}
		adaptive := computeStage2(key, th, base)
		if adaptive >= 0 {
			return clampAdaptive(adaptive, base, AdaptiveClampPct), true
		}
		return base, false
	}

	if th == nil || th.AvgDailyMess < CampStage2MinMess {
		return base, false
	}
	adaptive := computeStage2(key, th, base)
	if adaptive >= 0 {
		return clampAdaptive(adaptive, base, AdaptiveClampPct), true
	}
	return base, false
}

func getCampaignMetaAndThresholds(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) (metaCreatedAt int64, th *adsmodels.AdsCampThresholds, err error) {
	campColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return 0, nil, nil
	}
	var campDoc struct {
		MetaCreatedAt int64 `bson:"metaCreatedAt"`
	}
	if err := campColl.FindOne(ctx, bson.M{
		"campaignId":         campaignId,
		"adAccountId":        adAccountIdFilter(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}, mongoopts.FindOne().SetProjection(bson.M{"metaCreatedAt": 1})).Decode(&campDoc); err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	metaCreatedAt = campDoc.MetaCreatedAt
	th, err = GetCampThresholds(ctx, campaignId, adAccountId, ownerOrgID)
	return metaCreatedAt, th, err
}

func blendStage1(key string, global float64, th *adsmodels.AdsCampThresholds) float64 {
	if th == nil {
		return global
	}
	var campAvg float64
	switch key {
	case adsconfig.KeyCpaMessKill:
		campAvg = th.CpaMessP50
	case adsconfig.KeyCpaPurchaseHardStop:
		campAvg = th.CpaPurchaseP50
	case adsconfig.KeyConvRateMessTrap, adsconfig.KeyConvRateMessTrap6:
		campAvg = th.ConvRateP50
	case adsconfig.KeyCtrKill:
		campAvg = th.CtrP50
	default:
		return global
	}
	if campAvg <= 0 {
		return global
	}
	return 0.5*global + 0.5*campAvg
}

func computeStage2(key string, th *adsmodels.AdsCampThresholds, global float64) float64 {
	switch key {
	case adsconfig.KeyCpaMessKill:
		if th.CpaMessP75 <= 0 {
			return -1
		}
		return th.CpaMessP75 * CpaMessKillMultiplier
	case adsconfig.KeyCpaPurchaseHardStop:
		if th.CpaPurchaseP75 <= 0 {
			return -1
		}
		return th.CpaPurchaseP75 * CpaPurchaseMultiplier
	case adsconfig.KeyConvRateMessTrap, adsconfig.KeyConvRateMessTrap6:
		if th.ConvRateP25 <= 0 {
			return -1
		}
		v := th.ConvRateP25 * ConvRateMessTrapMultiplier / 100
		cap5 := 0.05
		if v < cap5 {
			return v
		}
		return cap5
	case adsconfig.KeyCtrKill:
		if th.CtrP25 <= 0 {
			return -1
		}
		return th.CtrP25 * CtrKillMultiplier / 100
	default:
		return -1
	}
}
