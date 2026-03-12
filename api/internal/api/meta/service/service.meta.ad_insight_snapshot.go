// Package metasvc — Lưu snapshot daily insights mỗi lần sync. So sánh mới vs cũ → suy ra hourly/30p.
package metasvc

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	metamodels "meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func adAccountIdFilterForSnapshots(adAccountId string) interface{} {
	if adAccountId == "" {
		return adAccountId
	}
	if strings.HasPrefix(adAccountId, "act_") {
		return bson.M{"$in": bson.A{adAccountId, strings.TrimPrefix(adAccountId, "act_")}}
	}
	return bson.M{"$in": bson.A{adAccountId, "act_" + adAccountId}}
}

// SaveDailySnapshot lưu snapshot cumulative "today" khi agent sync insights (15p/lần).
// Chỉ lưu khi dateStart là ngày hiện tại (insights date_preset=today).
// Dùng để so sánh snapshot mới vs cũ → suy ra spend/impressions theo từng giờ hoặc 30p.
func SaveDailySnapshot(ctx context.Context, doc *metamodels.MetaAdInsight) error {
	if doc == nil || doc.DateStart == "" {
		return nil
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	today := time.Now().In(loc).Format("2006-01-02")
	if doc.DateStart != today {
		return nil // Chỉ lưu snapshot cho ngày hiện tại
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsightsDailySnapshots)
	if !ok {
		return fmt.Errorf("không tìm thấy collection meta_ad_insights_daily_snapshots")
	}
	now := time.Now()
	nowMs := now.UnixMilli()
	snap := metamodels.MetaAdInsightDailySnapshot{
		ObjectId:            doc.ObjectId,
		ObjectType:          doc.ObjectType,
		AdAccountId:         doc.AdAccountId,
		OwnerOrganizationID: doc.OwnerOrganizationID,
		Date:                doc.DateStart,
		SnapshotAt:          nowMs,
		Spend:               parseFloat(doc.Spend),
		Impressions:         parseInt64(doc.Impressions),
		Clicks:              parseInt64(doc.Clicks),
		Reach:               parseInt64(doc.Reach),
		Cpm:                 parseFloat(doc.Cpm),
		Ctr:                 parseFloat(doc.Ctr),
		Cpc:                 parseFloat(doc.Cpc),
		CreatedAt:           nowMs,
		ExpiresAt:           now.Add(7 * 24 * time.Hour), // TTL 7 ngày
	}
	_, err := coll.InsertOne(ctx, snap)
	return err
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// GetHourlySpendFromSnapshots suy ra spend theo giờ từ snapshots.
// Trả về map[hour]spend — hour là 0-23 (giờ bắt đầu). spend = tổng delta trong giờ đó.
// Delta giữa 2 snapshot liên tiếp được gán vào giờ của snapshot trước.
func GetHourlySpendFromSnapshots(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, date string) (map[int]float64, error) {
	return getHourlySpendFromSnapshotsWithFilter(ctx, bson.M{
		"objectType":          "ad_account",
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"date":                date,
	})
}

// GetHourlySpendFromSnapshotsForCampaign suy ra spend theo giờ từ snapshots (campaign level).
// Dùng cho Hourly Peak Matrix — tính peak hours từ spend distribution.
func GetHourlySpendFromSnapshotsForCampaign(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, date string) (map[int]float64, error) {
	return getHourlySpendFromSnapshotsWithFilter(ctx, bson.M{
		"objectType":          "campaign",
		"objectId":            campaignId,
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"date":                date,
	})
}

func getHourlySpendFromSnapshotsWithFilter(ctx context.Context, filter bson.M) (map[int]float64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsightsDailySnapshots)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ad_insights_daily_snapshots")
	}
	cursor, err := coll.Find(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var snaps []struct {
		SnapshotAt int64   `bson:"snapshotAt"`
		Spend      float64 `bson:"spend"`
	}
	for cursor.Next(ctx) {
		var r struct {
			SnapshotAt int64   `bson:"snapshotAt"`
			Spend      float64 `bson:"spend"`
		}
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		snaps = append(snaps, r)
	}
	if len(snaps) < 2 {
		return map[int]float64{}, nil
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].SnapshotAt < snaps[j].SnapshotAt })

	hourly := make(map[int]float64)
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	for i := 1; i < len(snaps); i++ {
		prev := snaps[i-1]
		curr := snaps[i]
		t := time.UnixMilli(prev.SnapshotAt).In(loc)
		hour := t.Hour()
		delta := curr.Spend - prev.Spend
		if delta < 0 {
			delta = 0
		}
		hourly[hour] += delta
	}
	return hourly, nil
}

