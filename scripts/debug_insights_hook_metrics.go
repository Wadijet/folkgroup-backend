// Script chẩn đoán luồng hook: meta_ad_insights → currentMetrics ở các level (Ad, AdSet, Campaign, Account).
// Kiểm tra: insights có đủ field? meta_ads có linkage? currentMetrics có được cập nhật?
// Chạy: go run scripts/debug_insights_hook_metrics.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
		parent := filepath.Dir(cwd)
		if _, err := os.Stat(filepath.Join(parent, p)); err == nil {
			_ = godotenv.Load(filepath.Join(parent, p))
			return
		}
	}
}

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH trong .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	insightsColl := db.Collection("meta_ad_insights")
	adsColl := db.Collection("meta_ads")
	adSetsColl := db.Collection("meta_adsets")
	campaignsColl := db.Collection("meta_campaigns")
	accountsColl := db.Collection("meta_ad_accounts")

	fmt.Println("=== CHẨN ĐOÁN LUỒNG HOOK: INSIGHTS → METRICS ===\n")

	// 1. Thống kê meta_ad_insights
	insightTotal, _ := insightsColl.CountDocuments(ctx, bson.M{})
	fmt.Printf("1. meta_ad_insights: %d documents\n", insightTotal)

	if insightTotal == 0 {
		fmt.Println("\n⚠️ Collection meta_ad_insights TRỐNG — chưa có insight nào. Hook không chạy.")
		return
	}

	// Đếm theo objectType
	for _, ot := range []string{"ad", "adset", "campaign", "ad_account"} {
		n, _ := insightsColl.CountDocuments(ctx, bson.M{"objectType": ot})
		fmt.Printf("   - objectType=%s: %d\n", ot, n)
	}

	// Kiểm tra field bắt buộc cho hook (ObjectType, ObjectId, AdAccountId, OwnerOrganizationID)
	missingObjType, _ := insightsColl.CountDocuments(ctx, bson.M{"$or": []bson.M{{"objectType": ""}, {"objectType": bson.M{"$exists": false}}}})
	missingObjId, _ := insightsColl.CountDocuments(ctx, bson.M{"$or": []bson.M{{"objectId": ""}, {"objectId": bson.M{"$exists": false}}}})
	missingAdAcc, _ := insightsColl.CountDocuments(ctx, bson.M{"$or": []bson.M{{"adAccountId": ""}, {"adAccountId": bson.M{"$exists": false}}}})
	missingOwner, _ := insightsColl.CountDocuments(ctx, bson.M{"$or": []bson.M{{"ownerOrganizationId": nil}, {"ownerOrganizationId": bson.M{"$exists": false}}}})

	fmt.Printf("\n2. Field bắt buộc cho hook (hook bỏ qua nếu thiếu):\n")
	fmt.Printf("   - objectType rỗng/thiếu: %d\n", missingObjType)
	fmt.Printf("   - objectId rỗng/thiếu: %d\n", missingObjId)
	fmt.Printf("   - adAccountId rỗng/thiếu: %d\n", missingAdAcc)
	fmt.Printf("   - ownerOrganizationId rỗng/thiếu: %d\n", missingOwner)

	if missingObjType > 0 || missingObjId > 0 || missingAdAcc > 0 || missingOwner > 0 {
		fmt.Println("\n   ⚠️ Có insight thiếu field → hook KHÔNG xử lý (return sớm).")
	}

	// 3. Lấy mẫu insights (objectType=ad) — kiểm tra linkage với meta_ads
	adInsightCount, _ := insightsColl.CountDocuments(ctx, bson.M{"objectType": "ad"})
	fmt.Printf("\n3. Insights objectType=ad: %d\n", adInsightCount)

	if adInsightCount > 0 {
		cursor, _ := insightsColl.Find(ctx, bson.M{"objectType": "ad"},
			options.Find().SetLimit(5).SetSort(bson.M{"updatedAt": -1}))
		linked := 0
		unlinked := 0
		var unlinkedSamples []string
		for cursor.Next(ctx) {
			var doc bson.M
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			objId, _ := doc["objectId"].(string)
			adAccId, _ := doc["adAccountId"].(string)
			ownerOrg := doc["ownerOrganizationId"]
			if objId == "" || adAccId == "" {
				unlinked++
				unlinkedSamples = append(unlinkedSamples, fmt.Sprintf("objectId=%q adAccountId=%q", objId, adAccId))
				continue
			}
			filter := bson.M{
				"adId":                 objId,
				"adAccountId":          adAccId,
				"ownerOrganizationId": ownerOrg,
			}
			n, _ := adsColl.CountDocuments(ctx, filter)
			if n > 0 {
				linked++
			} else {
				// Thử chỉ adId+adAccountId (bỏ ownerOrg) để xem có phải do ownerOrg khác không
				n2, _ := adsColl.CountDocuments(ctx, bson.M{"adId": objId, "adAccountId": adAccId})
				reason := "meta_ads không có"
				if n2 > 0 {
					reason = "ownerOrganizationId KHÁC giữa insight và meta_ads!"
				}
				unlinked++
				unlinkedSamples = append(unlinkedSamples, fmt.Sprintf("adId=%q adAccountId=%q (%s)", objId, adAccId, reason))
			}
		}
		cursor.Close(ctx)
		fmt.Printf("   - Trong 5 mẫu: %d có meta_ads tương ứng, %d KHÔNG có\n", linked, unlinked)
		if len(unlinkedSamples) > 0 {
			fmt.Println("   - Mẫu không link được:")
			for _, s := range unlinkedSamples {
				fmt.Printf("     • %s\n", s)
			}
			fmt.Println("\n   ⚠️ Insight có objectType=ad nhưng meta_ads không có adId tương ứng → UpdateRawFromSource cập nhật meta_ads nhưng filter không match!")
		}
	}

	// 4. Thống kê currentMetrics ở các level
	fmt.Println("\n4. currentMetrics ở các level:")
	for _, row := range []struct {
		name string
		coll *mongo.Collection
		idField string
	}{
		{"meta_ads", adsColl, "adId"},
		{"meta_adsets", adSetsColl, "adSetId"},
		{"meta_campaigns", campaignsColl, "campaignId"},
		{"meta_ad_accounts", accountsColl, "adAccountId"},
	} {
		total, _ := row.coll.CountDocuments(ctx, bson.M{})
		withMetrics, _ := row.coll.CountDocuments(ctx, bson.M{"currentMetrics": bson.M{"$exists": true, "$ne": nil}})
		withRaw, _ := row.coll.CountDocuments(ctx, bson.M{"currentMetrics.raw": bson.M{"$exists": true}})
		withLayer1, _ := row.coll.CountDocuments(ctx, bson.M{"currentMetrics.layer1": bson.M{"$exists": true}})
		fmt.Printf("   %s: total=%d, có currentMetrics=%d, có raw=%d, có layer1=%d\n",
			row.name, total, withMetrics, withRaw, withLayer1)
	}

	// 5. Mẫu meta_ads có insight nhưng không có currentMetrics
	fmt.Println("\n5. Kiểm tra meta_ads: có insight (objectType=ad) nhưng chưa có currentMetrics:")
	// Lấy adIds từ insights
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"objectType": "ad"}}},
		{{Key: "$group", Value: bson.M{"_id": bson.M{"objectId": "$objectId", "adAccountId": "$adAccountId", "ownerOrganizationId": "$ownerOrganizationId"}}}},
		{{Key: "$limit", Value: 20}},
	}
	cursor, err := insightsColl.Aggregate(ctx, pipe)
	if err != nil {
		fmt.Printf("   Lỗi aggregate: %v\n", err)
	} else {
		missingCount := 0
		for cursor.Next(ctx) {
			var doc bson.M
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			id, ok := doc["_id"].(bson.M)
			if !ok {
				continue
			}
			objId, _ := id["objectId"].(string)
			adAccId, _ := id["adAccountId"].(string)
			ownerOrg := id["ownerOrganizationId"]
			if objId == "" || adAccId == "" {
				continue
			}
			// Kiểm tra meta_ads có ad này không
			adFilter := bson.M{"adId": objId, "adAccountId": adAccId, "ownerOrganizationId": ownerOrg}
			var adDoc bson.M
			err := adsColl.FindOne(ctx, adFilter).Decode(&adDoc)
			if err != nil {
				missingCount++
				if missingCount <= 3 {
					fmt.Printf("   - adId=%q: meta_ads KHÔNG TÌM THẤY (có thể chưa sync ads)\n", objId)
				}
				continue
			}
			cm, _ := adDoc["currentMetrics"]
			if cm == nil {
				missingCount++
				if missingCount <= 5 {
					fmt.Printf("   - adId=%q: meta_ads có document nhưng currentMetrics=NULL\n", objId)
				}
			}
		}
		cursor.Close(ctx)
		if missingCount > 0 {
			fmt.Printf("   → Tổng %d ad có insight nhưng chưa có currentMetrics (hoặc chưa có trong meta_ads)\n", missingCount)
		} else {
			fmt.Println("   → OK: Các ad có insight đều có currentMetrics hoặc chưa có trong meta_ads.")
		}
	}

	// 6. So sánh adId: insights vs meta_ads (có overlap không?)
	fmt.Println("\n6. So sánh adId giữa insights và meta_ads:")
	insightAdIds := make(map[string]bool)
	cur, _ := insightsColl.Find(ctx, bson.M{"objectType": "ad"}, options.Find().SetProjection(bson.M{"objectId": 1}))
	for cur.Next(ctx) {
		var d bson.M
		if err := cur.Decode(&d); err != nil {
			continue
		}
		if id, ok := d["objectId"].(string); ok && id != "" {
			insightAdIds[id] = true
		}
	}
	cur.Close(ctx)

	adsAdIds := make(map[string]bool)
	cur2, _ := adsColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"adId": 1}))
	for cur2.Next(ctx) {
		var d bson.M
		if err := cur2.Decode(&d); err != nil {
			continue
		}
		if id, ok := d["adId"].(string); ok && id != "" {
			adsAdIds[id] = true
		}
	}
	cur2.Close(ctx)

	overlap := 0
	for id := range insightAdIds {
		if adsAdIds[id] {
			overlap++
		}
	}
	fmt.Printf("   - Số adId unique trong insights: %d\n", len(insightAdIds))
	fmt.Printf("   - Số adId unique trong meta_ads: %d\n", len(adsAdIds))
	fmt.Printf("   - Số adId trùng (có trong cả 2): %d\n", overlap)
	if overlap == 0 && len(insightAdIds) > 0 && len(adsAdIds) > 0 {
		fmt.Println("   ⚠️ KHÔNG CÓ overlap — adId trong insights KHÁC adId trong meta_ads!")
	} else if overlap > 0 {
		// Có overlap nhưng bước 3 báo "không link" → có thể ownerOrg hoặc adAccountId khác
		fmt.Println("   → Có overlap nhưng filter (adId+adAccountId+ownerOrgId) không match — kiểm tra ownerOrganizationId.")
		// Lấy 1 insight mẫu và 1 meta_ad cùng adId, so sánh ownerOrg
		for id := range insightAdIds {
			var insDoc, adDoc bson.M
			_ = insightsColl.FindOne(ctx, bson.M{"objectType": "ad", "objectId": id}).Decode(&insDoc)
			_ = adsColl.FindOne(ctx, bson.M{"adId": id}).Decode(&adDoc)
			if insDoc != nil && adDoc != nil {
				insOrg := insDoc["ownerOrganizationId"]
				adOrg := adDoc["ownerOrganizationId"]
				insAcc := insDoc["adAccountId"]
				adAcc := adDoc["adAccountId"]
				fmt.Printf("   Mẫu adId=%q: insight(ownerOrg=%v, adAcc=%v) vs meta_ads(ownerOrg=%v, adAcc=%v)\n", id, insOrg, insAcc, adOrg, adAcc)
				break
			}
		}
	}

	fmt.Println("\n=== GỢI Ý KHẮC PHỤC ===")
	fmt.Println("• Nếu insights thiếu objectType/objectId/adAccountId/ownerOrganizationId: Kiểm tra nguồn sync (API, n8n) — phải gửi đủ field.")
	fmt.Println("• Nếu insights được insert TRỰC TIẾP vào MongoDB (không qua API): Hook KHÔNG chạy — chỉ BaseServiceMongoImpl.Upsert/Insert mới emit event.")
	fmt.Println("• Nếu meta_ads không có ad tương ứng: Cần sync meta_ads trước khi sync insights (objectId=ad phải match adId trong meta_ads).")
	fmt.Println("• Nếu đã có đủ insight + meta_ads nhưng currentMetrics vẫn trống: Gọi POST /meta/ad/recalculate-all để tính lại toàn bộ.")
	fmt.Println("• Debounce 3s: Nếu add nhiều insight cùng lúc, chỉ có 1 lần recompute sau 3s.")
}
