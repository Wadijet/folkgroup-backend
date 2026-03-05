// Script kiểm tra số lượng document trong các collection Meta Ads.
// Chạy: go run scripts/check_meta_db.go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	godotenv.Load("api/config/env/development.env")
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)
	db := client.Database(dbName)
	cols := []string{"meta_ad_accounts", "meta_campaigns", "meta_adsets", "meta_ads", "meta_ad_insights"}
	log.Println("=== Kiểm tra Meta Ads collections ===")
	for _, c := range cols {
		n, err := db.Collection(c).CountDocuments(ctx, bson.M{})
		if err != nil {
			log.Printf("  %s: LỖI %v", c, err)
			continue
		}
		log.Printf("  %s: %d documents", c, n)
	}
}