// GetSpendFor30pSlot suy ra spend trong khung 30p (startSlotMs, endSlotMs] từ snapshots.
// endSlotMs = thời điểm kết thúc slot (vd: 14:30:00). startSlotMs = endSlotMs - 30min.
// Trả về (spend, impressions, ok). ok=false khi không đủ 2 snapshot.
func GetSpendFor30pSlot(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, date string, endSlotMs int64) (spend, impressions float64, ok bool) {
	startSlotMs := endSlotMs - 30*60*1000
	coll, okColl := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsightsDailySnapshots)
	if !okColl {
		return 0, 0, false
	}
	filter := bson.M{
		"objectType":          "ad_account",
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"date":                date,
		"snapshotAt":          bson.M{"$lte": endSlotMs},
	}
	cursor, err := coll.Find(ctx, filter, nil)
	if err != nil {
		return 0, 0, false
	}
	defer cursor.Close(ctx)

	type snapRow struct {
		SnapshotAt  int64   `bson:"snapshotAt"`
		Spend       float64 `bson:"spend"`
		Impressions int64   `bson:"impressions"`
	}
	var snaps []snapRow
	for cursor.Next(ctx) {
		var r snapRow
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		snaps = append(snaps, r)
	}
	if len(snaps) == 0 {
		return 0, 0, false
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].SnapshotAt < snaps[j].SnapshotAt })

	idxEnd, idxStart := -1, -1
	for i := len(snaps) - 1; i >= 0; i-- {
		if snaps[i].SnapshotAt <= endSlotMs && idxEnd < 0 {
			idxEnd = i
		}
		if snaps[i].SnapshotAt <= startSlotMs && idxStart < 0 {
			idxStart = i
			break
		}
	}
	if idxEnd < 0 || idxStart < 0 {
		return 0, 0, false
	}
	spend = snaps[idxEnd].Spend - snaps[idxStart].Spend
	if spend < 0 {
		spend = 0
	}
	imp := snaps[idxEnd].Impressions - snaps[idxStart].Impressions
	if imp < 0 {
		imp = 0
	}
	impressions = float64(imp)
	return spend, impressions, true
}

// GetSpendImpressions30pCurrentSlot suy ra spend và impressions cho slot 30p hiện tại.
// Trả về (spend, impressions, ok). CPM_30p = spend / (impressions/1000) khi impressions > 0.
func GetSpendImpressions30pCurrentSlot(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (spend, impressions float64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	m := now.Minute() / 30 * 30
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), m, 0, 0, loc)
	today := now.Format("2006-01-02")
	return GetSpendFor30pSlot(ctx, adAccountId, ownerOrgID, today, endSlot.UnixMilli())
}

// GetSpend30pAndYesterday suy ra spend_30p (hôm nay) và spend_yesterday_cùng_30p từ snapshots.
// Slot 30p align theo boundary: now=14:47 → slot 14:00-14:30.
// Trả về (spend30p, spendYesterdayCung30p, ok). ok=false khi không đủ data.
func GetSpend30pAndYesterday(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (spend30p, spendYesterday float64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	m := now.Minute() / 30 * 30
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), m, 0, 0, loc)
	endSlotMs := endSlot.UnixMilli()

	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	spend30p, _, okToday := GetSpendFor30pSlot(ctx, adAccountId, ownerOrgID, today, endSlotMs)
	if !okToday {
		return 0, 0, false
	}
	spendYesterday, _, okYest := GetSpendFor30pSlot(ctx, adAccountId, ownerOrgID, yesterday, endSlotMs)
	if !okYest {
		return spend30p, 0, true // Có spend_30p hôm nay, không có yesterday → vẫn dùng được
	}
	return spend30p, spendYesterday, true
}

