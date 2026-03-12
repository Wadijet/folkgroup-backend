// Script kiểm tra 691 visitor: có conversation thực tế không, tại sao bị tính visitor?
//
// Chạy: go run scripts/audit_691_visitors_has_conv.go [ownerOrganizationId]
//
// Phân tích:
// - visitor = orderCount=0 VÀ hasConversation=false
// - Nếu có conv trong fb_conversations nhưng hasConversation=false → mismatch, cần recalc
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
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	convColl := db.Collection("fb_conversations")

	if orgID.IsZero() {
		var doc struct {
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if crmColl.FindOne(ctx, bson.M{"journeyStage": "visitor"}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&doc) == nil {
			orgID = doc.OwnerOrganizationID
		} else {
			log.Fatal("Không tìm thấy org. Chạy với: go run scripts/audit_691_visitors_has_conv.go <ownerOrganizationId>")
		}
	}

	filter := bson.M{"ownerOrganizationId": orgID, "journeyStage": "visitor"}
	total, _ := crmColl.CountDocuments(ctx, filter)
	fmt.Printf("=== Kiểm tra %d visitor: có conversation không? ===\n\n", total)
	fmt.Printf("Org: %s\n\n", orgID.Hex())

	if total == 0 {
		return
	}

	// Lấy tất cả visitor với projection
	opts := options.Find().SetProjection(bson.M{
		"unifiedId": 1, "sourceIds": 1, "primarySource": 1,
		"hasConversation": 1, "conversationCount": 1, "orderCount": 1,
		"currentMetrics": 1,
	})
	cur, err := crmColl.Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("Find: %v", err)
	}

	type visitorDoc struct {
		UnifiedId         string                 `bson:"unifiedId"`
		SourceIds         map[string]interface{} `bson:"sourceIds"`
		PrimarySource     string                 `bson:"primarySource"`
		HasConversation   bool                   `bson:"hasConversation"`
		ConversationCount int                    `bson:"conversationCount"`
		OrderCount        int                    `bson:"orderCount"`
		CurrentMetrics    map[string]interface{} `bson:"currentMetrics"`
	}

	var visitors []visitorDoc
	if err := cur.All(ctx, &visitors); err != nil {
		log.Fatalf("Decode: %v", err)
	}
	cur.Close(ctx)

	// 1. Thống kê từ crm_customers
	hasConvField := 0
	convCountGt0 := 0
	primarySource := make(map[string]int64)
	for _, v := range visitors {
		if v.HasConversation {
			hasConvField++
		}
		if v.ConversationCount > 0 {
			convCountGt0++
		}
		ps := v.PrimarySource
		if ps == "" {
			ps = "(rỗng)"
		}
		primarySource[ps]++
	}

	fmt.Println("--- 1. Từ crm_customers (field lưu) ---")
	fmt.Printf("  hasConversation=true:  %d\n", hasConvField)
	fmt.Printf("  hasConversation=false: %d (đây là visitor theo định nghĩa)\n", int(total)-hasConvField)
	fmt.Printf("  conversationCount>0:  %d\n", convCountGt0)
	fmt.Println("  Phân bố primarySource:")
	for k, n := range primarySource {
		fmt.Printf("    %s: %d\n", k, n)
	}

	// 2. Kiểm tra thực tế: có conv trong fb_conversations không?
	fmt.Println("\n--- 2. Kiểm tra thực tế: có conversation trong fb_conversations không? ---")
	hasConvActual := 0
	noConvActual := 0
	var sampleMismatch []string

	for i, v := range visitors {
		if v.HasConversation || v.ConversationCount > 0 {
			continue // Đã có conv trong CRM — không cần check thực tế
		}

		ids := []string{v.UnifiedId}
		if v.SourceIds != nil {
			ids = append(ids, getStr(v.SourceIds, "pos"), getStr(v.SourceIds, "fb"))
		}
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
			cleanIds = []string{v.UnifiedId}
		}

		convFilter := bson.M{"ownerOrganizationId": orgID}
		convFilter["$or"] = buildConvFilter(cleanIds, numIds)["$or"]
		n, _ := convColl.CountDocuments(ctx, convFilter)
		if n > 0 {
			hasConvActual++
			if len(sampleMismatch) < 15 {
				sampleMismatch = append(sampleMismatch, v.UnifiedId)
			}
		} else {
			noConvActual++
		}

		if (i+1)%100 == 0 {
			fmt.Printf("  Đã kiểm tra %d/%d...\r", i+1, len(visitors))
		}
	}

	fmt.Printf("\n  Visitor có hasConversation=false (hoặc conversationCount=0):\n")
	fmt.Printf("    - Có conv trong fb_conversations (MISMATCH): %d\n", hasConvActual)
	fmt.Printf("    - Không có conv: %d\n", noConvActual)
	if len(sampleMismatch) > 0 {
		fmt.Printf("  Mẫu visitor MISMATCH (có conv nhưng bị tính visitor): %v\n", sampleMismatch)
	}

	fmt.Println("\n--- 3. Kết luận ---")
	if hasConvActual > 0 {
		fmt.Printf("⚠️ Có %d visitor thực tế CÓ conversation trong DB — đáng lẽ phải là engaged.\n", hasConvActual)
		fmt.Println("   Nguyên nhân: hasConversation chưa được cập nhật (linkage, expandIds, checkHasConversation).")
		fmt.Println("   Gợi ý: chạy RecalculateMismatchCustomers hoặc RecalculateOrderCountMismatchCustomers.")
	} else {
		fmt.Println("✓ Tất cả visitor đều không có conversation trong fb_conversations — phân loại đúng.")
	}

	fmt.Println("\n✓ Hoàn thành")
}
