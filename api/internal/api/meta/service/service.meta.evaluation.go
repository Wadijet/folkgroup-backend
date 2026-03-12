// Package metasvc - RecalculateForEntity: tính lại currentMetrics cho 1 entity (ad, adset, campaign, ad_account).
// Bottom-up: Ad tính từ insight+order+conversation; AdSet/Campaign/Account aggregate từ con.
// Hook: phát sinh từ nguồn nào thì chỉ update raw nguồn đó, rồi tính lại layer1→layer2→layer3.
package metasvc

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	adsadaptive "meta_commerce/internal/api/ads/adaptive"
	adsconfig "meta_commerce/internal/api/ads/config"
	"meta_commerce/internal/api/meta/models"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mongoopts "go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// DefaultWindowDays số ngày mặc định cho window (raw.window).
	DefaultWindowDays = 7
	// MetaLearningPhaseDays số ngày learning phase của Meta (ad mới tạo < N ngày = NEW).
	MetaLearningPhaseDays = 7
)

// adAccountIdFilterForMeta trả về giá trị filter cho field adAccountId.
// meta_ads/meta_adsets/meta_campaigns có thể lưu "act_XXX" hoặc "XXX" (chỉ số) — cần match cả hai.
func adAccountIdFilterForMeta(adAccountId string) interface{} {
	if adAccountId == "" {
		return adAccountId
	}
	if strings.HasPrefix(adAccountId, "act_") {
		return bson.M{"$in": bson.A{adAccountId, strings.TrimPrefix(adAccountId, "act_")}}
	}
	return bson.M{"$in": bson.A{adAccountId, "act_" + adAccountId}}
}

// RecalculateForEntity tính lại currentMetrics cho 1 entity. Bottom-up: nếu là parent thì gọi con trước.
// objectType: ad_account | campaign | adset | ad
func RecalculateForEntity(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID) error {
	switch objectType {
	case "ad":
		return updateRawAndLayersForAd(ctx, objectId, adAccountId, ownerOrgID)
	case "adset", "campaign", "ad_account":
		return rollupFromChildren(ctx, objectType, objectId, adAccountId, ownerOrgID)
	default:
		return nil
	}
}

// RecalculateAllResult kết quả recalculate toàn bộ Meta ads.
type RecalculateAllResult struct {
	TotalAdsProcessed   int      `json:"totalAdsProcessed"`   // Số Ad đã tính toán lại thành công
	TotalAdsFailed      int      `json:"totalAdsFailed"`      // Số Ad lỗi khi tính toán
	FailedAdIds         []string `json:"failedAdIds,omitempty"` // Danh sách adId lỗi (tối đa 10 mẫu)
	TotalAdSetsRolledUp int      `json:"totalAdSetsRolledUp"` // Số AdSet đã roll-up
	TotalCampaignsRolledUp int   `json:"totalCampaignsRolledUp"` // Số Campaign đã roll-up
	TotalAccountsRolledUp int    `json:"totalAccountsRolledUp"`  // Số AdAccount đã roll-up
}

const maxFailedAdIds = 10

// RecalculateAllMetaAds tính toán lại toàn bộ currentMetrics cho Meta ads của org.
//
// Luồng (bottom-up):
// 1. Lấy tất cả meta_ads theo ownerOrganizationId (có limit nếu > 0)
// 2. Phase 1 - Ads: Với mỗi Ad, gọi RecalculateForEntity("ad", ...) — tính raw từ meta insights + pos orders + conversations → layer1→layer2→layer3
// 3. Phase 2 - AdSets: Thu thập unique (adSetId, adAccountId), gọi rollupFromChildren cho từng AdSet
// 4. Phase 3 - Campaigns: Thu thập unique (campaignId, adAccountId), gọi rollupFromChildren cho từng Campaign
// 5. Phase 4 - AdAccounts: Thu thập unique adAccountId, gọi rollupFromChildren cho từng AdAccount
//
// Tham số:
// - ctx: context
// - ownerOrgID: ID tổ chức sở hữu
// - limit: giới hạn số Ad xử lý (0 = tất cả)
//
// Trả về:
// - *RecalculateAllResult: kết quả (số processed, failed, roll-up)
// - error: lỗi nếu có (ví dụ không tìm thấy collection)
func RecalculateAllMetaAds(ctx context.Context, ownerOrgID primitive.ObjectID, limit int) (*RecalculateAllResult, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ads")
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	opts := mongoopts.Find().SetProjection(bson.M{"adId": 1, "adSetId": 1, "campaignId": 1, "adAccountId": 1})
	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	type adRef struct {
		AdId        string `bson:"adId"`
		AdSetId     string `bson:"adSetId"`
		CampaignId  string `bson:"campaignId"`
		AdAccountId string `bson:"adAccountId"`
	}
	var ads []adRef
	for cursor.Next(ctx) {
		var doc adRef
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.AdId == "" || doc.AdAccountId == "" {
			continue
		}
		ads = append(ads, doc)
	}

	result := &RecalculateAllResult{}
	adSetMap := make(map[string]string)     // adSetId -> adAccountId
	campaignMap := make(map[string]string)   // campaignId -> adAccountId
	accountSet := make(map[string]struct{}) // adAccountId

	for _, ad := range ads {
		if err := RecalculateForEntity(ctx, "ad", ad.AdId, ad.AdAccountId, ownerOrgID); err != nil {
			result.TotalAdsFailed++
			if len(result.FailedAdIds) < maxFailedAdIds {
				result.FailedAdIds = append(result.FailedAdIds, ad.AdId)
			}
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"adId": ad.AdId, "ownerOrgId": ownerOrgID.Hex(),
			}).Warn("[META] Recalculate Ad thất bại")
			continue
		}
		result.TotalAdsProcessed++
		if ad.AdSetId != "" {
			adSetMap[ad.AdSetId] = ad.AdAccountId
		}
		if ad.CampaignId != "" {
			campaignMap[ad.CampaignId] = ad.AdAccountId
		}
		if ad.AdAccountId != "" {
			accountSet[ad.AdAccountId] = struct{}{}
		}
	}

	// Phase 2: Roll-up AdSets
	for adSetId, adAccountId := range adSetMap {
		if err := RecalculateForEntity(ctx, "adset", adSetId, adAccountId, ownerOrgID); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"adSetId": adSetId, "ownerOrgId": ownerOrgID.Hex(),
			}).Warn("[META] Roll-up AdSet thất bại")
			continue
		}
		result.TotalAdSetsRolledUp++
	}

	// Phase 3: Roll-up Campaigns
	for campaignId, adAccountId := range campaignMap {
		if err := RecalculateForEntity(ctx, "campaign", campaignId, adAccountId, ownerOrgID); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"campaignId": campaignId, "ownerOrgId": ownerOrgID.Hex(),
			}).Warn("[META] Roll-up Campaign thất bại")
			continue
		}
		result.TotalCampaignsRolledUp++
	}

	// Phase 4: Roll-up AdAccounts
	for adAccountId := range accountSet {
		if err := RecalculateForEntity(ctx, "ad_account", adAccountId, adAccountId, ownerOrgID); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"adAccountId": adAccountId, "ownerOrgId": ownerOrgID.Hex(),
			}).Warn("[META] Roll-up AdAccount thất bại")
			continue
		}
		result.TotalAccountsRolledUp++
	}

	return result, nil
}

