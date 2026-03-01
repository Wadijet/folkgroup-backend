// Script kiểm tra chất lượng dữ liệu crm_activity_history — phát hiện bất thường.
// Chạy: go run scripts/check_activity_data_quality.go [ownerOrgId]
// Nếu không truyền ownerOrgId thì kiểm tra toàn bộ.
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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	msThreshold  = int64(1e12) // activityAt < 1e12 có thể là Unix seconds (sai đơn vị)
	year2020Ms   = 1577836800000
	year2030Ms   = 1893456000000
	maxFutureMs  = 24 * 3600 * 1000 // Cho phép activityAt > now tối đa 1 ngày (đồng bộ lệch)
)

func loadEnv() {
	tryPaths := []string{".env", "api/.env", "config/env/development.env", "api/config/env/development.env"}
	cwd, _ := os.Getwd()
	for _, p := range tryPaths {
		full := filepath.Join(cwd, p)
		if _, err := os.Stat(full); err == nil {
			_ = godotenv.Load(full)
			break
		}
		parent := filepath.Dir(cwd)
		if _, err := os.Stat(filepath.Join(parent, p)); err == nil {
			_ = godotenv.Load(filepath.Join(parent, p))
			break
		}
	}
}

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	coll := client.Database(dbName).Collection("crm_activity_history")

	// Filter theo org nếu có
	filter := bson.M{}
	if len(os.Args) > 1 {
		orgHex := os.Args[1]
		orgID, err := primitive.ObjectIDFromHex(orgHex)
		if err != nil {
			log.Fatalf("ownerOrgId không hợp lệ: %s", orgHex)
		}
		filter["ownerOrganizationId"] = orgID
		fmt.Printf("🔍 Lọc theo org: %s\n\n", orgHex)
	}

	total, _ := coll.CountDocuments(ctx, filter)
	fmt.Printf("📊 Tổng activity: %d\n\n", total)
	if total == 0 {
		return
	}

	nowMs := time.Now().UnixMilli()

	// 1. activityAt = 0 hoặc null
	zeroAt, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{"$or": []bson.M{{"activityAt": 0}, {"activityAt": bson.M{"$exists": false}}}}))
	fmt.Printf("⚠️ activityAt = 0 hoặc thiếu: %d\n", zeroAt)

	// 2. activityAt có thể là seconds (sai đơn vị) — giá trị 1e9–1e11
	likelySeconds, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{
		"activityAt": bson.M{"$gt": 1e9, "$lt": msThreshold},
	}))
	fmt.Printf("⚠️ activityAt trong khoảng 1e9–1e11 (có thể là seconds, sai đơn vị): %d\n", likelySeconds)

	// 3. activityAt quá xa quá khứ (trước 2020)
	tooOld, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{"activityAt": bson.M{"$lt": year2020Ms}}))
	fmt.Printf("⚠️ activityAt trước 2020: %d\n", tooOld)

	// 4. activityAt trong tương lai xa (> now + 1 ngày)
	tooFuture, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{"activityAt": bson.M{"$gt": nowMs + maxFutureMs}}))
	fmt.Printf("⚠️ activityAt trong tương lai xa (> now+1d): %d\n", tooFuture)

	// 5. unifiedId rỗng
	emptyUnified, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{"$or": []bson.M{
		{"unifiedId": ""},
		{"unifiedId": bson.M{"$exists": false}},
	}}))
	fmt.Printf("⚠️ unifiedId rỗng: %d\n", emptyUnified)

	// 6. Thiếu metricsSnapshot (bị bỏ qua trong report)
	noMetrics, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{
		"$or": []bson.M{
			{"metadata.metricsSnapshot": bson.M{"$exists": false}},
			{"metadata.metricsSnapshot": nil},
		},
	}))
	withMetrics, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{"metadata.metricsSnapshot": bson.M{"$exists": true, "$ne": nil}}))
	fmt.Printf("📋 Có metadata.metricsSnapshot: %d | Thiếu: %d (bị bỏ qua trong report)\n", withMetrics, noMetrics)

	// 7. Thống kê theo domain
	fmt.Println("\n📈 Thống kê theo domain:")
	pipe := []bson.M{
		{"$match": filter},
		{"$group": bson.M{"_id": "$domain", "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"count": -1}},
	}
	cursor, err := coll.Aggregate(ctx, pipe)
	if err != nil {
		log.Printf("Aggregate domain lỗi: %v", err)
	} else {
		for cursor.Next(ctx) {
			var doc struct {
				ID    string `bson:"_id"`
				Count int64  `bson:"count"`
			}
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			domain := doc.ID
			if domain == "" {
				domain = "(empty)"
			}
			// Đếm có metricsSnapshot trong domain này
			domainFilter := mergeFilter(filter, bson.M{"domain": doc.ID, "metadata.metricsSnapshot": bson.M{"$exists": true, "$ne": nil}})
			withM, _ := coll.CountDocuments(ctx, domainFilter)
			fmt.Printf("  - %s: %d (có metricsSnapshot: %d)\n", domain, doc.Count, withM)
		}
		cursor.Close(ctx)
	}

	// 8. Mẫu activity có activityAt bất thường
	if likelySeconds > 0 || zeroAt > 0 {
		fmt.Println("\n📝 Mẫu activity có activityAt bất thường:")
		var orConditions []bson.M
		if zeroAt > 0 {
			orConditions = append(orConditions, bson.M{"activityAt": 0}, bson.M{"activityAt": bson.M{"$exists": false}})
		}
		if likelySeconds > 0 {
			orConditions = append(orConditions, bson.M{"activityAt": bson.M{"$gt": 1e9, "$lt": msThreshold}})
		}
		sampleFilter := bson.M{"$or": orConditions}
		opts := options.Find().SetLimit(5)
		cursor2, _ := coll.Find(ctx, mergeFilter(filter, sampleFilter), opts)
		for cursor2.Next(ctx) {
			var doc struct {
				ID          primitive.ObjectID `bson:"_id"`
				UnifiedId   string              `bson:"unifiedId"`
				Domain      string              `bson:"domain"`
				ActivityAt  int64               `bson:"activityAt"`
				CreatedAt   int64               `bson:"createdAt"`
				ActivityType string             `bson:"activityType"`
			}
			if err := cursor2.Decode(&doc); err != nil {
				continue
			}
			atStr := "N/A"
			if doc.ActivityAt > 0 {
				if doc.ActivityAt < msThreshold {
					atStr = fmt.Sprintf("%d (có thể seconds)", doc.ActivityAt)
				} else {
					atStr = time.UnixMilli(doc.ActivityAt).Format("2006-01-02 15:04:05")
				}
			}
			fmt.Printf("  _id=%s | unifiedId=%s | domain=%s | activityAt=%s | type=%s\n",
				doc.ID.Hex(), doc.UnifiedId, doc.Domain, atStr, doc.ActivityType)
		}
		cursor2.Close(ctx)
	}

	// 9. Phân bố activityAt theo đơn vị (seconds vs ms)
	fmt.Println("\n📐 Phân bố activityAt (kiểm tra đơn vị):")
	pipe2 := []bson.M{
		{"$match": filter},
		{"$bucket": bson.M{
			"groupBy": "$activityAt",
			"boundaries": []interface{}{int64(0), int64(1), int64(1e9), int64(1e10), int64(1e11), int64(1e12), int64(1e13)},
			"default": "other",
			"output": bson.M{"count": bson.M{"$sum": 1}},
		}},
	}
	cursor3, err3 := coll.Aggregate(ctx, pipe2)
	if err3 != nil {
		log.Printf("Aggregate bucket lỗi: %v", err3)
	} else {
	for cursor3.Next(ctx) {
		var doc struct {
			ID    interface{} `bson:"_id"`
			Count int64       `bson:"count"`
		}
		if err := cursor3.Decode(&doc); err != nil {
			continue
		}
		label := fmt.Sprintf("%v", doc.ID)
		if doc.ID == "other" {
			label = "other (ngoài khoảng)"
		}
		fmt.Printf("  %s: %d\n", label, doc.Count)
	}
	cursor3.Close(ctx)
	}

	// 10. Activity trong 1–2 ngày gần đây (kiểm tra activityAt có được lấy đúng không)
	fmt.Println("\n📅 Activity trong 1–2 ngày gần đây:")
	oneDayAgo := nowMs - 24*3600*1000
	twoDaysAgo := nowMs - 2*24*3600*1000
	count1d, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{"activityAt": bson.M{"$gte": oneDayAgo, "$lte": nowMs}}))
	count2d, _ := coll.CountDocuments(ctx, mergeFilter(filter, bson.M{"activityAt": bson.M{"$gte": twoDaysAgo, "$lte": nowMs}}))
	fmt.Printf("  Trong 1 ngày qua (activityAt): %d\n", count1d)
	fmt.Printf("  Trong 2 ngày qua (activityAt): %d\n", count2d)
	if count2d > 0 {
		fmt.Println("\n  Mẫu 10 activity gần nhất (so sánh activityAt vs createdAt):")
		opts := options.Find().SetLimit(10).SetSort(bson.D{{Key: "activityAt", Value: -1}})
		cursorSample, _ := coll.Find(ctx, mergeFilter(filter, bson.M{"activityAt": bson.M{"$gte": twoDaysAgo}}), opts)
		for cursorSample.Next(ctx) {
			var doc struct {
				UnifiedId   string `bson:"unifiedId"`
				Domain      string `bson:"domain"`
				ActivityAt  int64  `bson:"activityAt"`
				CreatedAt   int64  `bson:"createdAt"`
				ActivityType string `bson:"activityType"`
			}
			if err := cursorSample.Decode(&doc); err != nil {
				continue
			}
			atStr := time.UnixMilli(doc.ActivityAt).Format("2006-01-02 15:04:05")
			ctStr := time.UnixMilli(doc.CreatedAt).Format("2006-01-02 15:04:05")
			diff := (doc.CreatedAt - doc.ActivityAt) / 1000 / 3600 // giờ
			fmt.Printf("    activityAt=%s | createdAt=%s | diff=%dh | %s/%s\n", atStr, ctStr, diff, doc.Domain, doc.ActivityType)
		}
		cursorSample.Close(ctx)
	}

	// 11. Khách có nhiều activity trong 1 tuần (có thể gây số nhảy vọt nếu logic cũ đếm từng lần)
	fmt.Println("\n📊 Khách có nhiều activity trong 1 tuần (top 5):")
	weekAgo := nowMs - 7*24*3600*1000
	pipe4 := []bson.M{
		{"$match": mergeFilter(filter, bson.M{
			"activityAt":            bson.M{"$gte": weekAgo, "$lte": nowMs},
			"metadata.metricsSnapshot": bson.M{"$exists": true},
		})},
		{"$group": bson.M{"_id": "$unifiedId", "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 5},
	}
	cursor4, err4 := coll.Aggregate(ctx, pipe4)
	if err4 != nil {
		log.Printf("Aggregate per-customer lỗi: %v", err4)
	} else {
		for cursor4.Next(ctx) {
			var doc struct {
				ID    string `bson:"_id"`
				Count int64  `bson:"count"`
			}
			if err := cursor4.Decode(&doc); err != nil {
				continue
			}
			fmt.Printf("  unifiedId=%s: %d activities\n", doc.ID, doc.Count)
		}
		cursor4.Close(ctx)
	}

	fmt.Println("\n✓ Kiểm tra xong")
}

func mergeFilter(base, extra bson.M) bson.M {
	if len(base) == 0 {
		return extra
	}
	return bson.M{"$and": []bson.M{base, extra}}
}
