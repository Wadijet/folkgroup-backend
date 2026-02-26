// Script kiểm tra dữ liệu snapshot trong crm_activities và crm_customers.
// Chạy: go run scripts/verify_snapshot_data.go
// Cần: MONGODB_CONNECTION_URI, MONGODB_DBNAME_AUTH (hoặc MONGODB_ConnectionURI)
package main

import (
	"context"
	"encoding/json"
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
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH (hoặc MONGODB_ConnectionURI)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Ping MongoDB lỗi: %v", err)
	}
	fmt.Println("✓ Đã kết nối MongoDB\n")

	db := client.Database(dbName)

	// 1. Thống kê crm_customers
	var crmCount int64
	crmCount, _ = db.Collection("crm_customers").CountDocuments(ctx, bson.M{})
	fmt.Printf("=== crm_customers ===\n")
	fmt.Printf("Tổng số bản ghi: %d\n\n", crmCount)

	// 2. Thống kê crm_activity_history
	actColl := "crm_activity_history"
	var actCount int64
	actCount, _ = db.Collection(actColl).CountDocuments(ctx, bson.M{})
	fmt.Printf("=== crm_activity_history ===\n")
	fmt.Printf("Tổng số bản ghi: %d\n", actCount)

	// Số activity có metadata.snapshotChanges (không rỗng)
	var withSnapshot int64
	withSnapshot, _ = db.Collection(actColl).CountDocuments(ctx, bson.M{
		"metadata.snapshotChanges": bson.M{"$exists": true, "$ne": []interface{}{}},
	})
	fmt.Printf("Số bản ghi có metadata.snapshotChanges (không rỗng): %d\n\n", withSnapshot)

	// 3. Lấy mẫu activity có snapshotChanges
	fmt.Println("=== Mẫu crm_activity_history có metadata.snapshotChanges ===\n")
	cursor, err := db.Collection(actColl).Find(ctx, bson.M{
		"metadata.snapshotChanges": bson.M{"$exists": true, "$ne": []interface{}{}},
	}, options.Find().SetLimit(3).SetSort(bson.M{"metadata.snapshotAt": -1}))
	if err != nil {
		log.Printf("Lỗi query: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var samples []bson.M
	if err := cursor.All(ctx, &samples); err != nil {
		log.Printf("Lỗi decode: %v", err)
		return
	}

	for i, doc := range samples {
		fmt.Printf("--- Activity #%d ---\n", i+1)
		fmt.Printf("  activityType: %v\n", doc["activityType"])
		fmt.Printf("  unifiedId: %v\n", doc["unifiedId"])
		if meta, ok := doc["metadata"].(bson.M); ok {
			fmt.Printf("  metadata.snapshotAt: %v\n", meta["snapshotAt"])
			if changes, ok := meta["snapshotChanges"].(bson.A); ok {
				fmt.Printf("  snapshotChanges (%d mục):\n", len(changes))
				for j, c := range changes {
					if j >= 5 {
						fmt.Printf("    ... và %d mục khác\n", len(changes)-5)
						break
					}
					if cm, ok := c.(bson.M); ok {
						b, _ := json.MarshalIndent(cm, "    ", "  ")
						fmt.Printf("    [%d] %s\n", j+1, string(b))
					}
				}
			}
		}
		fmt.Println()
	}

	// 4. Kiểm tra report_snapshots (nếu có)
	var snapCount int64
	snapCount, _ = db.Collection("report_snapshots").CountDocuments(ctx, bson.M{})
	fmt.Printf("=== report_snapshots ===\n")
	fmt.Printf("Tổng số bản ghi: %d\n\n", snapCount)

	fmt.Println("✓ Đã kiểm tra xong")
}
