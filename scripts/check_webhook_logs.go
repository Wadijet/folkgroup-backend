// Script kiểm tra webhook_logs — có Pancake webhook nào vào không.
// Chạy: go run scripts/check_webhook_logs.go
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
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	if uri == "" {
		uri = os.Getenv("MongoDB_ConnectionURI")
	}
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if dbName == "" {
		dbName = os.Getenv("MONGODB_DBNAME")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	coll := client.Database(dbName).Collection("webhook_logs")

	total, _ := coll.CountDocuments(ctx, bson.M{})
	fmt.Printf("=== webhook_logs — Tổng: %d ===\n\n", total)

	if total == 0 {
		fmt.Println("Không có webhook log nào.")
		return
	}

	// Phân bố theo source
	fmt.Println("--- Phân bố theo source ---")
	pipe := []bson.M{
		{"$group": bson.M{"_id": "$source", "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"count": -1}},
	}
	cur, _ := coll.Aggregate(ctx, pipe)
	for cur.Next(ctx) {
		var d struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if cur.Decode(&d) == nil {
			fmt.Printf("  %s: %d\n", d.ID, d.Count)
		}
	}
	cur.Close(ctx)

	// Phân bố theo eventType (pancake)
	fmt.Println("\n--- Phân bố theo eventType (source=pancake) ---")
	pipe = []bson.M{
		{"$match": bson.M{"source": "pancake"}},
		{"$group": bson.M{"_id": "$eventType", "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"count": -1}},
	}
	cur, _ = coll.Aggregate(ctx, pipe)
	for cur.Next(ctx) {
		var d struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if cur.Decode(&d) == nil {
			fmt.Printf("  %s: %d\n", d.ID, d.Count)
		}
	}
	cur.Close(ctx)

	// 5 webhook mới nhất
	fmt.Println("\n--- 5 webhook mới nhất ---")
	cur, _ = coll.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "receivedAt", Value: -1}}).SetLimit(5).SetProjection(bson.M{
		"source": 1, "eventType": 1, "pageId": 1, "shopId": 1, "processed": 1, "processError": 1,
		"receivedAt": 1, "createdAt": 1,
	}))
	for cur.Next(ctx) {
		var d struct {
			Source       string `bson:"source"`
			EventType    string `bson:"eventType"`
			PageID       string `bson:"pageId"`
			ShopID       int64  `bson:"shopId"`
			Processed    bool   `bson:"processed"`
			ProcessError string `bson:"processError"`
			ReceivedAt   int64  `bson:"receivedAt"`
		}
		if cur.Decode(&d) == nil {
			ts := time.UnixMilli(d.ReceivedAt).Format("2006-01-02 15:04:05")
			errStr := ""
			if d.ProcessError != "" {
				errStr = " | err=" + d.ProcessError
			}
			fmt.Printf("  %s | %s | pageId=%s | %s | processed=%v%s\n",
				d.Source, d.EventType, d.PageID, ts, d.Processed, errStr)
		}
	}
	cur.Close(ctx)

	// Pancake webhook có processed=false (lỗi)
	pancakeFailed, _ := coll.CountDocuments(ctx, bson.M{"source": "pancake", "processed": false})
	hasErr, _ := coll.CountDocuments(ctx, bson.M{"source": "pancake", "processError": bson.M{"$ne": "", "$exists": true}})
	fmt.Printf("\n--- Pancake webhook lỗi: processed=false=%d | processError!=''=%d ---\n", pancakeFailed, hasErr)

	fmt.Println("\n✓ Hoàn thành")
}
