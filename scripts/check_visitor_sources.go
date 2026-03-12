// Script kiểm tra nguồn của khách visitor (journeyStage=visitor).
// Phân tích: primarySource, có conversation trong fb_conversations không?
//
// Chạy: go run scripts/check_visitor_sources.go [ownerOrganizationId]
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
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

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func buildConvFilter(ids []string, numIds []interface{}) bson.M {
	convOr := []bson.M{
		{"customerId": bson.M{"$in": ids}},
		{"panCakeData.customer_id": bson.M{"$in": ids}},
		{"panCakeData.customer.id": bson.M{"$in": ids}},
		{"panCakeData.customers.id": bson.M{"$in": ids}},
		{"panCakeData.page_customer.id": bson.M{"$in": ids}},
		{"panCakeData.page_customer.customer_id": bson.M{"$in": ids}},
	}
	if len(numIds) > 0 {
		convOr = append(convOr,
			bson.M{"panCakeData.customer_id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customers.id": bson.M{"$in": numIds}},
		)
	}
	return bson.M{"$or": convOr}
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

	var orgID primitive.ObjectID
	if len(os.Args) >= 2 {
		var err error
		orgID, err = primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
		fmt.Printf("=== Phân tích visitor cho org: %s ===\n\n", os.Args[1])
	} else {
		log.Fatal("Chạy: go run scripts/check_visitor_sources.go <ownerOrganizationId>")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	convColl := db.Collection("fb_conversations")

	filter := bson.M{
		"ownerOrganizationId": orgID,
		"journeyStage":        "visitor",
	}
	totalVisitor, _ := crmColl.CountDocuments(ctx, filter)
	fmt.Printf("Tổng visitor: %d\n\n", totalVisitor)
	if totalVisitor == 0 {
		return
	}

	// 1. Phân bố primarySource
	fmt.Println("--- Phân bố primarySource ---")
	pipe := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": bson.M{"$ifNull": []interface{}{"$primarySource", "(rỗng)"}}, "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
	}
	cur, _ := crmColl.Aggregate(ctx, pipe)
	var srcResults []bson.M
	cur.All(ctx, &srcResults)
	cur.Close(ctx)
	for _, r := range srcResults {
		fmt.Printf("  %v: %v\n", r["_id"], r["count"])
	}

	// 2. Chỉ kiểm tra visitor primarySource=fb (vì user nói khách từ FB chat nên phải có conv)
	fbFilter := bson.M{"ownerOrganizationId": orgID, "journeyStage": "visitor", "primarySource": "fb"}
	fbVisitorCount, _ := crmColl.CountDocuments(ctx, fbFilter)
	fmt.Printf("\nVisitor primarySource=fb: %d (đáng lẽ tất cả phải có conv)\n", fbVisitorCount)

	// 3. Lấy mẫu visitor fb, kiểm tra có conv trong DB không
	fmt.Println("\n--- Kiểm tra mẫu visitor (fb): có conversation trong fb_conversations không? ---")
	opts := options.Find().SetProjection(bson.M{"unifiedId": 1, "sourceIds": 1, "primarySource": 1}).SetLimit(100)
	cur, _ = crmColl.Find(ctx, fbFilter, opts)
	var visitors []bson.M
	cur.All(ctx, &visitors)
	cur.Close(ctx)

	hasConv := 0
	noConv := 0
	var sampleNoConv []string
	for _, doc := range visitors {
		uid := getStr(doc, "unifiedId")
		sourceIds, _ := doc["sourceIds"].(map[string]interface{})
		posId := getStr(sourceIds, "pos")
		fbId := getStr(sourceIds, "fb")

		ids := []string{uid, posId, fbId}
		var cleanIds []string
		var numIds []interface{}
		for _, id := range ids {
			if id != "" {
				cleanIds = append(cleanIds, id)
				if n, err := strconv.ParseInt(id, 10, 64); err == nil {
					numIds = append(numIds, n)
				}
			}
		}
		if len(cleanIds) == 0 {
			cleanIds = []string{uid}
		}

		convOr := buildConvFilter(cleanIds, numIds)["$or"]
		convFilter := bson.M{"ownerOrganizationId": orgID, "$or": convOr}
		n, _ := convColl.CountDocuments(ctx, convFilter)
		if n > 0 {
			hasConv++
		} else {
			noConv++
			if len(sampleNoConv) < 10 {
				sampleNoConv = append(sampleNoConv, uid)
			}
		}
	}

	fmt.Printf("  Trong mẫu %d visitor:\n", len(visitors))
	fmt.Printf("    - Có conv trong DB: %d (MISMATCH: đáng lẽ phải engaged)\n", hasConv)
	fmt.Printf("    - Không có conv: %d\n", noConv)
	if len(sampleNoConv) > 0 {
		fmt.Printf("  Mẫu visitor không có conv: %v\n", sampleNoConv)
	}

	// 3. Nếu có mismatch nhiều → gợi ý recalc
	if hasConv > 0 {
		fmt.Printf("\n⚠️ Có %d visitor thực tế CÓ conversation trong DB — đáng lẽ phải là engaged.\n", hasConv)
		fmt.Println("   Gợi ý: chạy RecalculateMismatchCustomers hoặc recalc visitor để cập nhật hasConversation.")
	}
}