// UpdateRawFromSource chỉ cập nhật raw từ 1 nguồn (meta|pancake.pos|pancake.conversation), giữ nguyên raw khác, rồi tính lại layers.
func UpdateRawFromSource(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, source string) error {
	if objectType != "ad" {
		// Phase 1: Chỉ Ad có raw từ nhiều nguồn
		return nil
	}
	dateStart, dateStop := getWindowDates(DefaultWindowDays)
	windowMs := getWindowMsForCurrentMetrics(ctx)
	start7dMs, end7dMs := getWindowMsRangeFromDates(DefaultWindowDays)

	// Lấy currentMetrics hiện tại
	current, err := getAdCurrentMetrics(ctx, objectId, adAccountId, ownerOrgID)
	if err != nil {
		return err
	}
	raw, _ := current["raw"].(map[string]interface{})
	if raw == nil {
		raw = make(map[string]interface{})
	}
	r7 := getRaw7d(raw)

	// Chỉ cập nhật raw.7d từ nguồn được chỉ định (meta, pancake.pos, pancake.conversation)
	switch source {
	case "meta":
		metaRaw, err := fetchRawMetaFromInsights(ctx, objectType, objectId, adAccountId, ownerOrgID, dateStart, dateStop)
		if err != nil {
			return fmt.Errorf("fetch raw meta: %w", err)
		}
		r7["meta"] = metaRaw
	case "pancake.pos":
		posRaw, err := fetchRawPosFromOrders(ctx, objectId, ownerOrgID, windowMs, start7dMs, end7dMs)
		if err != nil {
			return fmt.Errorf("fetch raw pos: %w", err)
		}
		if r7["pancake"] == nil {
			r7["pancake"] = make(map[string]interface{})
		}
		pancake, _ := r7["pancake"].(map[string]interface{})
		if pancake == nil {
			pancake = make(map[string]interface{})
		}
		pancake["pos"] = posRaw
		r7["pancake"] = pancake
	case "pancake.conversation":
		convRaw, err := fetchRawConversationFromConversations(ctx, objectId, ownerOrgID, windowMs, start7dMs, end7dMs)
		if err != nil {
			return fmt.Errorf("fetch raw conversation: %w", err)
		}
		if r7["pancake"] == nil {
			r7["pancake"] = make(map[string]interface{})
		}
		pancake, _ := r7["pancake"].(map[string]interface{})
		if pancake == nil {
			pancake = make(map[string]interface{})
		}
		pancake["conversation"] = convRaw
		r7["pancake"] = pancake
	default:
		return fmt.Errorf("nguồn không hợp lệ: %s", source)
	}

	r7["window"] = map[string]interface{}{
		"dateStart": dateStart,
		"dateStop":  dateStop,
	}
	r7["metaCreatedAt"] = fetchAdMetaCreatedAt(ctx, objectId, adAccountId, ownerOrgID)
	// Đảm bảo raw có cấu trúc nested khi đã có 2h/1h
	if raw["7d"] != nil {
		raw["7d"] = r7
	} else {
		raw = r7
	}
	current["raw"] = raw

	// Tính layer1, layer2, layer3. Theo FolkForm v4.1: 13 rules CHỈ apply cho campaign — Ad không có alertFlags.
	layer1 := computeLayer1(raw)
	layer2 := computeLayer2(raw, layer1)
	layer3 := computeLayer3(layer1, layer2)
	current["layer1"] = layer1
	current["layer2"] = layer2
	current["layer3"] = layer3
	current["alertFlags"] = []string{}
	current["actions"] = []map[string]interface{}{}

	return updateAdCurrentMetrics(ctx, objectId, adAccountId, ownerOrgID, current, source)
}

// updateRawAndLayersForAd tính đầy đủ raw từ 3 nguồn (7d, 2h, 1h) rồi tính layers cho Ad.
// Cấu trúc raw: { "7d": { meta, pancake, window, metaCreatedAt }, "2h": { orders, revenue, mess }, "1h": { orders, revenue, mess } }
func updateRawAndLayersForAd(ctx context.Context, adId, adAccountId string, ownerOrgID primitive.ObjectID) error {
	dateStart, dateStop := getWindowDates(DefaultWindowDays)
	window7dMs := getWindowMsForCurrentMetrics(ctx)
	window2hMs := int64(2 * 60 * 60 * 1000)
	window1hMs := int64(60 * 60 * 1000)

	// raw.7d — Theo FolkForm 01: Pancake (orders) + FB (mess). Source: meta_ad_insights (FB) + pc_pos_orders (Pancake)
	// Dùng calendar range (startMs, endMs) để align với meta — cùng khoảng thời gian theo múi giờ.
	start7dMs, end7dMs := getWindowMsRangeFromDates(DefaultWindowDays)
	raw7d := make(map[string]interface{})
	metaRaw, err := fetchRawMetaFromInsights(ctx, "ad", adId, adAccountId, ownerOrgID, dateStart, dateStop)
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("[ADS_PROFILE] Không lấy được raw meta")
	} else {
		raw7d["meta"] = metaRaw
	}
	posRaw7d, err := fetchRawPosFromOrders(ctx, adId, ownerOrgID, window7dMs, start7dMs, end7dMs)
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("[ADS_PROFILE] Không lấy được raw pos 7d")
	} else {
		raw7d["pancake"] = map[string]interface{}{"pos": posRaw7d}
	}
	convRaw7d, err := fetchRawConversationFromConversations(ctx, adId, ownerOrgID, window7dMs, start7dMs, end7dMs)
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("[ADS_PROFILE] Không lấy được raw conversation 7d")
	} else {
		if raw7d["pancake"] == nil {
			raw7d["pancake"] = make(map[string]interface{})
		}
		pancake, _ := raw7d["pancake"].(map[string]interface{})
		if pancake == nil {
			pancake = make(map[string]interface{})
		}
		pancake["conversation"] = convRaw7d
		raw7d["pancake"] = pancake
	}
	raw7d["window"] = map[string]interface{}{"dateStart": dateStart, "dateStop": dateStop}
	raw7d["metaCreatedAt"] = fetchAdMetaCreatedAt(ctx, adId, adAccountId, ownerOrgID)

	// raw.2h — Theo FolkForm 04: Conv_Rate_now = Pancake_orders_2h / FB_Mess_2h. Source: pc_pos_orders (Pancake) + fb_conversations (FB mess)
	// Dùng khoảng align theo boundary 2h (slot đã hoàn thành gần nhất).
	start2hMs, end2hMs := getWindowMsRangeForShortCycle(120)
	raw2h := fetchRawForShortWindow(ctx, adId, ownerOrgID, window2hMs, start2hMs, end2hMs)

	// raw.1h — Theo FolkForm PATCH 03 HB-3: FB_Mess_1h, Pancake_orders_1h. Source: pc_pos_orders (Pancake) + fb_conversations (FB mess)
	start1hMs, end1hMs := getWindowMsRangeForShortCycle(60)
	raw1h := fetchRawForShortWindow(ctx, adId, ownerOrgID, window1hMs, start1hMs, end1hMs)

	// raw.30p — Theo FolkForm 01: Mess_30p cho MQS, Msg_Rate. meta_ad_insights chỉ daily → mess từ fb_conversations.
	window30pMs := int64(30 * 60 * 1000)
	start30pMs, end30pMs := getWindowMsRangeForShortCycle(30)
	raw30p := fetchRawForShortWindow(ctx, adId, ownerOrgID, window30pMs, start30pMs, end30pMs)

	raw := map[string]interface{}{
		"7d":  raw7d,
		"2h":  raw2h,
		"1h":  raw1h,
		"30p": raw30p,
	}

	layer1 := computeLayer1(raw)
	layer2 := computeLayer2(raw, layer1)
	layer3 := computeLayer3(layer1, layer2)
	// Theo FolkForm v4.1: 13 rules CHỈ apply cho campaign — Ad không có alertFlags.
	current := map[string]interface{}{
		"raw":         raw,
		"layer1":     layer1,
		"layer2":     layer2,
		"layer3":     layer3,
		"alertFlags": []string{},
		"actions":    []map[string]interface{}{},
	}
	return updateAdCurrentMetrics(ctx, adId, adAccountId, ownerOrgID, current, "recompute")
}