// GetSpendImpressions1h suy ra spend và impressions 1h gần nhất từ snapshots.
// CPM_1h = spend / (impressions/1000) khi impressions > 0.
func GetSpendImpressions1h(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (spend, impressions float64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
	endSlotMs := endSlot.UnixMilli()
	startSlotMs := endSlotMs - 60*60*1000
	today := now.Format("2006-01-02")
	return getSpendImpressionsForSlot(ctx, adAccountId, ownerOrgID, today, startSlotMs, endSlotMs)
}

// GetSpendImpressions1hAgo suy ra spend và impressions 1h trước đó (2h ago → 1h ago).
func GetSpendImpressions1hAgo(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (spend, impressions float64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
	endSlotMs := endSlot.UnixMilli()
	startSlotMs := endSlotMs - 2*60*60*1000
	today := now.Format("2006-01-02")
	return getSpendImpressionsForSlot(ctx, adAccountId, ownerOrgID, today, startSlotMs, endSlotMs)
}

func getSpendImpressionsForSlot(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, date string, startSlotMs, endSlotMs int64) (spend, impressions float64, ok bool) {
	return getSpendImpressionsForSlotWithFilter(ctx, bson.M{
		"objectType":          "ad_account",
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"date":                date,
		"snapshotAt":          bson.M{"$lte": endSlotMs},
	}, startSlotMs, endSlotMs)
}

// GetSpendImpressions1hForCampaign suy ra spend và impressions 1h gần nhất cho campaign.
func GetSpendImpressions1hForCampaign(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) (spend, impressions float64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
	endSlotMs := endSlot.UnixMilli()
	startSlotMs := endSlotMs - 60*60*1000
	today := now.Format("2006-01-02")
	return getSpendImpressionsForSlotWithFilter(ctx, bson.M{
		"objectType":          "campaign",
		"objectId":            campaignId,
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"date":                today,
		"snapshotAt":          bson.M{"$lte": endSlotMs},
	}, startSlotMs, endSlotMs)
}

// GetSpendImpressions1hAgoForCampaign suy ra spend và impressions 1h trước cho campaign.
func GetSpendImpressions1hAgoForCampaign(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) (spend, impressions float64, ok bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
	endSlotMs := endSlot.UnixMilli()
	startSlotMs := endSlotMs - 2*60*60*1000
	today := now.Format("2006-01-02")
	return getSpendImpressionsForSlotWithFilter(ctx, bson.M{
		"objectType":          "campaign",
		"objectId":            campaignId,
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"date":                today,
		"snapshotAt":          bson.M{"$lte": endSlotMs},
	}, startSlotMs, endSlotMs)
}

func getSpendImpressionsForSlotWithFilter(ctx context.Context, filter bson.M, startSlotMs, endSlotMs int64) (spend, impressions float64, ok bool) {
	coll, okColl := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsightsDailySnapshots)
	if !okColl {
		return 0, 0, false
	}
	cursor, err := coll.Find(ctx, filter, nil)
	if err != nil {
		return 0, 0, false
	}
	defer cursor.Close(ctx)
	var snaps []struct {
		SnapshotAt  int64   `bson:"snapshotAt"`
		Spend       float64 `bson:"spend"`
		Impressions int64   `bson:"impressions"`
	}
	for cursor.Next(ctx) {
		var r struct {
			SnapshotAt  int64   `bson:"snapshotAt"`
			Spend       float64 `bson:"spend"`
			Impressions int64   `bson:"impressions"`
		}
		if cursor.Decode(&r) == nil {
			snaps = append(snaps, r)
		}
	}
	if len(snaps) == 0 {
		return 0, 0, false
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].SnapshotAt < snaps[j].SnapshotAt })
	idxEnd, idxStart := -1, -1
	for i := len(snaps) - 1; i >= 0; i-- {
		if snaps[i].SnapshotAt <= endSlotMs && idxEnd < 0 {
			idxEnd = i
		}
		if snaps[i].SnapshotAt <= startSlotMs && idxStart < 0 {
			idxStart = i
			break
		}
	}
	if idxEnd < 0 || idxStart < 0 {
		return 0, 0, false
	}
	spend = snaps[idxEnd].Spend - snaps[idxStart].Spend
	if spend < 0 {
		spend = 0
	}
	imp := snaps[idxEnd].Impressions - snaps[idxStart].Impressions
	if imp < 0 {
		imp = 0
	}
	impressions = float64(imp)
	return spend, impressions, true
}

