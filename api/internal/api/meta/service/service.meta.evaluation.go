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
	windowMs := getWindowMs(DefaultWindowDays)

	// Lấy currentMetrics hiện tại
	current, err := getAdCurrentMetrics(ctx, objectId, adAccountId, ownerOrgID)
	if err != nil {
		return err
	}
	raw, _ := current["raw"].(map[string]interface{})
	if raw == nil {
		raw = make(map[string]interface{})
	}

	// Chỉ cập nhật raw từ nguồn được chỉ định
	switch source {
	case "meta":
		metaRaw, err := fetchRawMetaFromInsights(ctx, objectType, objectId, adAccountId, ownerOrgID, dateStart, dateStop)
		if err != nil {
			return fmt.Errorf("fetch raw meta: %w", err)
		}
		raw["meta"] = metaRaw
	case "pancake.pos":
		posRaw, err := fetchRawPosFromOrders(ctx, objectId, ownerOrgID, windowMs)
		if err != nil {
			return fmt.Errorf("fetch raw pos: %w", err)
		}
		if raw["pancake"] == nil {
			raw["pancake"] = make(map[string]interface{})
		}
		pancake, _ := raw["pancake"].(map[string]interface{})
		if pancake == nil {
			pancake = make(map[string]interface{})
		}
		pancake["pos"] = posRaw
		raw["pancake"] = pancake
	case "pancake.conversation":
		convRaw, err := fetchRawConversationFromConversations(ctx, objectId, ownerOrgID, windowMs)
		if err != nil {
			return fmt.Errorf("fetch raw conversation: %w", err)
		}
		if raw["pancake"] == nil {
			raw["pancake"] = make(map[string]interface{})
		}
		pancake, _ := raw["pancake"].(map[string]interface{})
		if pancake == nil {
			pancake = make(map[string]interface{})
		}
		pancake["conversation"] = convRaw
		raw["pancake"] = pancake
	default:
		return fmt.Errorf("nguồn không hợp lệ: %s", source)
	}

	raw["window"] = map[string]interface{}{
		"dateStart": dateStart,
		"dateStop":  dateStop,
	}
	raw["metaCreatedAt"] = fetchAdMetaCreatedAt(ctx, objectId, adAccountId, ownerOrgID)
	current["raw"] = raw

	// Tính layer1, layer2, layer3, alertFlags
	layer1 := computeLayer1(raw)
	layer2 := computeLayer2(raw, layer1)
	layer3 := computeLayer3(layer1, layer2)
	current["layer1"] = layer1
	current["layer2"] = layer2
	current["layer3"] = layer3
	current["alertFlags"] = computeAlertFlags(raw, layer1, layer2, layer3)

	return updateAdCurrentMetrics(ctx, objectId, adAccountId, ownerOrgID, current, source)
}