func getAdCurrentMetrics(ctx context.Context, adId, adAccountId string, ownerOrgID primitive.ObjectID) (map[string]interface{}, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ads")
	}
	filter := bson.M{
		"adId":                 adId,
		"adAccountId":          adAccountIdFilterForMeta(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}
	var doc struct {
		CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
	}
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return make(map[string]interface{}), nil
		}
		return nil, err
	}
	if doc.CurrentMetrics == nil {
		return make(map[string]interface{}), nil
	}
	return doc.CurrentMetrics, nil
}

func updateAdCurrentMetrics(ctx context.Context, adId, adAccountId string, ownerOrgID primitive.ObjectID, current map[string]interface{}, trigger string) error {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return fmt.Errorf("không tìm thấy collection meta_ads")
	}
	filter := bson.M{
		"adId":                 adId,
		"adAccountId":          adAccountIdFilterForMeta(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}
	old, _ := getAdCurrentMetrics(ctx, adId, adAccountId, ownerOrgID)
	if trigger == "" {
		trigger = "manual"
	}
	recordActivityIfChanged(ctx, adId, adAccountId, ownerOrgID, old, current, trigger)
	_, err := coll.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"currentMetrics": current, "updatedAt": time.Now().UnixMilli()}})
	return err
}

// recordActivityIfChanged so sánh old vs current, nếu khác thì ghi AdsActivityHistory với snapshotChanges (cho Ad).
func recordActivityIfChanged(ctx context.Context, adId, adAccountId string, ownerOrgID primitive.ObjectID, old, current map[string]interface{}, trigger string) {
	recordActivityForEntity(ctx, "ad", adId, adAccountId, ownerOrgID, old, current, trigger)
}

// recordActivityForEntity ghi ads_activity_history khi currentMetrics thay đổi (Ad, AdSet, Campaign, Account).
func recordActivityForEntity(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, old, current map[string]interface{}, trigger string) {
	changes := diffMapsFlatten(old, current)
	if len(changes) == 0 {
		return
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.AdsActivityHistory)
	if !ok {
		logger.GetAppLogger().WithError(fmt.Errorf("không tìm thấy collection ads_activity_history")).Warn("[META] Không ghi activity history")
		return
	}
	now := time.Now().UnixMilli()
	doc := models.AdsActivityHistory{
		ActivityType:        models.AdsActivityTypeMetricsChanged,
		AdAccountId:         adAccountId,
		ObjectType:          objectType,
		ObjectId:            objectId,
		OwnerOrganizationID: ownerOrgID,
		ActivityAt:          now,
		Metadata: map[string]interface{}{
			"metricsSnapshot": current,
			"snapshotChanges": changes,
			"trigger":         trigger,
		},
		CreatedAt: now,
	}
	if _, err := coll.InsertOne(ctx, doc); err != nil {
		logger.GetAppLogger().WithError(err).Warn("[META] Ghi activity history thất bại")
	}
}

// diffMapsFlatten so sánh hai map, trả về []{field, oldValue, newValue} cho các key khác nhau.
func diffMapsFlatten(old, new map[string]interface{}) []map[string]interface{} {
	flatOld := flattenMap(old, "")
	flatNew := flattenMap(new, "")
	var out []map[string]interface{}
	seen := make(map[string]bool)
	for k, vNew := range flatNew {
		seen[k] = true
		vOld := flatOld[k]
		if !reflect.DeepEqual(vOld, vNew) {
			out = append(out, map[string]interface{}{"field": k, "oldValue": vOld, "newValue": vNew})
		}
	}
	for k, vOld := range flatOld {
		if seen[k] {
			continue
		}
		out = append(out, map[string]interface{}{"field": k, "oldValue": vOld, "newValue": nil})
	}
	return out
}

func flattenMap(m map[string]interface{}, prefix string) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if v == nil {
			out[key] = nil
			continue
		}
		switch val := v.(type) {
		case map[string]interface{}:
			for kk, vv := range flattenMap(val, key) {
				out[kk] = vv
			}
		default:
			out[key] = v
		}
	}
	return out
}

// getRaw7d trả về raw cho window 7d. Hỗ trợ cả cấu trúc mới (raw.7d) và cũ (raw phẳng).
func getRaw7d(raw map[string]interface{}) map[string]interface{} {
	if r, ok := raw["7d"].(map[string]interface{}); ok && r != nil {
		return r
	}
	return raw
}

// getRaw2h trả về raw cho window 2h. Nil nếu không có.
func getRaw2h(raw map[string]interface{}) map[string]interface{} {
	r, _ := raw["2h"].(map[string]interface{})
	return r
}

// getRaw1h trả về raw cho window 1h. Nil nếu không có.
func getRaw1h(raw map[string]interface{}) map[string]interface{} {
	r, _ := raw["1h"].(map[string]interface{})
	return r
}

// getRaw30p trả về raw cho window 30 phút. Nil nếu không có.
// Mess từ fb_conversations (meta_ad_insights chỉ có daily).
func getRaw30p(raw map[string]interface{}) map[string]interface{} {
	r, _ := raw["30p"].(map[string]interface{})
	return r
}

// getTimeFactorForMQS trả về Time_Factor theo FolkForm 01. Dùng timezone Asia/Ho_Chi_Minh.
func getTimeFactorForMQS() float64 {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.FixedZone("UTC+7", 7*3600)
	}
	hour := time.Now().In(loc).Hour()
	min := time.Now().In(loc).Minute()
	// 07:00–11:59 ×1.2 | 12:00–16:59 ×1.0 | 17:00–19:59 ×0.8 | 20:00–22:29 ×0.5 | khác ×1.0
	if hour >= 7 && hour < 12 {
		return 1.2
	}
	if hour >= 12 && hour < 17 {
		return 1.0
	}
	if hour >= 17 && hour < 20 {
		return 0.8
	}
	if hour == 20 || hour == 21 || (hour == 22 && min < 30) {
		return 0.5
	}
	return 1.0
}

