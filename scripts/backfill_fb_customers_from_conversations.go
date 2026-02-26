// Script backfill fb_customers từ panCakeData của fb_conversations.
// Khi conversation có customer/customers trong panCakeData nhưng chưa có trong fb_customers,
// upsert customer để IngestConversationTouchpoint có thể resolve và tạo activity conversation_started.
// Chạy: go run scripts/backfill_fb_customers_from_conversations.go [ownerOrganizationId]
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

func extractCustomerFromPanCakeData(panCakeData map[string]interface{}) (map[string]interface{}, string) {
	if panCakeData == nil {
		return nil, ""
	}
	var customerData map[string]interface{}
	if cust, ok := panCakeData["customer"].(map[string]interface{}); ok && cust != nil {
		customerData = cust
	} else if arr, ok := panCakeData["customers"].([]interface{}); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]interface{}); ok {
			customerData = m
		}
	}
	if customerData == nil {
		return nil, ""
	}
	var customerId string
	if s, ok := customerData["id"].(string); ok && s != "" {
		customerId = s
	} else if n, ok := customerData["id"].(float64); ok {
		customerId = fmt.Sprintf("%.0f", n)
	}
	return customerData, customerId
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

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	convColl := db.Collection("fb_conversations")
	fbCustColl := db.Collection("fb_customers")

	var orgFilter bson.M
	if len(os.Args) > 1 && os.Args[1] != "" {
		orgID, err := primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
		orgFilter = bson.M{"ownerOrganizationId": orgID}
		log.Printf("Lọc theo org: %s\n", os.Args[1])
	}

	filter := bson.M{"customerId": bson.M{"$exists": true, "$ne": ""}}
	if orgFilter != nil {
		filter = bson.M{"$and": []bson.M{orgFilter, filter}}
	}

	cursor, err := convColl.Find(ctx, filter, options.Find().SetProjection(bson.M{"customerId": 1, "pageId": 1, "ownerOrganizationId": 1, "panCakeData": 1}))
	if err != nil {
		log.Fatalf("Find conversations lỗi: %v", err)
	}
	defer cursor.Close(ctx)

	upserted := 0
	skipped := 0
	noCustomerObj := 0
	seen := make(map[string]bool)

	for cursor.Next(ctx) {
		var doc struct {
			CustomerId       string                 `bson:"customerId"`
			PageId          string                 `bson:"pageId"`
			OwnerOrgID      primitive.ObjectID     `bson:"ownerOrganizationId"`
			PanCakeData     map[string]interface{}  `bson:"panCakeData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.CustomerId == "" || doc.OwnerOrgID.IsZero() {
			skipped++
			continue
		}
		key := doc.CustomerId + "|" + doc.OwnerOrgID.Hex()
		if seen[key] {
			continue
		}
		seen[key] = true

		// Kiểm tra đã có trong fb_customers chưa
		exist, _ := fbCustColl.CountDocuments(ctx, bson.M{"customerId": doc.CustomerId, "ownerOrganizationId": doc.OwnerOrgID})
		if exist > 0 {
			skipped++
			continue
		}

		// Lấy customer object từ panCakeData
		customerData, cid := extractCustomerFromPanCakeData(doc.PanCakeData)
		if customerData == nil || cid == "" {
			noCustomerObj++
			// Fallback: tạo minimal customer từ customerId (không có chi tiết)
			customerData = map[string]interface{}{"id": doc.CustomerId}
			cid = doc.CustomerId
		}

		now := time.Now().UnixMilli()
		_, err := fbCustColl.UpdateOne(ctx,
			bson.M{"customerId": cid},
			bson.M{
				"$set": bson.M{
					"panCakeData": customerData, "pageId": doc.PageId,
					"ownerOrganizationId": doc.OwnerOrgID, "updatedAt": now,
				},
				"$setOnInsert": bson.M{"customerId": cid, "createdAt": now},
			},
			options.Update().SetUpsert(true),
		)
		if err != nil {
			log.Printf("Upsert customer %s lỗi: %v", cid, err)
			continue
		}
		upserted++
	}

	log.Printf("Hoàn tất: upsert %d fb_customers, bỏ qua %d (đã có), %d không có customer object trong panCakeData", upserted, skipped, noCustomerObj)
	fmt.Println("Bước tiếp theo: gọi POST /api/v1/customers/backfill-activity với ownerOrganizationId để tạo activity conversation_started.")
}