// updateRawAndLayersForAd tính đầy đủ raw từ 3 nguồn rồi tính layers cho Ad.
func updateRawAndLayersForAd(ctx context.Context, adId, adAccountId string, ownerOrgID primitive.ObjectID) error {
	dateStart, dateStop := getWindowDates(DefaultWindowDays)
	windowMs := getWindowMs(DefaultWindowDays)

	raw := make(map[string]interface{})
	metaRaw, err := fetchRawMetaFromInsights(ctx, "ad", adId, adAccountId, ownerOrgID, dateStart, dateStop)
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("[ADS_PROFILE] Không lấy được raw meta")
	} else {
		raw["meta"] = metaRaw
	}
	posRaw, err := fetchRawPosFromOrders(ctx, adId, ownerOrgID, windowMs)
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("[ADS_PROFILE] Không lấy được raw pos")
	} else {
		raw["pancake"] = map[string]interface{}{"pos": posRaw}
	}
	convRaw, err := fetchRawConversationFromConversations(ctx, adId, ownerOrgID, windowMs)
	if err != nil {
		logger.GetAppLogger().WithError(err).Warn("[ADS_PROFILE] Không lấy được raw conversation")
	} else {
		if raw["pancake"] == nil {
			raw["pancake"] = make(map[string]interface{})
		}
		pancake, _ := raw["pancake"].(map[string]interface{})
		if pancake == nil {
			pancake = make(map[string]interface{})
		}
		pancake["conversation"] = convRaw
		raw["pancake"] = pancake
	}
	raw["window"] = map[string]interface{}{"dateStart": dateStart, "dateStop": dateStop}
	raw["metaCreatedAt"] = fetchAdMetaCreatedAt(ctx, adId, adAccountId, ownerOrgID)

	layer1 := computeLayer1(raw)
	layer2 := computeLayer2(raw, layer1)
	layer3 := computeLayer3(layer1, layer2)
	alertFlags := computeAlertFlags(raw, layer1, layer2, layer3)

	current := map[string]interface{}{
		"raw":         raw,
		"layer1":     layer1,
		"layer2":     layer2,
		"layer3":     layer3,
		"alertFlags": alertFlags,
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

func computeLayer1(raw map[string]interface{}) map[string]interface{} {
	meta, _ := raw["meta"].(map[string]interface{})
	pancake, _ := raw["pancake"].(map[string]interface{})
	pos, _ := mapOrNil(pancake, "pos").(map[string]interface{})

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
	roas := 0.0
	if spend > 0 {
		roas = revenue / spend
	}

	// Lifecycle: ưu tiên metaCreatedAt (thời gian tạo gốc từ Meta) — ad trong learning phase (< 7 ngày) = NEW
	lifecycle := "NEW"
	metaCreatedAt := toInt64(raw, "metaCreatedAt")
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

	return map[string]interface{}{
		"lifecycle":    lifecycle,
		"msgRate":      msgRate,
		"cpaMess":      cpaMess,
		"cpaPurchase":  cpaPurchase,
		"convRate":     convRate,
		"roas":         roas,
	}
}

func computeLayer2(raw map[string]interface{}, layer1 map[string]interface{}) map[string]interface{} {
	meta, _ := raw["meta"].(map[string]interface{})
	cpm := toFloat(meta, "cpm")
	ctr := toFloat(meta, "ctr")
	frequency := toFloat(meta, "frequency")

	// Đơn giản hóa: 5 trục 0-100 dựa trên ngưỡng
	efficiency := scoreFromRoas(toFloat(layer1, "roas"))
	demandQuality := scoreFromRate(toFloat(layer1, "msgRate"), toFloat(layer1, "convRate"))
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

	roas := toFloat(layer1, "roas")
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

// fetchRawPosFromOrders aggregate pc_pos_orders có posData.ad_id = adId → raw.pancake.pos.
func fetchRawPosFromOrders(ctx context.Context, adId string, ownerOrgID primitive.ObjectID, windowMs int64) (map[string]interface{}, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection pc_pos_orders")
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"posData.ad_id":       adId,
	}
	_ = windowMs // Phase 1: không filter theo thời gian

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

// fetchRawConversationFromConversations đếm fb_conversations có panCakeData.ad_ids chứa adId hoặc ads[].ad_id = adId.
func fetchRawConversationFromConversations(ctx context.Context, adId string, ownerOrgID primitive.ObjectID, windowMs int64) (map[string]interface{}, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.FbConvesations)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection fb_conversations")
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or": []bson.M{
			{"panCakeData.ad_ids": adId},
			{"panCakeData.ads.ad_id": adId},
		},
	}
	_ = windowMs // Phase 1: không filter theo thời gian
	count, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
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
	childrenRaw["window"] = map[string]interface{}{"dateStart": dateStart, "dateStop": dateStop}
	raw := childrenRaw

	layer1 := computeLayer1(raw)
	layer2 := computeLayer2(raw, layer1)
	layer3 := computeLayer3(layer1, layer2)
	alertFlags := computeAlertFlags(raw, layer1, layer2, layer3)
	current := map[string]interface{}{
		"raw":         raw,
		"layer1":     layer1,
		"layer2":     layer2,
		"layer3":     layer3,
		"alertFlags": alertFlags,
	}
	return updateParentCurrentMetrics(ctx, objectType, objectId, adAccountId, ownerOrgID, current)
}

// aggregateRawFromChildren aggregate raw từ currentMetrics của children.
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

	agg := map[string]interface{}{
		"meta": map[string]interface{}{
			"spend": 0.0, "impressions": int64(0), "clicks": int64(0), "reach": int64(0),
			"mess": int64(0), "inlineLinkClicks": int64(0), "frequency": 0.0, "cpm": 0.0, "ctr": 0.0, "cpc": 0.0,
		},
		"pancake": map[string]interface{}{
			"pos": map[string]interface{}{"orders": int64(0), "revenue": 0.0},
			"conversation": map[string]interface{}{"conversationCount": int64(0)},
		},
	}
	meta := agg["meta"].(map[string]interface{})
	pancake := agg["pancake"].(map[string]interface{})
	pos := pancake["pos"].(map[string]interface{})
	conv := pancake["conversation"].(map[string]interface{})
	freqSum, freqCount := 0.0, 0
	cpmSum, cpmCount := 0.0, 0
	ctrSum, ctrCount := 0.0, 0
	cpcSum, cpcCount := 0.0, 0

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
		m, _ := raw["meta"].(map[string]interface{})
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
		p, _ := raw["pancake"].(map[string]interface{})
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
	return agg, nil
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

func getWindowMs(days int) int64 {
	if days <= 0 {
		days = DefaultWindowDays
	}
	return int64(days) * 24 * 60 * 60 * 1000
}
