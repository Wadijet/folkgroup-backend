// Script kiểm tra định dạng customerId trong fb_customers vs fb_conversations.
// Chạy: go run scripts/check_customerid_format.go
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	// Mẫu fb_customers.customerId
	fmt.Println("=== FB_CUSTOMERS - mẫu customerId ===\n")
	var fbSamples []struct {
		CustomerId string `bson:"customerId"`
		PageId     string `bson:"pageId"`
	}
	cursor, _ := db.Collection("fb_customers").Find(ctx, bson.M{}, options.Find().SetLimit(5).SetProjection(bson.M{"customerId": 1, "pageId": 1}))
	cursor.All(ctx, &fbSamples)
	cursor.Close(ctx)
	for i, s := range fbSamples {
		fmt.Printf("  #%d customerId=%q pageId=%q\n", i+1, s.CustomerId, s.PageId)
	}

	// Mẫu fb_conversations.customerId
	fmt.Println("\n=== FB_CONVERSATIONS - mẫu customerId ===\n")
	var convSamples []struct {
		CustomerId string `bson:"customerId"`
		PageId     string `bson:"pageId"`
	}
	cursor2, _ := db.Collection("fb_conversations").Find(ctx, bson.M{}, options.Find().SetLimit(5).SetProjection(bson.M{"customerId": 1, "pageId": 1}))
	cursor2.All(ctx, &convSamples)
	cursor2.Close(ctx)
	for i, s := range convSamples {
		fmt.Printf("  #%d customerId=%q pageId=%q\n", i+1, s.CustomerId, s.PageId)
	}

	// Giao nhau: customerId có trong cả fb_customers và fb_conversations?
	fmt.Println("\n=== GIAO NHAU (cùng pageId) ===\n")
	// Lấy 1 pageId từ conversation
	var conv struct {
		CustomerId string `bson:"customerId"`
		PageId     string `bson:"pageId"`
	}
	db.Collection("fb_conversations").FindOne(ctx, bson.M{"customerId": bson.M{"$ne": ""}}, options.FindOne().SetProjection(bson.M{"customerId": 1, "pageId": 1})).Decode(&conv)
	if conv.PageId != "" {
		fmt.Printf("PageId mẫu: %s\n", conv.PageId)
		// fb_customers cùng page
		var fb struct {
			CustomerId string `bson:"customerId"`
		}
		err := db.Collection("fb_customers").FindOne(ctx, bson.M{"pageId": conv.PageId}, options.FindOne().SetProjection(bson.M{"customerId": 1})).Decode(&fb)
		if err == nil {
			fmt.Printf("fb_customers.customerId (cùng page): %q\n", fb.CustomerId)
		} else {
			fmt.Printf("Không có fb_customer nào với pageId=%s\n", conv.PageId)
		}
		// Conversation customerId
		fmt.Printf("fb_conversations.customerId: %q\n", conv.CustomerId)
		// Match?
		match, _ := db.Collection("fb_customers").CountDocuments(ctx, bson.M{"customerId": conv.CustomerId})
		fmt.Printf("Số fb_customers có customerId=%q: %d\n", conv.CustomerId, match)
	}

	// Kiểm tra thêm: trong 500 conv, có bao nhiêu customerId tồn tại trong fb_customers?
	fmt.Println("\n=== TỶ LỆ MATCH (500 conv) ===\n")
	cursor3, _ := db.Collection("fb_conversations").Find(ctx, bson.M{"customerId": bson.M{"$ne": ""}}, options.Find().SetLimit(500).SetProjection(bson.M{"customerId": 1}))
	seen := make(map[string]bool)
	matched := 0
	total := 0
	for cursor3.Next(ctx) {
		var d struct {
			CustomerId string `bson:"customerId"`
		}
		if cursor3.Decode(&d) != nil || d.CustomerId == "" || seen[d.CustomerId] {
			continue
		}
		seen[d.CustomerId] = true
		total++
		n, _ := db.Collection("fb_customers").CountDocuments(ctx, bson.M{"customerId": d.CustomerId})
		if n > 0 {
			matched++
		}
	}
	cursor3.Close(ctx)
	fmt.Printf("Trong %d customerId duy nhất: %d có trong fb_customers (%.1f%%)\n", total, matched, float64(matched)/float64(total)*100)
}
