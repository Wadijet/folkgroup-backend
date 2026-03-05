// Script kiểm tra meta_ad_insights: metaData có đủ frequency, actions, inline_link_clicks, cpm, ctr, cpc chưa.
// Chạy: go run scripts/check_meta_insights_fields.go
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	insights := db.Collection("meta_ad_insights")

	count, _ := insights.CountDocuments(ctx, bson.M{})
	fmt.Println("=== KIỂM TRA meta_ad_insights — CÁC FIELD CẦN CHO ADS INTELLIGENCE ===\n")
	fmt.Printf("Tổng số documents: %d\n\n", count)

	if count == 0 {
		fmt.Println("⚠️ Collection TRỐNG — chưa sync insights từ Meta API.")
		return
	}

	// Lấy 3 mẫu mới nhất (mỗi objectType khác nhau nếu có)
	cursor, err := insights.Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"updatedAt": -1}).SetLimit(5))
	if err != nil {
		log.Fatalf("Find insights: %v", err)
	}
	defer cursor.Close(ctx)

	fieldsNeeded := []string{"frequency", "inline_link_clicks", "actions", "cpm", "ctr", "cpc", "spend", "impressions", "clicks", "reach"}
	stats := make(map[string]int) // số doc có field
	for _, f := range fieldsNeeded {
		stats[f] = 0
	}

	sampleNum := 0
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		sampleNum++
		meta, _ := doc["metaData"].(bson.M)
		if meta == nil {
			fmt.Printf("--- Mẫu #%d: metaData = nil ---\n", sampleNum)
			continue
		}

		fmt.Printf("--- Mẫu #%d (objectType=%v objectId=%v dateStart=%v) ---\n",
			sampleNum, doc["objectType"], doc["objectId"], doc["dateStart"])

		for _, key := range fieldsNeeded {
			v := meta[key]
			has := v != nil
			if has {
				stats[key]++
			}
			// Hiển thị giá trị (rút gọn cho actions vì có thể dài)
			if key == "actions" {
				if arr, ok := v.(bson.A); ok {
					fmt.Printf("  %s: %v (array %d phần tử)\n", key, has, len(arr))
					// In 2 phần tử đầu nếu có
					for i := 0; i < len(arr) && i < 2; i++ {
						if m, ok := arr[i].(bson.M); ok {
							at, _ := m["action_type"].(string)
							val, _ := m["value"].(string)
							fmt.Printf("      [%d] action_type=%q value=%q\n", i, at, val)
						}
					}
				} else {
					fmt.Printf("  %s: %v (kiểu %T)\n", key, has, v)
				}
			} else {
				fmt.Printf("  %s: %v (giá trị: %v)\n", key, has, v)
			}
		}
		fmt.Println()
	}

	// Thống kê tổng: đếm % doc có từng field
	fmt.Println("=== THỐNG KÊ SỐ DOC CÓ FIELD (trong 5 mẫu mới nhất) ===")
	for _, f := range fieldsNeeded {
		pct := 0
		if sampleNum > 0 {
			pct = stats[f] * 100 / sampleNum
		}
		icon := "❌"
		if stats[f] > 0 {
			icon = "✅"
		}
		fmt.Printf("  %s %s: %d/%d (%d%%)\n", icon, f, stats[f], sampleNum, pct)
	}

	// Kiểm tra tổng thể: aggregate count docs có metaData.frequency
	fmt.Println("\n=== KIỂM TRA TỔNG THỂ (toàn bộ collection) ===")
	for _, key := range []string{"frequency", "inline_link_clicks", "actions", "cpm", "ctr", "cpc"} {
		n, _ := insights.CountDocuments(ctx, bson.M{"metaData." + key: bson.M{"$exists": true, "$ne": nil}})
		pct := 0
		if count > 0 {
			pct = int(n * 100 / count)
		}
		icon := "❌"
		if n > 0 {
			icon = "✅"
		}
		fmt.Printf("  %s metaData.%s: %d/%d docs (%d%%)\n", icon, key, n, count, pct)
	}

	// Kiểm tra có action_type chứa messaging_conversation_started không (cho Mess)
	cursor2, _ := insights.Find(ctx, bson.M{"metaData.actions": bson.M{"$exists": true}}, options.Find().SetLimit(50))
	messCount := 0
	for cursor2.Next(ctx) {
		var d bson.M
		if err := cursor2.Decode(&d); err != nil {
			continue
		}
		meta, _ := d["metaData"].(bson.M)
		arr, _ := meta["actions"].(bson.A)
		for _, item := range arr {
			if m, ok := item.(bson.M); ok {
				at, _ := m["action_type"].(string)
				if strings.Contains(at, "messaging_conversation_started") {
					messCount++
					break
				}
			}
		}
	}
	cursor2.Close(ctx)
	fmt.Printf("\n  📩 Số doc có action messaging_conversation_started* (trong 50 mẫu có actions): %d\n", messCount)
}
