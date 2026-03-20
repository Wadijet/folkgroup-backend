// Script khám phá dữ liệu Zalo trong database.
// Kiểm tra: collections mới, fb_conversations có Zalo không, fb_customers có khách Zalo không.
//
// Chạy: cd api && go run ../scripts/discover_zalo_data.go
package main

import (
	"context"
	"fmt"
	"log"
	"meta_commerce/config"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(cfg.MongoDB_DBName_Auth)

	fmt.Println("=== KHÁM PHÁ DỮ LIỆU ZALO TRONG DATABASE ===")
	fmt.Printf("Database: %s\n\n", cfg.MongoDB_DBName_Auth)

	// 1. Liệt kê tất cả collections — tìm collection mới có zalo
	fmt.Println("--- 1. COLLECTIONS CÓ TÊN CHỨA 'zalo' ---")
	cols, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.Printf("Lỗi list collections: %v", err)
	} else {
		for _, c := range cols {
			if strings.Contains(strings.ToLower(c), "zalo") {
				n, _ := db.Collection(c).CountDocuments(ctx, bson.M{})
				fmt.Printf("  ✓ %s: %d documents\n", c, n)
			}
		}
	}

	// 2. fb_conversations — tìm field phân biệt nguồn (source, channel, platform, type, ...)
	fmt.Println("\n--- 2. FB_CONVERSATIONS — PHÂN TÍCH panCakeData ---")
	convs := db.Collection("fb_conversations")
	totalConv, _ := convs.CountDocuments(ctx, bson.M{})

	// Các field thường dùng để phân biệt nguồn (Messenger vs Zalo)
	sourceFields := []string{
		"panCakeData.source", "panCakeData.channel", "panCakeData.platform",
		"panCakeData.type", "panCakeData.primary_source", "panCakeData.source_type",
		"panCakeData.conversation_type", "panCakeData.inbox_type",
	}
	for _, f := range sourceFields {
		// Aggregate đếm theo giá trị
		pipe := []bson.M{
			{"$match": bson.M{f: bson.M{"$exists": true, "$nin": []interface{}{nil, ""}}}},
			{"$group": bson.M{"_id": "$" + strings.ReplaceAll(f, "panCakeData.", "panCakeData."), "count": bson.M{"$sum": 1}}},
			{"$sort": bson.M{"count": -1}},
			{"$limit": 20},
		}
		cur, err := convs.Aggregate(ctx, pipe)
		if err != nil {
			continue
		}
		var results []struct {
			ID    interface{} `bson:"_id"`
			Count int64       `bson:"count"`
		}
		_ = cur.All(ctx, &results)
		cur.Close(ctx)
		if len(results) > 0 {
			fmt.Printf("  %s:\n", f)
			for _, r := range results {
				val := fmt.Sprintf("%v", r.ID)
				if strings.Contains(strings.ToLower(val), "zalo") {
					fmt.Printf("    *** ZALO: %v = %d ***\n", r.ID, r.Count)
				} else {
					fmt.Printf("    %v = %d\n", r.ID, r.Count)
				}
			}
		}
	}

	// 2b. Phân biệt Zalo qua pageId: page Zalo có pageId bắt đầu "pzl_" (personal_zalo)
	fmt.Println("\n--- 2b. FB_CONVERSATIONS — PHÂN LOẠI THEO pageId (Zalo vs Messenger) ---")
	zaloConvCount, _ := convs.CountDocuments(ctx, bson.M{"pageId": bson.M{"$regex": "^pzl_"}})
	messengerConvCount := totalConv - zaloConvCount
	// Fallback: conv có panCakeData chứa zalo
	zaloByField, _ := convs.CountDocuments(ctx, bson.M{
		"$or": []bson.M{
			{"panCakeData.source": bson.M{"$regex": "zalo", "$options": "i"}},
			{"panCakeData.channel": bson.M{"$regex": "zalo", "$options": "i"}},
			{"panCakeData.platform": bson.M{"$regex": "zalo", "$options": "i"}},
		},
	})
	fmt.Printf("  Conv có pageId bắt đầu 'pzl_' (Zalo): %d\n", zaloConvCount)
	fmt.Printf("  Conv có pageId khác (Messenger): %d\n", messengerConvCount)
	fmt.Printf("  Conv có panCakeData.source/channel/platform=zalo: %d\n", zaloByField)

	// Sample 3 documents Zalo (pageId pzl_)
	if zaloConvCount > 0 {
		cur, _ := convs.Find(ctx, bson.M{"pageId": bson.M{"$regex": "^pzl_"}}, options.Find().SetLimit(3).SetProjection(bson.M{
			"conversationId": 1, "customerId": 1, "pageId": 1,
			"panCakeData.type": 1, "panCakeData.customer_id": 1,
		}))
		var samples []bson.M
		_ = cur.All(ctx, &samples)
		cur.Close(ctx)
		fmt.Println("  Mẫu 3 documents:")
		for i, s := range samples {
			fmt.Printf("    [%d] convId=%v customerId=%v pageId=%v panCakeData=%v\n", i+1, s["conversationId"], s["customerId"], s["pageId"], s["panCakeData"])
		}
	}

	// 2c. Lấy tất cả key top-level trong panCakeData (sample 100 docs)
	fmt.Println("\n--- 2c. CÁC KEY TRONG panCakeData (sample 100 conv) ---")
	cur, _ := convs.Find(ctx, bson.M{}, options.Find().SetLimit(100).SetProjection(bson.M{"panCakeData": 1}))
	keySet := make(map[string]bool)
	var docs []bson.M
	_ = cur.All(ctx, &docs)
	cur.Close(ctx)
	for _, d := range docs {
		if pc, ok := d["panCakeData"].(bson.M); ok {
			for k := range pc {
				keySet[k] = true
			}
		}
	}
	fmt.Print("  Keys: ")
	for k := range keySet {
		fmt.Printf("%s ", k)
	}
	fmt.Println()

	// 3. fb_customers — tìm khách Zalo (qua pageId bắt đầu pzl_)
	fmt.Println("\n--- 3. FB_CUSTOMERS — CÓ KHÁCH ZALO KHÔNG? ---")
	fbCust := db.Collection("fb_customers")
	totalFbCust, _ := fbCust.CountDocuments(ctx, bson.M{})

	zaloCustCount, _ := fbCust.CountDocuments(ctx, bson.M{"pageId": bson.M{"$regex": "^pzl_"}})
	zaloCustByField, _ := fbCust.CountDocuments(ctx, bson.M{
		"$or": []bson.M{
			{"panCakeData.source": bson.M{"$regex": "zalo", "$options": "i"}},
			{"panCakeData.channel": bson.M{"$regex": "zalo", "$options": "i"}},
			{"panCakeData.platform": bson.M{"$regex": "zalo", "$options": "i"}},
		},
	})
	fmt.Printf("  fb_customers có pageId bắt đầu 'pzl_' (Zalo): %d / %d\n", zaloCustCount, totalFbCust)
	fmt.Printf("  fb_customers có panCakeData.source/channel/platform=zalo: %d\n", zaloCustByField)

	// Lấy keys trong panCakeData của fb_customers
	fbKeySet := make(map[string]bool)
	fbCur, _ := fbCust.Find(ctx, bson.M{}, options.Find().SetLimit(50).SetProjection(bson.M{"panCakeData": 1}))
	var fbDocs []bson.M
	_ = fbCur.All(ctx, &fbDocs)
	fbCur.Close(ctx)
	for _, d := range fbDocs {
		if pc, ok := d["panCakeData"].(bson.M); ok {
			for k := range pc {
				fbKeySet[k] = true
			}
		}
	}
	fmt.Print("  Keys trong panCakeData (fb_customers): ")
	for k := range fbKeySet {
		fmt.Printf("%s ", k)
	}
	fmt.Println()

	// 4. fb_pages — có page Zalo không?
	fmt.Println("\n--- 4. FB_PAGES — CÓ PAGE ZALO KHÔNG? ---")
	pages := db.Collection("fb_pages")
	zaloPageCount, _ := pages.CountDocuments(ctx, bson.M{
		"$or": []bson.M{
			{"panCakeData.source": bson.M{"$regex": "zalo", "$options": "i"}},
			{"panCakeData.channel": bson.M{"$regex": "zalo", "$options": "i"}},
			{"panCakeData.platform": bson.M{"$regex": "zalo", "$options": "i"}},
		},
	})
	totalPages, _ := pages.CountDocuments(ctx, bson.M{})
	fmt.Printf("  fb_pages có source/channel/platform = zalo: %d / %d\n", zaloPageCount, totalPages)

	// Chi tiết page Zalo + conv thuộc page đó
	if zaloPageCount > 0 {
		zaloPageCur, _ := pages.Find(ctx, bson.M{
			"$or": []bson.M{
				{"panCakeData.source": bson.M{"$regex": "zalo", "$options": "i"}},
				{"panCakeData.channel": bson.M{"$regex": "zalo", "$options": "i"}},
				{"panCakeData.platform": bson.M{"$regex": "zalo", "$options": "i"}},
			},
		}, options.Find().SetProjection(bson.M{"pageId": 1, "panCakeData": 1}))
		var zaloPages []bson.M
		_ = zaloPageCur.All(ctx, &zaloPages)
		zaloPageCur.Close(ctx)
		for i, p := range zaloPages {
			pageId := p["pageId"]
			fmt.Printf("  Page Zalo [%d]: pageId=%v, panCakeData=%v\n", i+1, pageId, p["panCakeData"])
			if pageId != nil && pageId != "" {
				convOfPage, _ := convs.CountDocuments(ctx, bson.M{"pageId": pageId})
				fmt.Printf("    → Số conversations thuộc page này: %d\n", convOfPage)
			}
		}
	}

	// 4b. Collections có tên liên quan zalo/conv/message/customer (có thể mới)
	fmt.Println("\n--- 4b. COLLECTIONS LIÊN QUAN (zalo, conv, message, customer) ---")
	allCols, _ := db.ListCollectionNames(ctx, bson.M{})
	for _, c := range allCols {
		lower := strings.ToLower(c)
		if strings.Contains(lower, "zalo") || strings.Contains(lower, "conv") || strings.Contains(lower, "message") || strings.Contains(lower, "customer") {
			n, _ := db.Collection(c).CountDocuments(ctx, bson.M{})
			fmt.Printf("  %s: %d\n", c, n)
		}
	}

	// 5. Tổng kết
	fmt.Println("\n=== TỔNG KẾT ===")
	if zaloConvCount > 0 || zaloCustCount > 0 || zaloPageCount > 0 {
		fmt.Println("  ✓ Có dữ liệu Zalo trong database!")
		fmt.Printf("    - fb_conversations (Zalo): %d\n", zaloConvCount)
		fmt.Printf("    - fb_customers (Zalo): %d\n", zaloCustCount)
		fmt.Printf("    - fb_pages (Zalo): %d\n", zaloPageCount)
	} else {
		fmt.Println("  ⚠ Chưa tìm thấy dữ liệu Zalo rõ ràng qua các field source/channel/platform/type.")
		fmt.Println("    Pancake có thể dùng tên field khác — kiểm tra 2c để xem keys có trong panCakeData.")
	}
}
