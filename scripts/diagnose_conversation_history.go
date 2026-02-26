// Script chẩn đoán lịch sử hội thoại: kiểm tra fb_conversations -> crm_customers -> crm_activities.
// Giúp tìm nguyên nhân conversation_started không hiện trong lịch sử khách hàng.
// Chạy: go run scripts/diagnose_conversation_history.go [ownerOrganizationId]
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	convColl := db.Collection("fb_conversations")
	crmColl := db.Collection("crm_customers")
	actColl := db.Collection("crm_activities")

	// Lọc theo org nếu truyền tham số
	var orgFilter bson.M
	if len(os.Args) > 1 && os.Args[1] != "" {
		orgID, err := primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
		orgFilter = bson.M{"ownerOrganizationId": orgID}
		fmt.Printf("Lọc theo org: %s\n\n", os.Args[1])
	}

	// 1. Thống kê fb_conversations
	fmt.Println("=== FB_CONVERSATIONS ===\n")
	total, _ := convColl.CountDocuments(ctx, bson.M{})
	fmt.Printf("Tổng số conversations: %d\n", total)

	if orgFilter != nil {
		totalOrg, _ := convColl.CountDocuments(ctx, orgFilter)
		fmt.Printf("  - Có ownerOrganizationId (đúng org): %d\n", totalOrg)
	}

	// Conversations thiếu ownerOrganizationId
	noOwnerFilter := bson.M{
		"$or": []bson.M{
			{"ownerOrganizationId": bson.M{"$exists": false}},
			{"ownerOrganizationId": primitive.NilObjectID},
			{"ownerOrganizationId": ""},
		},
	}
	if orgFilter != nil && len(orgFilter) > 0 {
		noOwnerFilter = bson.M{"$and": []bson.M{orgFilter, noOwnerFilter}}
	}
	noOwner, _ := convColl.CountDocuments(ctx, noOwnerFilter)
	fmt.Printf("  - Thiếu ownerOrganizationId: %d (cần chạy backfill_conversation_ownerorg)\n", noOwner)

	// Conversations có customerId
	hasCustomerFilter := bson.M{"customerId": bson.M{"$exists": true, "$ne": ""}}
	if orgFilter != nil && len(orgFilter) > 0 {
		hasCustomerFilter = bson.M{"$and": []bson.M{orgFilter, hasCustomerFilter}}
	}
	hasCustomer, _ := convColl.CountDocuments(ctx, hasCustomerFilter)
	fmt.Printf("  - Có customerId: %d\n", hasCustomer)

	// 2. Kiểm tra resolve: customerId -> crm_customers
	fmt.Println("\n=== RESOLVE CUSTOMERID -> CRM_CUSTOMERS ===\n")
	opts := options.Find().SetLimit(500)
	convFindFilter := hasCustomerFilter
	if orgFilter != nil && len(orgFilter) > 0 {
		convFindFilter = bson.M{"$and": []bson.M{orgFilter, hasCustomerFilter}}
	}
	cursor, err := convColl.Find(ctx, convFindFilter, opts)
	if err != nil {
		log.Printf("Find conversations lỗi: %v", err)
		return
	}
	defer cursor.Close(ctx)

	resolved := 0
	unresolved := 0
	var unresolvedSamples []string
	customerIds := make(map[string]bool)

	for cursor.Next(ctx) {
		var doc struct {
			CustomerId string `bson:"customerId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if doc.CustomerId == "" || customerIds[doc.CustomerId] {
			continue
		}
		customerIds[doc.CustomerId] = true

		// Tìm crm_customer có sourceIds.fb, sourceIds.pos, hoặc unifiedId = customerId
		filter := bson.M{
			"$or": []bson.M{
				{"unifiedId": doc.CustomerId},
				{"sourceIds.fb": doc.CustomerId},
				{"sourceIds.pos": doc.CustomerId},
			},
		}
		if orgID, ok := orgFilter["ownerOrganizationId"]; ok && orgID != nil {
			filter["ownerOrganizationId"] = orgID
		}
		count, _ := crmColl.CountDocuments(ctx, filter)
		if count > 0 {
			resolved++
		} else {
			unresolved++
			if len(unresolvedSamples) < 5 {
				unresolvedSamples = append(unresolvedSamples, doc.CustomerId)
			}
		}
	}

	fmt.Printf("Trong %d customerId duy nhất (mẫu 500 conv):\n", len(customerIds))
	fmt.Printf("  - Resolve được sang crm_customers: %d\n", resolved)
	fmt.Printf("  - Không resolve được: %d\n", unresolved)
	if len(unresolvedSamples) > 0 {
		fmt.Printf("  - Mẫu customerId không resolve: %v\n", unresolvedSamples)
	}

	// Kiểm tra fb_customers có các customerId này không (MergeFromFbCustomer cần)
	fbColl := db.Collection("fb_customers")
	fbHasCount := 0
	for cid := range customerIds {
		f := bson.M{"customerId": cid}
		if orgID, ok := orgFilter["ownerOrganizationId"]; ok && orgID != nil {
			f["ownerOrganizationId"] = orgID
		}
		if n, _ := fbColl.CountDocuments(ctx, f); n > 0 {
			fbHasCount++
		}
	}
	fmt.Printf("  - Có trong fb_customers (có thể merge): %d\n", fbHasCount)
	if fbHasCount > 0 && resolved == 0 {
		fmt.Println("  → Gọi POST /api/v1/customers/backfill-activity để merge và tạo activity.")
	}

	// 3. Kiểm tra crm_activities conversation_started
	fmt.Println("\n=== CRM_ACTIVITIES (conversation_started) ===\n")
	actFilter := bson.M{"activityType": "conversation_started"}
	if orgFilter != nil {
		if oid, ok := orgFilter["ownerOrganizationId"]; ok && oid != nil {
			actFilter["ownerOrganizationId"] = oid
		}
	}
	actCount, _ := actColl.CountDocuments(ctx, actFilter)
	fmt.Printf("Số activity conversation_started: %d\n", actCount)

	fmt.Println("\n=== KẾT LUẬN ===\n")
	if noOwner > 0 {
		fmt.Println("1. Chạy: go run scripts/backfill_conversation_ownerorg.go")
		fmt.Println("   để cập nhật ownerOrganizationId cho conversations thiếu.")
	}
	if unresolved > 0 {
		fmt.Println("2. Nhiều conversation có customerId không match crm_customers.")
		fmt.Println("   - Kiểm tra fb_customers, pc_pos_customers có customerId tương ứng không.")
		fmt.Println("   - Đã thêm MergeFromPosCustomer fallback trong IngestConversationTouchpoint.")
	}
	fmt.Println("3. Sau khi sửa, gọi: POST /api/v1/customers/backfill-activity")
	fmt.Println("   với body: {\"ownerOrganizationId\": \"<org_hex>\"}")
}

