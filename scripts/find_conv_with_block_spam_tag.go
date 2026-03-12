// Script tìm conversation có tag Block hoặc Spam — debug.
// Chạy: go run scripts/find_conv_with_block_spam_tag.go [ownerOrganizationId]
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func loadEnv() {
	for _, p := range []string{".env", "api/.env", "api/config/env/development.env"} {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			break
		}
	}
}

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if dbName == "" {
		dbName = os.Getenv("MONGODB_DBNAME")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH")
	}

	orgID := "69a655f0088600c32e62f955"
	if len(os.Args) >= 2 {
		orgID = os.Args[1]
	}
	oid, _ := primitive.ObjectIDFromHex(orgID)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, _ := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	convColl := db.Collection("fb_conversations")

	// Query: panCakeData.tags có phần tử với text = Block hoặc Spam (không phân biệt hoa thường)
	// MongoDB: $elemMatch hoặc regex
	filter := bson.M{
		"ownerOrganizationId": oid,
		"$or": []bson.M{
			{"panCakeData.tags.text": bson.M{"$regex": "block", "$options": "i"}},
			{"panCakeData.tags.text": bson.M{"$regex": "spam", "$options": "i"}},
			{"panCakeData.tags.text": "Block"},
			{"panCakeData.tags.text": "Spam"},
			{"panCakeData.tags.text": "Chặn"},
		},
	}

	count, _ := convColl.CountDocuments(ctx, filter)
	fmt.Printf("Số conv có tag Block/Spam/Chặn (regex): %d\n", count)

	cur, _ := convColl.Find(ctx, filter, options.Find().SetLimit(10).SetProjection(bson.M{
		"conversationId": 1, "customerId": 1, "panCakeData.tags": 1,
	}))
	var docs []bson.M
	cur.All(ctx, &docs)
	cur.Close(ctx)

	for i, doc := range docs {
		pc, _ := doc["panCakeData"].(map[string]interface{})
		tags := pc["tags"]
		fmt.Printf("\n[%d] conversationId=%v customerId=%v\n", i+1, doc["conversationId"], doc["customerId"])
		fmt.Printf("    panCakeData.tags: %v\n", tags)
	}

	// Liệt kê tất cả giá trị text khác nhau trong tags (để xem có Block/Spam không)
	fmt.Println("\n--- Các giá trị text trong tags (sample 1000 conv) ---")
	cur, _ = convColl.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"ownerOrganizationId": oid, "panCakeData.tags": bson.M{"$exists": true, "$ne": nil}}},
		{"$unwind": "$panCakeData.tags"},
		{"$match": bson.M{"panCakeData.tags": bson.M{"$ne": nil}}},
		{"$group": bson.M{"_id": "$panCakeData.tags.text", "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 50},
	})
	var tagCounts []bson.M
	cur.All(ctx, &tagCounts)
	cur.Close(ctx)
	for _, t := range tagCounts {
		text := t["_id"]
		if text == nil {
			text = "(null)"
		}
		lower := strings.ToLower(fmt.Sprintf("%v", text))
		mark := ""
		if strings.Contains(lower, "block") || strings.Contains(lower, "spam") || strings.Contains(lower, "chặn") {
			mark = " <-- Block/Spam"
		}
		fmt.Printf("  %v: %v%v\n", text, t["count"], mark)
	}

	fmt.Println("\n✓ Hoàn thành")
}