// computeLayer1 tính layer1 theo FolkForm 01. Nguồn: Pancake (orders) + FB (mess).
// raw.7d: meta (meta_ad_insights) + pancake.pos (pc_pos_orders). raw.2h/1h/30p: orders (pc_pos_orders) + mess (fb_conversations).
func computeLayer1(raw map[string]interface{}) map[string]interface{} {
	r7d := getRaw7d(raw)
	r2h := getRaw2h(raw)
	r1h := getRaw1h(raw)
	r30p := getRaw30p(raw)

	meta, _ := r7d["meta"].(map[string]interface{})
	pancake, _ := r7d["pancake"].(map[string]interface{})
	pos, _ := mapOrNil(pancake, "pos").(map[string]interface{})

	// FB_Mess_7d, Pancake_orders_7d từ meta_ad_insights + pc_pos_orders
	spend := toFloat(meta, "spend")
	mess := toInt64(meta, "mess")
	inlineLinkClicks := toInt64(meta, "inlineLinkClicks")
	orders := toInt64(pos, "orders")
	revenue := toFloat(pos, "revenue")

	msgRate := 0.0
	if inlineLinkClicks > 0 && mess > 0 {
		msgRate = float64(mess) / float64(inlineLinkClicks)
	}
	cpaMess := 0.0
	if mess > 0 {
		cpaMess = spend / float64(mess)
	}
	cpaPurchase := 0.0
	if orders > 0 {
		cpaPurchase = spend / float64(orders)
	}
	convRate := 0.0
	if mess > 0 {
		convRate = float64(orders) / float64(mess)
	}
	// convRate_2h, convRate_1h — từ raw.2h, raw.1h (orders/mess từ fb_conversations). Tỷ lệ 0-1.
	convRate2h := 0.0
	if r2h != nil {
		o2 := toInt64(r2h, "orders")
		m2 := toInt64(r2h, "mess")
		if m2 > 0 {
			convRate2h = float64(o2) / float64(m2)
		}
	}
	convRate1h := 0.0
	if r1h != nil {
		o1 := toInt64(r1h, "orders")
		m1 := toInt64(r1h, "mess")
		if m1 > 0 {
			convRate1h = float64(o1) / float64(m1)
		}
	}
	roas := 0.0
	if spend > 0 {
		roas = revenue / spend
	}

	// Lifecycle: ưu tiên metaCreatedAt (thời gian tạo gốc từ Meta) — ad trong learning phase (< 7 ngày) = NEW
	lifecycle := "NEW"
	metaCreatedAt := toInt64(r7d, "metaCreatedAt")
	if metaCreatedAt > 0 {
		daysSinceCreated := (time.Now().UnixMilli() - metaCreatedAt) / (24 * 60 * 60 * 1000)
		if daysSinceCreated < MetaLearningPhaseDays {
			lifecycle = "NEW" // Learning phase, bỏ qua clicks
		} else {
			// Ra khỏi learning phase: dùng click-based
			if inlineLinkClicks >= 2000 {
				lifecycle = "MATURE"
			} else if inlineLinkClicks >= 500 {
				lifecycle = "CALIBRATED"
			} else if inlineLinkClicks >= 100 {
				lifecycle = "WARMING"
			}
		}
	} else {
		// Fallback khi không có metaCreatedAt (dữ liệu cũ): dùng click-based
		if inlineLinkClicks >= 100 {
			lifecycle = "WARMING"
		}
		if inlineLinkClicks >= 500 {
			lifecycle = "CALIBRATED"
		}
		if inlineLinkClicks >= 2000 {
			lifecycle = "MATURE"
		}
	}

	// MQS = Mess(lag-30p) × CR_7day_Pancake × Time_Factor. Theo FolkForm 01.
	// Ưu tiên mess_30p từ fb_conversations; fallback mess_7d khi không có raw.30p.
	timeFactor := getTimeFactorForMQS()
	messForMQS := mess
	if r30p != nil {
		if m30 := toInt64(r30p, "mess"); m30 > 0 {
			messForMQS = m30
		}
	}
	mqs := float64(messForMQS) * convRate * timeFactor

	// spend_pct = spend / daily_budget (từ meta.dailyBudget khi có — campaign level)
	spendPct := 0.0
	if dailyBudget := toFloat(meta, "dailyBudget"); dailyBudget > 0 && spend > 0 {
		spendPct = spend / dailyBudget
	}

	// runtime_minutes = (now - metaCreatedAt) / 60000 — cho KO-A, BASE CONDITION
	runtimeMinutes := 0.0
	if metaCreatedAt := toInt64(r7d, "metaCreatedAt"); metaCreatedAt > 0 {
		runtimeMinutes = float64(time.Now().UnixMilli()-metaCreatedAt) / 60000
	}

	// msgRate_30p = mess_30p / clicks_30p. Meta không có clicks 30p → ước lượng: clicks_30p ≈ clicks_7d/336 (7d=10080p, 30p/10080p=1/336).
	msgRate30p := 0.0
	if r30p != nil && inlineLinkClicks > 0 {
		m30 := toInt64(r30p, "mess")
		clicks30pEst := float64(inlineLinkClicks) / 336.0 // 7 ngày = 10080 phút; 30p/10080p ≈ 1/336
		if clicks30pEst >= 1 && m30 > 0 {
			msgRate30p = float64(m30) / clicks30pEst
		}
	}

	// Tên metric thể hiện rõ chu kỳ theo FolkForm v4.1
	return map[string]interface{}{
		"lifecycle":        lifecycle,
		"msgRate_7d":       msgRate,
		"msgRate_30p":      msgRate30p,
		"mess_30p":         toInt64(r30p, "mess"),
		"cpaMess_7d":       cpaMess,
		"cpaPurchase_7d":   cpaPurchase,
		"convRate_7d":      convRate,
		"convRate_2h":      convRate2h,
		"convRate_1h":      convRate1h,
		"roas_7d":          roas,
		"mqs_7d":           mqs,
		"spendPct_7d":      spendPct,
		"runtimeMinutes":  runtimeMinutes,
	}
}

func computeLayer2(raw map[string]interface{}, layer1 map[string]interface{}) map[string]interface{} {
	r7d := getRaw7d(raw)
	meta, _ := r7d["meta"].(map[string]interface{})
	cpm := toFloat(meta, "cpm")
	ctr := toFloat(meta, "ctr")
	frequency := toFloat(meta, "frequency")

	// Đơn giản hóa: 5 trục 0-100 dựa trên ngưỡng. Dùng metrics 7d.
	efficiency := scoreFromRoas(toFloat(layer1, "roas_7d"))
	demandQuality := scoreFromRate(toFloat(layer1, "msgRate_7d"), toFloat(layer1, "convRate_7d"))
	auctionPressure := scoreFromCpmCtr(cpm, ctr)
	saturation := scoreFromFrequency(frequency)
	momentum := 50 // TODO: cần trend data

	return map[string]interface{}{
		"efficiency":      efficiency,
		"demandQuality":   demandQuality,
		"auctionPressure": auctionPressure,
		"saturation":     saturation,
		"momentum":       momentum,
	}
}

func computeLayer3(layer1, layer2 map[string]interface{}) map[string]interface{} {
	eff := toFloat(layer2, "efficiency")
	demand := toFloat(layer2, "demandQuality")
	auction := toFloat(layer2, "auctionPressure")
	sat := toFloat(layer2, "saturation")
	mom := toFloat(layer2, "momentum")
	chs := (eff + demand + auction + sat + mom) / 5

	healthState := "critical"
	if chs >= 80 {
		healthState = "strong"
	} else if chs >= 60 {
		healthState = "healthy"
	} else if chs >= 40 {
		healthState = "warning"
	}

	roas := toFloat(layer1, "roas_7d")
	performanceTier := "low"
	if roas >= 3 {
		performanceTier = "high"
	} else if roas >= 1.5 {
		performanceTier = "medium"
	}

	lifecycle, _ := layer1["lifecycle"].(string)
	portfolioCell := derivePortfolioCell(lifecycle, performanceTier)

	return map[string]interface{}{
		"chs":             chs,
		"healthState":     healthState,
		"performanceTier": performanceTier,
		"stage":           "stable",
		"portfolioCell":   portfolioCell,
		"diagnoses":       []string{},
	}
}

func derivePortfolioCell(lifecycle, performanceTier string) string {
	switch lifecycle {
	case "NEW":
		return "test"
	case "WARMING":
		if performanceTier == "high" {
			return "potential"
		}
		return "test"
	case "CALIBRATED":
		if performanceTier == "high" {
			return "scale"
		}
		if performanceTier == "medium" {
			return "maintain"
		}
		return "fix"
	case "MATURE":
		if performanceTier == "high" {
			return "scale"
		}
		if performanceTier == "medium" {
			return "maintain"
		}
		return "recover"
	}
	return "test"
}

