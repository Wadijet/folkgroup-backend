// Script chẩn đoán: Kiểm tra tại sao Meta ads currentMetrics không có orders/conversations.
// Chạy: go run scripts/diagnose_meta_ads_metrics.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func loadEnv() {
	tryPaths := []string{".env", "api/.env", "config/env/development.env", "api/config/env/development.env"}
	cwd, _ := os.Getwd()
	for _, p := range tryPaths {
		full := filepath.Join(cwd, p)
		if _, err := os.Stat(full); err == nil {
			_ = godotenv.Load(full)
			return
		}
	}
}

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	orders := db.Collection("pc_pos_orders")
	convs := db.Collection("fb_conversations")
	ads := db.Collection("meta_ads")
	insights := db.Collection("meta_ad_insights")

	fmt.Println("=== CHẨN ĐOÁN META ADS METRICS ===\n")

	// 0. Kiểm tra raw META (meta_ad_insights → currentMetrics.raw.meta)
	fmt.Println("--- RAW META (meta_ad_insights) ---")
	insightCount, _ := insights.CountDocuments(ctx, bson.M{})
	insightAdCount, _ := insights.CountDocuments(ctx, bson.M{"objectType": "ad"})
	fmt.Printf("0. meta_ad_insights: %d docs tổng, %d objectType=ad\n", insightCount, insightAdCount)

	// Mẫu insight
	var sampleInsight bson.M
	if insights.FindOne(ctx, bson.M{"objectType": "ad"}, nil).Decode(&sampleInsight) == nil {
		objId := sampleInsight["objectId"]
		objType := sampleInsight["objectType"]
		accId := sampleInsight["adAccountId"]
		dateStart := sampleInsight["dateStart"]
		fmt.Printf("   Mẫu insight: objectType=%v objectId=%v adAccountId=%v dateStart=%v\n", objType, objId, accId, dateStart)
	}

	// Mẫu meta_ads
	var sampleAd bson.M
	if ads.FindOne(ctx, bson.M{}, nil).Decode(&sampleAd) == nil {
		adId := sampleAd["adId"]
		accId := sampleAd["adAccountId"]
		fmt.Printf("   Mẫu meta_ads: adId=%v adAccountId=%v\n", adId, accId)
	}

	// Kiểm tra ad có insight không (objectId=adId, objectType=ad)
	if len(fmt.Sprintf("%v", sampleAd["adId"])) > 0 {
		testAdId := fmt.Sprintf("%v", sampleAd["adId"])
		testAccId := fmt.Sprintf("%v", sampleAd["adAccountId"])
		insightMatch, _ := insights.CountDocuments(ctx, bson.M{
			"objectType": "ad",
			"objectId":   testAdId,
			"adAccountId": testAccId,
		})
		fmt.Printf("   Ad %s + adAccountId %s: %d insights match (exact)\n", testAdId, testAccId, insightMatch)
		// Thử với act_ prefix
		if insightMatch == 0 && !strings.HasPrefix(testAccId, "act_") {
			insightMatchAct, _ := insights.CountDocuments(ctx, bson.M{
				"objectType":  "ad",
				"objectId":    testAdId,
				"adAccountId": "act_" + testAccId,
			})
			fmt.Printf("   Thử adAccountId=act_%s: %d insights match\n", testAccId, insightMatchAct)
		}
	}

	// Thống kê currentMetrics.raw.meta
	adsWithMeta := 0
	adsWithMetaNonZero := 0
	curMeta, _ := ads.Find(ctx, bson.M{"currentMetrics.raw.meta": bson.M{"$exists": true}}, nil)
	for curMeta.Next(ctx) {
		var d bson.M
		_ = curMeta.Decode(&d)
		adsWithMeta++
		if cm, ok := d["currentMetrics"].(bson.M); ok {
			if raw, ok := cm["raw"].(bson.M); ok {
				if meta, ok := raw["meta"].(bson.M); ok {
					spend := getFloat(meta, "spend")
					if spend > 0 {
						adsWithMetaNonZero++
					}
				}
			}
		}
	}
	curMeta.Close(ctx)
	totalAds, _ := ads.CountDocuments(ctx, bson.M{})
	fmt.Printf("   meta_ads: %d/%d có raw.meta, %d có spend>0\n\n", adsWithMeta, totalAds, adsWithMetaNonZero)

	// 1. Lấy distinct ad_id từ orders
	orderAdIds, _ := orders.Distinct(ctx, "posData.ad_id", bson.M{"posData.ad_id": bson.M{"$exists": true, "$ne": ""}})
	fmt.Printf("1. pc_pos_orders: %d distinct posData.ad_id\n", len(orderAdIds))
	if len(orderAdIds) > 0 {
		fmt.Printf("   Mẫu 5 ad_id từ orders: %v\n", orderAdIds[:min(5, len(orderAdIds))])
	}

	// 2. Lấy distinct adId từ meta_ads
	var adIdsFromMeta []string
	cur, _ := ads.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"adId": 1}))
	for cur.Next(ctx) {
		var d bson.M
		_ = cur.Decode(&d)
		if id, ok := d["adId"].(string); ok && id != "" {
			adIdsFromMeta = append(adIdsFromMeta, id)
		}
	}
	cur.Close(ctx)
	fmt.Printf("\n2. meta_ads: %d ads có adId\n", len(adIdsFromMeta))
	if len(adIdsFromMeta) > 0 {
		fmt.Printf("   Mẫu 5 adId từ meta_ads: %v\n", adIdsFromMeta[:min(5, len(adIdsFromMeta))])
	}

	// 3. Kiểm tra overlap: orders.ad_id có trong meta_ads không?
	metaSet := make(map[string]bool)
	for _, id := range adIdsFromMeta {
		metaSet[id] = true
	}
	overlapOrders := 0
	for _, v := range orderAdIds {
		s := fmt.Sprintf("%v", v)
		if metaSet[s] {
			overlapOrders++
		}
	}
	fmt.Printf("\n3. Overlap: %d/%d ad_id từ orders có trong meta_ads\n", overlapOrders, len(orderAdIds))
	if overlapOrders == 0 && len(orderAdIds) > 0 {
		fmt.Println("   ⚠️ KHÔNG CÓ OVERLAP! Có thể orders.posData.ad_id lưu post_id thay vì ad_id.")
	}

	// 4. Lấy ad_ids từ fb_conversations (panCakeData.ad_ids array)
	var convAdIds []string
	cur2, _ := convs.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"panCakeData.ad_ids.0": bson.M{"$exists": true}}}},
		{{Key: "$unwind", Value: "$panCakeData.ad_ids"}},
		{{Key: "$group", Value: bson.M{"_id": "$panCakeData.ad_ids"}}},
		{{Key: "$limit", Value: 500}},
	})
	for cur2.Next(ctx) {
		var d bson.M
		_ = cur2.Decode(&d)
		if id, ok := d["_id"].(string); ok && id != "" {
			convAdIds = append(convAdIds, id)
		}
	}
	cur2.Close(ctx)
	fmt.Printf("\n4. fb_conversations: %d distinct ad_id từ panCakeData.ad_ids (mẫu)\n", len(convAdIds))
	if len(convAdIds) > 0 {
		fmt.Printf("   Mẫu 5: %v\n", convAdIds[:min(5, len(convAdIds))])
		overlapConv := 0
		for _, id := range convAdIds {
			if metaSet[id] {
				overlapConv++
			}
		}
		fmt.Printf("   Overlap với meta_ads: %d/%d\n", overlapConv, len(convAdIds))
	}

	// 5. Với 1 ad có trong meta_ads, thử query orders và convs trực tiếp
	if len(adIdsFromMeta) > 0 {
		sampleAdId := adIdsFromMeta[0]
		ordCount, _ := orders.CountDocuments(ctx, bson.M{"posData.ad_id": sampleAdId})
		convCount, _ := convs.CountDocuments(ctx, bson.M{
			"$or": []bson.M{
				{"panCakeData.ad_ids": sampleAdId},
				{"panCakeData.ads.ad_id": sampleAdId},
			},
		})
		fmt.Printf("\n5. Ad mẫu adId=%s:\n", sampleAdId)
		fmt.Printf("   - Orders match posData.ad_id: %d\n", ordCount)
		fmt.Printf("   - Conversations match: %d\n", convCount)

		// Lấy currentMetrics của ad này
		var adDoc bson.M
		if ads.FindOne(ctx, bson.M{"adId": sampleAdId}).Decode(&adDoc) == nil {
			if cm, ok := adDoc["currentMetrics"].(bson.M); ok {
				raw, _ := cm["raw"].(bson.M)
				if raw != nil {
					meta, _ := raw["meta"].(bson.M)
					pancake, _ := raw["pancake"].(bson.M)
					fmt.Printf("   - currentMetrics.raw.meta: %v\n", meta)
					fmt.Printf("   - currentMetrics.raw.pancake: %v\n", pancake)
				} else {
					fmt.Printf("   - currentMetrics.raw: nil\n")
				}
			} else {
				fmt.Printf("   - currentMetrics: nil hoặc không parse được\n")
			}
		}
	}

	// 6. Tìm 1 order có ad_id KHỚP với meta_ads để xem metrics
	for _, v := range orderAdIds {
		s := fmt.Sprintf("%v", v)
		if metaSet[s] {
			ordCount, _ := orders.CountDocuments(ctx, bson.M{"posData.ad_id": s})
			var adDoc bson.M
			if ads.FindOne(ctx, bson.M{"adId": s}).Decode(&adDoc) == nil {
				orgID, _ := adDoc["ownerOrganizationId"]
				fmt.Printf("\n6. Ad CÓ OVERLAP: adId=%s | orders=%d | ownerOrg=%v\n", s, ordCount, orgID)
				if cm, ok := adDoc["currentMetrics"].(bson.M); ok && cm != nil {
					raw, _ := cm["raw"].(bson.M)
					if raw != nil {
						meta, _ := raw["meta"].(bson.M)
						pancake, _ := raw["pancake"].(bson.M)
						fmt.Printf("   currentMetrics.raw.meta: %v\n", meta)
						fmt.Printf("   currentMetrics.raw.pancake: %v\n", pancake)
					}
				}
				break
			}
		}
	}

	// 7. Kiểm tra insight cho ad có overlap
	fmt.Println("\n--- KIỂM TRA INSIGHT CHO AD CÓ OVERLAP ---")
	var overlapAdId, overlapAccId string
	insightExact, insightWithIn := int64(0), int64(0)
	for _, v := range orderAdIds {
		s := fmt.Sprintf("%v", v)
		if metaSet[s] {
			var adDoc bson.M
			if ads.FindOne(ctx, bson.M{"adId": s}).Decode(&adDoc) == nil {
				overlapAdId = s
				overlapAccId = fmt.Sprintf("%v", adDoc["adAccountId"])
				break
			}
		}
	}
	if overlapAdId != "" {
		insightExact, _ = insights.CountDocuments(ctx, bson.M{
			"objectType":  "ad",
			"objectId":    overlapAdId,
			"adAccountId": overlapAccId,
		})
		accAct := overlapAccId
		if !strings.HasPrefix(overlapAccId, "act_") {
			accAct = "act_" + overlapAccId
		}
		insightWithIn, _ = insights.CountDocuments(ctx, bson.M{
			"objectType":  "ad",
			"objectId":    overlapAdId,
			"adAccountId": bson.M{"$in": bson.A{overlapAccId, accAct}},
		})
		var sampleInsightAd bson.M
		_ = insights.FindOne(ctx, bson.M{"objectType": "ad", "objectId": overlapAdId}, nil).Decode(&sampleInsightAd)
		fmt.Printf("7. Ad %s (adAccountId=%s): %d insights exact, %d với $in [%s, %s]\n", overlapAdId, overlapAccId, insightExact, insightWithIn, overlapAccId, accAct)
		if sampleInsightAd != nil {
			fmt.Printf("   Mẫu insight adAccountId=%v dateStart=%v spend=%v\n", sampleInsightAd["adAccountId"], sampleInsightAd["dateStart"], sampleInsightAd["spend"])
		}
	}

	fmt.Println("\n=== KẾT LUẬN ===")
	if overlapOrders == 0 {
		fmt.Println("• posData.ad_id trong orders KHÔNG khớp với meta_ads.adId → recalculate sẽ trả orders=0")
		fmt.Println("• Cần kiểm tra: Pancake gửi post_id hay ad_id trong posData.ad_id?")
	}
	if overlapAdId != "" && insightExact == 0 && insightWithIn == 0 {
		fmt.Printf("• Ad %s: KHÔNG CÓ insight trong meta_ad_insights → raw.meta sẽ = 0 (cần sync insights cho ad này)\n", overlapAdId)
	}
	if adsWithMetaNonZero < int(totalAds) && insightCount > 0 {
		fmt.Println("• Đã sửa fetchRawMetaFromInsights: dùng adAccountIdFilterForMeta để match cả act_XXX và XXX")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getFloat(m bson.M, k string) float64 {
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
	}
	return 0
}