// GetSpend1hFromSnapshots suy ra spend 1h gần nhất từ snapshots. Dùng cho ROAS_1h.
func GetSpend1hFromSnapshots(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (float64, bool) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endSlot := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
	endSlotMs := endSlot.UnixMilli()
	today := now.Format("2006-01-02")
	startSlotMs := endSlotMs - 60*60*1000

	coll, okColl := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsightsDailySnapshots)
	if !okColl {
		return 0, false
	}
	filter := bson.M{
		"objectType":          "ad_account",
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"date":                today,
		"snapshotAt":          bson.M{"$lte": endSlotMs},
	}
	cursor, err := coll.Find(ctx, filter, nil)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)

	var snaps []struct {
		SnapshotAt int64   `bson:"snapshotAt"`
		Spend      float64 `bson:"spend"`
	}
	for cursor.Next(ctx) {
		var r struct {
			SnapshotAt int64   `bson:"snapshotAt"`
			Spend      float64 `bson:"spend"`
		}
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		snaps = append(snaps, r)
	}
	if len(snaps) == 0 {
		return 0, false
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].SnapshotAt < snaps[j].SnapshotAt })

	idxEnd, idxStart := -1, -1
	for i := len(snaps) - 1; i >= 0; i-- {
		if snaps[i].SnapshotAt <= endSlotMs && idxEnd < 0 {
			idxEnd = i
		}
		if snaps[i].SnapshotAt <= startSlotMs && idxStart < 0 {
			idxStart = i
			break
		}
	}
	if idxEnd < 0 || idxStart < 0 {
		return 0, false
	}
	spend := snaps[idxEnd].Spend - snaps[idxStart].Spend
	if spend < 0 {
		spend = 0
	}
	return spend, true
}

// GetFBPurchasesForCampaign lấy tổng FB Purchases (từ metaData.actions) cho campaign trong 2 ngày gần nhất.
// Dùng cho Dual-source confirm (FolkForm v4.1 PATCH 05): Pancake=0 VÀ FB>0 → attribution gap, chờ 1 checkpoint.
func GetFBPurchasesForCampaign(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID) (int64, bool) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return 0, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	dateEnd := now.Format("2006-01-02")
	dateStart := now.AddDate(0, 0, -1).Format("2006-01-02") // 2 ngày: hôm qua + hôm nay

	extractPurchases := bson.M{
		"$reduce": bson.M{
			"input":    bson.M{"$ifNull": bson.A{"$metaData.actions", bson.A{}}},
			"initialValue": int64(0),
			"in": bson.M{
				"$add": bson.A{
					"$$value",
					bson.M{
						"$cond": bson.M{
							"if": bson.M{
								"$or": bson.A{
									bson.M{"$regexMatch": bson.M{"input": bson.M{"$toLower": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$$this.action_type", ""}}, ""}}}, "regex": "purchase"}},
									bson.M{"$regexMatch": bson.M{"input": bson.M{"$toLower": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$$this.action_type", ""}}, ""}}}, "regex": "omni_purchase"}},
								},
							},
							"then": bson.M{"$convert": bson.M{"input": "$$this.value", "to": "long", "onError": 0, "onNull": 0}},
							"else": int64(0),
						},
					},
				},
			},
		},
	}
	pipeline := []bson.M{
		{"$match": bson.M{
			"objectType":          "campaign",
			"objectId":            campaignId,
			"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
			"ownerOrganizationId": ownerOrgID,
			"dateStart":           bson.M{"$gte": dateStart, "$lte": dateEnd},
		}},
		{"$addFields": bson.M{"_purchases": extractPurchases}},
		{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$_purchases"}}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)
	var doc struct {
		Total int64 `bson:"total"`
	}
	if !cursor.Next(ctx) {
		return 0, false
	}
	if err := cursor.Decode(&doc); err != nil {
		return 0, false
	}
	return doc.Total, true
}

// GetCPM3dayAvgFromInsights lấy CPM trung bình 3 ngày từ meta_ad_insights (ad_account, daily).
// Dùng cho CB-2: so sánh CPM_30p vs CPM_3day_avg.
func GetCPM3dayAvgFromInsights(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID) (float64, bool) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return 0, false
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	dateEnd := now.Format("2006-01-02")
	dateStart := now.AddDate(0, 0, -2).Format("2006-01-02")

	filter := bson.M{
		"objectType":          "ad_account",
		"adAccountId":         adAccountIdFilterForSnapshots(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"dateStart":           bson.M{"$gte": dateStart, "$lte": dateEnd},
	}
	cursor, err := coll.Find(ctx, filter, nil)
	if err != nil {
		return 0, false
	}
	defer cursor.Close(ctx)

	var totalCpm float64
	var count int
	for cursor.Next(ctx) {
		var doc struct {
			Cpm string `bson:"cpm"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		cpm := parseFloat(doc.Cpm)
		if cpm > 0 {
			totalCpm += cpm
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return totalCpm / float64(count), true
}