func scoreFromRoas(roas float64) float64 {
	if roas >= 3 {
		return 100
	}
	if roas >= 2 {
		return 80
	}
	if roas >= 1 {
		return 60
	}
	if roas >= 0.5 {
		return 40
	}
	return 20
}

func scoreFromRate(msgRate, convRate float64) float64 {
	s := (msgRate*50 + convRate*50) / 2
	if s > 100 {
		return 100
	}
	return s
}

func scoreFromCpmCtr(cpm, ctr float64) float64 {
	// CPM thấp + CTR cao = tốt
	if cpm < 50000 && ctr > 1 {
		return 80
	}
	if cpm < 100000 && ctr > 0.5 {
		return 60
	}
	return 40
}

func scoreFromFrequency(f float64) float64 {
	if f <= 2 {
		return 80
	}
	if f <= 4 {
		return 60
	}
	return 40
}

func mapOrNil(m map[string]interface{}, k string) interface{} {
	if m == nil {
		return nil
	}
	return m[k]
}

func toFloat(m map[string]interface{}, k string) float64 {
	if m == nil {
		return 0
	}
	v := m[k]
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func toInt64(m map[string]interface{}, k string) int64 {
	if m == nil {
		return 0
	}
	v := m[k]
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}

// fetchAdMetaCreatedAt lấy metaCreatedAt (thời gian tạo gốc từ Meta API) từ meta_ads.
// Trả về 0 nếu không tìm thấy hoặc chưa có field.
func fetchAdMetaCreatedAt(ctx context.Context, adId, adAccountId string, ownerOrgID primitive.ObjectID) int64 {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
	if !ok {
		return 0
	}
	filter := bson.M{
		"adId":                 adId,
		"adAccountId":          adAccountIdFilterForMeta(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}
	var doc struct {
		MetaCreatedAt int64 `bson:"metaCreatedAt"`
	}
	if err := coll.FindOne(ctx, filter, mongoopts.FindOne().SetProjection(bson.M{"metaCreatedAt": 1})).Decode(&doc); err != nil {
		return 0
	}
	return doc.MetaCreatedAt
}

// fetchRawMetaFromInsights aggregate meta_ad_insights cho entity trong window → raw.meta.
// Extract Frequency, Mess, InlineLinkClicks từ metaData (đã có trong DB từ sync).
func fetchRawMetaFromInsights(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, dateStart, dateStop string) (map[string]interface{}, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdInsights)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection meta_ad_insights")
	}
	// adAccountId có thể lưu "act_XXX" hoặc "XXX" — meta_ad_insights có thể dùng format khác meta_ads.
	filter := bson.M{
		"objectType":          objectType,
		"objectId":            objectId,
		"adAccountId":         adAccountIdFilterForMeta(adAccountId),
		"ownerOrganizationId": ownerOrgID,
		"dateStart":           bson.M{"$gte": dateStart, "$lte": dateStop},
	}
	// $addFields: extract từ metaData trước khi $group
	// - _extractedMess: sum value từ actions có action_type chứa "messaging_conversation_started"
	// - _extractedInlineClicks: metaData.inline_link_clicks
	// - _extractedFrequency: metaData.frequency
	extractMess := bson.M{
		"$reduce": bson.M{
			"input":    bson.M{"$ifNull": bson.A{"$metaData.actions", bson.A{}}},
			"initialValue": int64(0),
			"in": bson.M{
				"$add": bson.A{
					"$$value",
					bson.M{
						"$cond": bson.M{
							"if": bson.M{
								"$regexMatch": bson.M{
									"input":   bson.M{"$toLower": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$$this.action_type", ""}}, ""}}},
									"regex":   "messaging_conversation_started",
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
	extractInlineClicks := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$metaData.inline_link_clicks", "0"}}, "to": "long", "onError": 0, "onNull": 0}}
	extractFrequency := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$metaData.frequency", "0"}}, "to": "double", "onError": 0, "onNull": 0}}
	extractCpm := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$metaData.cpm", "$cpm"}}, "0"}}, "to": "double", "onError": 0, "onNull": 0}}
	extractCtr := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$metaData.ctr", "$ctr"}}, "0"}}, "to": "double", "onError": 0, "onNull": 0}}
	extractCpc := bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{bson.M{"$ifNull": bson.A{"$metaData.cpc", "$cpc"}}, "0"}}, "to": "double", "onError": 0, "onNull": 0}}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$addFields", Value: bson.M{
			"_extractedMess":           extractMess,
			"_extractedInlineClicks":   extractInlineClicks,
			"_extractedFrequency":      extractFrequency,
			"_extractedCpm":            extractCpm,
			"_extractedCtr":            extractCtr,
			"_extractedCpc":            extractCpc,
		}}},
		{{Key: "$group", Value: bson.M{
			"_id":               nil,
			"spend":             bson.M{"$sum": bson.M{"$convert": bson.M{"input": "$spend", "to": "double", "onError": 0, "onNull": 0}}},
			"impressions":       bson.M{"$sum": bson.M{"$convert": bson.M{"input": "$impressions", "to": "long", "onError": 0, "onNull": 0}}},
			"clicks":            bson.M{"$sum": bson.M{"$convert": bson.M{"input": "$clicks", "to": "long", "onError": 0, "onNull": 0}}},
			"reach":             bson.M{"$sum": bson.M{"$convert": bson.M{"input": "$reach", "to": "long", "onError": 0, "onNull": 0}}},
			"mess":              bson.M{"$sum": "$_extractedMess"},
			"inlineLinkClicks":   bson.M{"$sum": "$_extractedInlineClicks"},
			"frequency":         bson.M{"$avg": "$_extractedFrequency"},
			"cpm":               bson.M{"$avg": "$_extractedCpm"},
			"ctr":               bson.M{"$avg": "$_extractedCtr"},
			"cpc":               bson.M{"$avg": "$_extractedCpc"},
		}}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	raw := map[string]interface{}{
		"spend":           0.0,
		"impressions":     int64(0),
		"clicks":          int64(0),
		"reach":           int64(0),
		"mess":            int64(0),
		"inlineLinkClicks": int64(0),
		"frequency":       0.0,
		"cpm":             0.0,
		"ctr":             0.0,
		"cpc":             0.0,
	}
	if cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		raw["spend"] = toFloatFromBson(doc, "spend")
		raw["impressions"] = toInt64FromBson(doc, "impressions")
		raw["clicks"] = toInt64FromBson(doc, "clicks")
		raw["reach"] = toInt64FromBson(doc, "reach")
		raw["mess"] = toInt64FromBson(doc, "mess")
		raw["inlineLinkClicks"] = toInt64FromBson(doc, "inlineLinkClicks")
		raw["frequency"] = toFloatFromBson(doc, "frequency")
		raw["cpm"] = toFloatFromBson(doc, "cpm")
		raw["ctr"] = toFloatFromBson(doc, "ctr")
		raw["cpc"] = toFloatFromBson(doc, "cpc")
	}
	return raw, nil
}

func toFloatFromBson(d bson.M, k string) float64 {
	v := d[k]
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int32:
		return float64(x)
	}
	return 0
}

func toInt64FromBson(d bson.M, k string) int64 {
	v := d[k]
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

// fetchRawPosFromOrders aggregate pc_pos_orders có posData.ad_id = adId trong window → raw.pancake.pos.
// Theo FolkForm v4.1: Conv_Rate_7day = Pancake_orders_7d / FB_Mess_7d — orders phải filter cùng chu kỳ với Meta insights.
// Khi startEndMs có 2 phần tử: dùng calendar range (align với meta). Ngược lại: dùng rolling window (now - windowMs đến now).
func fetchRawPosFromOrders(ctx context.Context, adId string, ownerOrgID primitive.ObjectID, windowMs int64, startEndMs ...int64) (map[string]interface{}, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection pc_pos_orders")
	}
	var startMs, endMs int64
	if len(startEndMs) >= 2 && startEndMs[0] > 0 && startEndMs[1] > 0 {
		startMs, endMs = startEndMs[0], startEndMs[1]
	} else {
		nowMs := time.Now().UnixMilli()
		startMs = nowMs - windowMs
		endMs = nowMs
	}
	startSec := startMs / 1000
	endSec := endMs / 1000

	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"posData.ad_id":       adId,
		"$or": []bson.M{
			{"posCreatedAt": bson.M{"$gte": startSec, "$lte": endSec}},
			{"insertedAt": bson.M{"$gte": startSec, "$lte": endSec}},
			{"posCreatedAt": bson.M{"$gte": startMs, "$lte": endMs}},
			{"insertedAt": bson.M{"$gte": startMs, "$lte": endMs}},
		},
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{
			"_id":    nil,
			"orders": bson.M{"$sum": 1},
			"revenue": bson.M{"$sum": bson.M{"$convert": bson.M{"input": bson.M{"$ifNull": bson.A{"$posData.total_price_after_sub_discount", 0}}, "to": "double", "onError": 0, "onNull": 0}}},
		}}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	raw := map[string]interface{}{"orders": int64(0), "revenue": 0.0}
	if cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		raw["orders"] = toInt64FromBson(doc, "orders")
		raw["revenue"] = toFloatFromBson(doc, "revenue")
	}
	return raw, nil
}

// fetchRawForShortWindow lấy raw cho window ngắn (2h, 1h, 30p) theo FolkForm 04, PATCH 03.
// Orders: pc_pos_orders (Pancake). Mess: fb_conversations (FB — hội thoại từ ad, meta_ad_insights chỉ có daily).
// Khi startEndMs có 2 phần tử: dùng khoảng align theo boundary (đúng chu kỳ). Ngược lại: rolling window.
func fetchRawForShortWindow(ctx context.Context, adId string, ownerOrgID primitive.ObjectID, windowMs int64, startEndMs ...int64) map[string]interface{} {
	out := map[string]interface{}{"orders": int64(0), "revenue": 0.0, "mess": int64(0)}
	posRaw, err := fetchRawPosFromOrders(ctx, adId, ownerOrgID, windowMs, startEndMs...)
	if err == nil {
		out["orders"] = toInt64(posRaw, "orders")
		out["revenue"] = toFloat(posRaw, "revenue")
	}
	convRaw, err := fetchRawConversationFromConversations(ctx, adId, ownerOrgID, windowMs, startEndMs...)
	if err == nil {
		out["mess"] = toInt64(convRaw, "conversationCount")
	}
	return out
}

// fetchRawConversationFromConversations đếm fb_conversations (FB mess) có panCakeData.ad_ids chứa adId trong window.
// FolkForm: FB_Mess = hội thoại từ ad. Dùng panCakeData.inserted_at (thời điểm hội thoại bắt đầu) — không dùng panCakeUpdatedAt.
// inserted_at có thể là string ISO ("2026-02-14T13:03:30") hoặc number (Unix sec/ms) — dùng aggregation để xử lý.
// Khi startEndMs có 2 phần tử: dùng calendar range (align với meta). Ngược lại: dùng rolling window.
func fetchRawConversationFromConversations(ctx context.Context, adId string, ownerOrgID primitive.ObjectID, windowMs int64, startEndMs ...int64) (map[string]interface{}, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection fb_conversations")
	}
	var startMs, endMs int64
	if len(startEndMs) >= 2 && startEndMs[0] > 0 && startEndMs[1] > 0 {
		startMs, endMs = startEndMs[0], startEndMs[1]
	} else {
		nowMs := time.Now().UnixMilli()
		startMs = nowMs - windowMs
		endMs = nowMs
	}

	// convTsMs: chuyển panCakeData.inserted_at sang Unix ms. Hỗ trợ string ISO và number (sec/ms).
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
				{"panCakeData.ad_ids": adId},
				{"panCakeData.ads.ad_id": adId},
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
		return nil, err
	}
	defer cursor.Close(ctx)

	count := int64(0)
	if cursor.Next(ctx) {
		var doc struct {
			N int64 `bson:"n"`
		}
		if err := cursor.Decode(&doc); err == nil {
			count = doc.N
		}
	}
	return map[string]interface{}{"conversationCount": count}, nil
}

// rollupFromChildren aggregate currentMetrics.raw từ children rồi tính layer1→layer2→layer3 cho parent.
// objectType: adset | campaign | ad_account
func rollupFromChildren(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID) error {
	childrenRaw, err := aggregateRawFromChildren(ctx, objectType, objectId, adAccountId, ownerOrgID)
	if err != nil {
		return err
	}
	if childrenRaw == nil {
		childrenRaw = make(map[string]interface{})
	}
	dateStart, dateStop := getWindowDates(DefaultWindowDays)
	// Đặt window vào raw.7d
	r7d := getRaw7d(childrenRaw)
	r7d["window"] = map[string]interface{}{"dateStart": dateStart, "dateStop": dateStop}
	raw := childrenRaw

	// Enrich raw với entity-level data (daily_budget, metaCreatedAt, ...) khi có
	switch objectType {
	case "campaign":
		enrichRawWithCampaignData(ctx, objectId, adAccountId, ownerOrgID, raw)
	case "adset":
		enrichRawWithAdSetData(ctx, objectId, adAccountId, ownerOrgID, raw)
	case "ad_account":
		enrichRawWithAdAccountData(ctx, adAccountId, ownerOrgID, raw)
	}

	layer1 := computeLayer1(raw)
	layer2 := computeLayer2(raw, layer1)
	// Bổ sung currentMode từ ad_account.accountMode — SL-B BLITZ/PROTECT dùng ngưỡng 0.20
	if adAccountId != "" {
		enrichLayer2WithAdAccountMode(ctx, adAccountId, ownerOrgID, layer2)
	}
	layer3 := computeLayer3(layer1, layer2)
	// Theo FolkForm v4.1: toàn bộ 13 rules CHỈ apply cho campaign. AdSet/AdAccount không có FLAG_RULE.
	// Rules engine cần raw.7d (có meta, pancake) — getRaw7d hỗ trợ cả cấu trúc mới và cũ.
	var alertFlags []string
	var actions []map[string]interface{}
	if objectType == "campaign" {
		cfg, _ := adsconfig.GetConfigForCampaign(ctx, adAccountId, ownerOrgID)
		alertFlags = computeAlertFlags(ctx, getRaw7d(raw), layer1, layer2, layer3, cfg, objectId, adAccountId, ownerOrgID)
		actions = computeSuggestedActions(ctx, alertFlags, adAccountId, ownerOrgID, cfg, getRaw7d(raw), layer1)
		// Cập nhật ads_camp_thresholds (P25/P50/P75) cho Per-Camp Adaptive — FolkForm v4.1 Section 2.2
		if metaCreatedAt := toInt64(getRaw7d(raw), "metaCreatedAt"); metaCreatedAt > 0 {
			if err := adsadaptive.ComputeCampThresholds(ctx, objectId, adAccountId, ownerOrgID, metaCreatedAt); err != nil {
				logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
					"campaignId": objectId, "adAccountId": adAccountId,
				}).Debug("[META] ComputeCampThresholds bỏ qua (camp chưa đủ data)")
			}
		}
	}
	current := map[string]interface{}{
		"raw":         raw,
		"layer1":     layer1,
		"layer2":     layer2,
		"layer3":     layer3,
		"alertFlags": alertFlags,
		"actions":    actions,
	}
	return updateParentCurrentMetrics(ctx, objectType, objectId, adAccountId, ownerOrgID, current)
}

// enrichLayer2WithAdAccountMode bổ sung currentMode vào layer2.
// Đọc từ ads_meta_config.account.accountMode (nguồn duy nhất).
// Dùng cho SL-B BLITZ/PROTECT: khi accountMode=BLITZ hoặc PROTECT thì ngưỡng spend_pct = 0.20.
func enrichLayer2WithAdAccountMode(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, layer2 map[string]interface{}) {
	if layer2 == nil || adAccountId == "" {
		return
	}
	cfg, _ := adsconfig.GetConfig(ctx, adAccountId, ownerOrgID)
	if cfg != nil && cfg.Account.AccountMode != "" {
		layer2["currentMode"] = strings.TrimSpace(cfg.Account.AccountMode)
	}
}

// enrichRawWithCampaignData bổ sung daily_budget, metaCreatedAt từ campaign document vào raw.
// Dùng cho spend_pct và runtime_minutes trong rules.
func enrichRawWithCampaignData(ctx context.Context, campaignId, adAccountId string, ownerOrgID primitive.ObjectID, raw map[string]interface{}) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
	if !ok {
		return
	}
	var doc struct {
		MetaData      map[string]interface{} `bson:"metaData"`
		MetaCreatedAt int64                 `bson:"metaCreatedAt"`
	}
	err := coll.FindOne(ctx, bson.M{
		"campaignId":         campaignId,
		"adAccountId":        adAccountIdFilterForMeta(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil {
		return
	}
	r7d := getRaw7d(raw)
	if meta, _ := r7d["meta"].(map[string]interface{}); meta != nil {
		if db := toFloat(doc.MetaData, "daily_budget"); db > 0 {
			meta["dailyBudget"] = db
		}
		// delivery_status từ metaData (insights) — cho KO-A
		if ds, ok := doc.MetaData["delivery_status"].(string); ok && ds != "" {
			meta["deliveryStatus"] = ds
		}
	}
	if doc.MetaCreatedAt > 0 {
		r7d["metaCreatedAt"] = doc.MetaCreatedAt
	}
}

// enrichRawWithAdSetData bổ sung metaCreatedAt từ adset document vào raw.
// Dùng cho lifecycle và runtimeMinutes khi rollup AdSet.
func enrichRawWithAdSetData(ctx context.Context, adSetId, adAccountId string, ownerOrgID primitive.ObjectID, raw map[string]interface{}) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdSets)
	if !ok {
		return
	}
	var doc struct {
		MetaCreatedAt int64 `bson:"metaCreatedAt"`
	}
	err := coll.FindOne(ctx, bson.M{
		"adSetId":             adSetId,
		"adAccountId":         adAccountIdFilterForMeta(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil || doc.MetaCreatedAt <= 0 {
		return
	}
	r7d := getRaw7d(raw)
	r7d["metaCreatedAt"] = doc.MetaCreatedAt
}

// enrichRawWithAdAccountData bổ sung metaCreatedAt từ ad account document vào raw.
// Dùng cho lifecycle và runtimeMinutes khi rollup AdAccount.
func enrichRawWithAdAccountData(ctx context.Context, adAccountId string, ownerOrgID primitive.ObjectID, raw map[string]interface{}) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
	if !ok {
		return
	}
	var doc struct {
		MetaCreatedAt int64 `bson:"metaCreatedAt"`
	}
	err := coll.FindOne(ctx, bson.M{
		"adAccountId":         adAccountIdFilterForMeta(adAccountId),
		"ownerOrganizationId": ownerOrgID,
	}).Decode(&doc)
	if err != nil || doc.MetaCreatedAt <= 0 {
		return
	}
	r7d := getRaw7d(raw)
	r7d["metaCreatedAt"] = doc.MetaCreatedAt
}

// aggregateRawFromChildren aggregate raw từ currentMetrics của children.
// Hỗ trợ cả cấu trúc mới (raw.7d, raw.2h, raw.1h) và cũ (raw phẳng).
func aggregateRawFromChildren(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID) (map[string]interface{}, error) {
	var coll *mongo.Collection
	var childIdField string
	switch objectType {
	case "adset":
		coll, _ = global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAds)
		childIdField = "adSetId"
	case "campaign":
		coll, _ = global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdSets)
		childIdField = "campaignId"
	case "ad_account":
		coll, _ = global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
		childIdField = "adAccountId"
	default:
		return nil, nil
	}
	if coll == nil {
		return nil, fmt.Errorf("không tìm thấy collection cho %s", objectType)
	}
	filter := bson.M{
		childIdField:          objectId,
		"ownerOrganizationId": ownerOrgID,
	}
	if objectType == "adset" || objectType == "campaign" {
		filter["adAccountId"] = adAccountIdFilterForMeta(adAccountId)
	}
	cursor, err := coll.Find(ctx, filter, nil)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Aggregator cho raw.7d (meta + pancake)
	agg7d := map[string]interface{}{
		"meta": map[string]interface{}{
			"spend": 0.0, "impressions": int64(0), "clicks": int64(0), "reach": int64(0),
			"mess": int64(0), "inlineLinkClicks": int64(0), "frequency": 0.0, "cpm": 0.0, "ctr": 0.0, "cpc": 0.0,
		},
		"pancake": map[string]interface{}{
			"pos": map[string]interface{}{"orders": int64(0), "revenue": 0.0},
			"conversation": map[string]interface{}{"conversationCount": int64(0)},
		},
	}
	meta := agg7d["meta"].(map[string]interface{})
	pancake := agg7d["pancake"].(map[string]interface{})
	pos := pancake["pos"].(map[string]interface{})
	conv := pancake["conversation"].(map[string]interface{})
	freqSum, freqCount := 0.0, 0
	cpmSum, cpmCount := 0.0, 0
	ctrSum, ctrCount := 0.0, 0
	cpcSum, cpcCount := 0.0, 0

	// Aggregator cho raw.2h, raw.1h, raw.30p
	agg2h := map[string]interface{}{"orders": int64(0), "revenue": 0.0, "mess": int64(0)}
	agg1h := map[string]interface{}{"orders": int64(0), "revenue": 0.0, "mess": int64(0)}
	agg30p := map[string]interface{}{"orders": int64(0), "revenue": 0.0, "mess": int64(0)}

	for cursor.Next(ctx) {
		var doc struct {
			CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.CurrentMetrics == nil {
			continue
		}
		raw, _ := doc.CurrentMetrics["raw"].(map[string]interface{})
		if raw == nil {
			continue
		}
		r7 := getRaw7d(raw)
		r2h := getRaw2h(raw)
		r1h := getRaw1h(raw)
		r30p := getRaw30p(raw)

		// Aggregate raw.7d
		m, _ := r7["meta"].(map[string]interface{})
		if m != nil {
			meta["spend"] = toFloat(meta, "spend") + toFloat(m, "spend")
			meta["impressions"] = toInt64(meta, "impressions") + toInt64(m, "impressions")
			meta["clicks"] = toInt64(meta, "clicks") + toInt64(m, "clicks")
			meta["reach"] = toInt64(meta, "reach") + toInt64(m, "reach")
			meta["mess"] = toInt64(meta, "mess") + toInt64(m, "mess")
			meta["inlineLinkClicks"] = toInt64(meta, "inlineLinkClicks") + toInt64(m, "inlineLinkClicks")
			if f := toFloat(m, "frequency"); f > 0 {
				freqSum += f
				freqCount++
			}
			if c := toFloat(m, "cpm"); c > 0 {
				cpmSum += c
				cpmCount++
			}
			if c := toFloat(m, "ctr"); c > 0 {
				ctrSum += c
				ctrCount++
			}
			if c := toFloat(m, "cpc"); c > 0 {
				cpcSum += c
				cpcCount++
			}
		}
		p, _ := r7["pancake"].(map[string]interface{})
		if p != nil {
			po, _ := p["pos"].(map[string]interface{})
			if po != nil {
				pos["orders"] = toInt64(pos, "orders") + toInt64(po, "orders")
				pos["revenue"] = toFloat(pos, "revenue") + toFloat(po, "revenue")
			}
			co, _ := p["conversation"].(map[string]interface{})
			if co != nil {
				conv["conversationCount"] = toInt64(conv, "conversationCount") + toInt64(co, "conversationCount")
			}
		}

		// Aggregate raw.2h, raw.1h, raw.30p
		if r2h != nil {
			agg2h["orders"] = toInt64(agg2h, "orders") + toInt64(r2h, "orders")
			agg2h["revenue"] = toFloat(agg2h, "revenue") + toFloat(r2h, "revenue")
			agg2h["mess"] = toInt64(agg2h, "mess") + toInt64(r2h, "mess")
		}
		if r1h != nil {
			agg1h["orders"] = toInt64(agg1h, "orders") + toInt64(r1h, "orders")
			agg1h["revenue"] = toFloat(agg1h, "revenue") + toFloat(r1h, "revenue")
			agg1h["mess"] = toInt64(agg1h, "mess") + toInt64(r1h, "mess")
		}
		if r30p != nil {
			agg30p["orders"] = toInt64(agg30p, "orders") + toInt64(r30p, "orders")
			agg30p["revenue"] = toFloat(agg30p, "revenue") + toFloat(r30p, "revenue")
			agg30p["mess"] = toInt64(agg30p, "mess") + toInt64(r30p, "mess")
		}
	}
	if freqCount > 0 {
		meta["frequency"] = freqSum / float64(freqCount)
	}
	if cpmCount > 0 {
		meta["cpm"] = cpmSum / float64(cpmCount)
	}
	if ctrCount > 0 {
		meta["ctr"] = ctrSum / float64(ctrCount)
	}
	if cpcCount > 0 {
		meta["cpc"] = cpcSum / float64(cpcCount)
	}

	// Trả về cấu trúc mới: raw.7d, raw.2h, raw.1h, raw.30p
	return map[string]interface{}{
		"7d":  agg7d,
		"2h":  agg2h,
		"1h":  agg1h,
		"30p": agg30p,
	}, nil
}

// updateParentCurrentMetrics cập nhật currentMetrics cho AdSet, Campaign, hoặc AdAccount.
// Ghi ads_activity_history khi có thay đổi.
func updateParentCurrentMetrics(ctx context.Context, objectType, objectId, adAccountId string, ownerOrgID primitive.ObjectID, current map[string]interface{}) error {
	var coll *mongo.Collection
	var idField string
	switch objectType {
	case "adset":
		coll, _ = global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdSets)
		idField = "adSetId"
	case "campaign":
		coll, _ = global.RegistryCollections.Get(global.MongoDB_ColNames.MetaCampaigns)
		idField = "campaignId"
	case "ad_account":
		coll, _ = global.RegistryCollections.Get(global.MongoDB_ColNames.MetaAdAccounts)
		idField = "adAccountId"
	default:
		return fmt.Errorf("objectType không hỗ trợ roll-up: %s", objectType)
	}
	if coll == nil {
		return fmt.Errorf("không tìm thấy collection cho %s", objectType)
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	if objectType == "ad_account" {
		filter[idField] = adAccountIdFilterForMeta(adAccountId)
	} else {
		filter[idField] = objectId
		filter["adAccountId"] = adAccountIdFilterForMeta(adAccountId)
	}
	old := getParentCurrentMetrics(ctx, coll, filter)
	recordActivityForEntity(ctx, objectType, objectId, adAccountId, ownerOrgID, old, current, "rollup")
	_, err := coll.UpdateOne(ctx, filter, bson.M{"$set": bson.M{"currentMetrics": current, "updatedAt": time.Now().UnixMilli()}})
	return err
}

// getParentCurrentMetrics lấy currentMetrics từ document theo filter.
func getParentCurrentMetrics(ctx context.Context, coll *mongo.Collection, filter bson.M) map[string]interface{} {
	var doc struct {
		CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
	}
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil || doc.CurrentMetrics == nil {
		return make(map[string]interface{})
	}
	return doc.CurrentMetrics
}

func getWindowDates(days int) (string, string) {
	if days <= 0 {
		days = DefaultWindowDays
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	end := now
	start := now.AddDate(0, 0, -days+1)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

// getWindowMsRangeForShortCycle trả về (startMs, endMs) cho chu kỳ ngắn (30p, 1h, 2h) align theo boundary.
// Ví dụ: now=14:47 → 30p: 14:00-14:30, 1h: 13:00-14:00, 2h: 12:00-14:00 (slot đã hoàn thành gần nhất).
func getWindowMsRangeForShortCycle(windowMinutes int) (startMs, endMs int64) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	var end time.Time
	switch windowMinutes {
	case 30:
		m := now.Minute() / 30 * 30
		end = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), m, 0, 0, loc)
	case 60:
		end = time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
	case 120:
		h := now.Hour() / 2 * 2
		end = time.Date(now.Year(), now.Month(), now.Day(), h, 0, 0, 0, loc)
	default:
		nowMs := now.UnixMilli()
		wMs := int64(windowMinutes) * 60 * 1000
		return nowMs - wMs, nowMs
	}
	endMs = end.UnixMilli()
	wMs := int64(windowMinutes) * 60 * 1000
	startMs = endMs - wMs
	return startMs, endMs
}

// getWindowMsRangeFromDates trả về (startMs, endMs) theo calendar dates từ getWindowDates.
// Dùng để align raw.7d pos/conversation với meta (cùng khoảng thời gian theo múi giờ Asia/Ho_Chi_Minh).
func getWindowMsRangeFromDates(days int) (startMs, endMs int64) {
	if days <= 0 {
		days = DefaultWindowDays
	}
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endDate := now
	startDate := now.AddDate(0, 0, -days+1)
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, loc)
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999999999, loc)
	return start.UnixMilli(), end.UnixMilli()
}

func getWindowMs(days int) int64 {
	if days <= 0 {
		days = DefaultWindowDays
	}
	return int64(days) * 24 * 60 * 60 * 1000
}

// getWindowMsForCurrentMetrics trả về windowMs cho currentMetrics (7d).
// Ưu tiên từ ads_metric_definitions; fallback DefaultWindowDays.
func getWindowMsForCurrentMetrics(ctx context.Context) int64 {
	return adsconfig.GetWindowMsForCurrentMetrics(ctx)
}
