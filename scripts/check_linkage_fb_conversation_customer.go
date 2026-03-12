// Script kiểm tra liên kết fb_conversations theo _LINKAGE_KEYS.md và BAO_CAO_LIEN_KET_DU_LIEU.
//
// Theo tài liệu:
// - fb_conversations: ownerOrganizationId, pageId -> auth_organizations, fb_pages
// - fb_conversations.panCakeData.customers[].id -> fb_customers.customerId (BAO_CAO 2.2)
// - crm_customers: sourceIds.pos -> pc_pos_customers.customerId, sourceIds.fb -> fb_customers.customerId
//
// Kiểm tra:
// 1. fb_conversations.panCakeData.customers[].id / customerId -> fb_customers.customerId (tỷ lệ khớp)
// 2. Trùng customer: cùng customerId xuất hiện trong nhiều conv (phân bố)
// 3. Trong 1 conv: nhiều customer ID khác nhau có trùng nhau không (cùng 1 người?)
//
// Chạy: go run scripts/check_linkage_fb_conversation_customer.go [ownerOrganizationId]
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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

func extractId(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int:
		return fmt.Sprintf("%d", x)
	case int64:
		return fmt.Sprintf("%d", x)
	}
	return ""
}

func extractIdsFromConv(doc map[string]interface{}) []string {
	var ids []string
	seen := make(map[string]bool)
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			ids = append(ids, s)
		}
	}

	if cid, ok := doc["customerId"].(string); ok {
		add(strings.TrimSpace(cid))
	}
	pd, _ := doc["panCakeData"].(map[string]interface{})
	if pd == nil {
		return ids
	}
	// panCakeData.customers[].id (theo _LINKAGE_KEYS, BAO_CAO 2.2)
	if arr, ok := pd["customers"].([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				add(extractId(m["id"]))
			}
		}
	}
	// page_customer.id, customer.id, customer_id (fallback)
	if pc, ok := pd["page_customer"].(map[string]interface{}); ok {
		add(extractId(pc["id"]))
	}
	if cust, ok := pd["customer"].(map[string]interface{}); ok {
		add(extractId(cust["id"]))
	}
	add(extractId(pd["customer_id"]))
	return ids
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

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(dbName)
	convColl := db.Collection("fb_conversations")
	fbCustColl := db.Collection("fb_customers")

	orgID := primitive.NilObjectID
	if len(os.Args) > 1 {
		oid, err := primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatal("ownerOrganizationId không hợp lệ")
		}
		orgID = oid
	} else {
		var doc struct {
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if convColl.FindOne(context.Background(), bson.M{}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&doc) == nil {
			orgID = doc.OwnerOrganizationID
		} else {
			log.Fatal("Chạy với: go run scripts/check_linkage_fb_conversation_customer.go <ownerOrganizationId>")
		}
	}

	ctx := context.Background()
	filter := bson.M{"ownerOrganizationId": orgID}

	// 1. Build set fb_customers.customerId
	fbCustomerIds := make(map[string]bool)
	fbCur, _ := fbCustColl.Find(ctx, filter, options.Find().SetProjection(bson.M{"customerId": 1}))
	for fbCur.Next(ctx) {
		var d struct {
			CustomerId string `bson:"customerId"`
		}
		if fbCur.Decode(&d) == nil && d.CustomerId != "" {
			fbCustomerIds[d.CustomerId] = true
		}
	}
	fbCur.Close(ctx)

	// 2. Quét fb_conversations
	convCur, err := convColl.Find(ctx, filter, nil)
	if err != nil {
		log.Fatalf("Find fb_conversations: %v", err)
	}
	defer convCur.Close(ctx)

	totalConvs := 0
	convWithCustomerId := 0
	convMatchFbCustomers := 0
	customerIdToConvCount := make(map[string]int)
	convIdCount := make(map[string]int)
	// Trong 1 conv: có nhiều ID khác nhau không (cùng 1 người?)
	convMultiIdSamePerson := 0
	var multiIdSamples []string

	for convCur.Next(ctx) {
		totalConvs++
		var doc map[string]interface{}
		if convCur.Decode(&doc) != nil {
			continue
		}
		convId := extractId(doc["conversationId"])
		if convId != "" {
			convIdCount[convId]++
		}

		ids := extractIdsFromConv(doc)
		if len(ids) == 0 {
			continue
		}
		convWithCustomerId++

		matched := false
		for _, id := range ids {
			if fbCustomerIds[id] {
				matched = true
				break
			}
		}
		if matched {
			convMatchFbCustomers++
		}

		// Đếm conv per customerId (dùng customers[0].id hoặc id đầu tiên)
		primaryId := ids[0]
		customerIdToConvCount[primaryId]++

		// Trong 1 conv có >1 ID khác nhau
		if len(ids) > 1 {
			convMultiIdSamePerson++
			if len(multiIdSamples) < 3 {
				multiIdSamples = append(multiIdSamples, fmt.Sprintf("%s: %v", convId, ids))
			}
		}
	}

	// Phân bố: customerId có bao nhiêu conv
	oneConv := 0
	multiConv := 0
	for _, c := range customerIdToConvCount {
		if c == 1 {
			oneConv++
		} else {
			multiConv++
		}
	}

	// conversationId trùng
	dupConvId := 0
	for _, c := range convIdCount {
		if c > 1 {
			dupConvId++
		}
	}

	fmt.Println("=== Kiểm tra liên kết fb_conversations (theo _LINKAGE_KEYS.md) ===\n")
	fmt.Printf("Org: %s\n\n", orgID.Hex())

	fmt.Println("--- 1. Liên kết fb_conversations.panCakeData.customers[].id -> fb_customers.customerId ---")
	fmt.Printf("  Tổng fb_conversations: %d\n", totalConvs)
	fmt.Printf("  Conv có customerId/customers[].id: %d\n", convWithCustomerId)
	fmt.Printf("  fb_customers (customerId): %d\n", len(fbCustomerIds))
	matchPct := 0.0
	if convWithCustomerId > 0 {
		matchPct = float64(convMatchFbCustomers) * 100 / float64(convWithCustomerId)
	}
	fmt.Printf("  Conv khớp fb_customers: %d (%.1f%%)\n", convMatchFbCustomers, matchPct)

	fmt.Println("\n--- 2. Trùng customer: cùng customerId trong nhiều conv (phân bố) ---")
	fmt.Printf("  Số customerId duy nhất: %d\n", len(customerIdToConvCount))
	fmt.Printf("  Customer có 1 conv: %d\n", oneConv)
	fmt.Printf("  Customer có >1 conv: %d (bình thường — 1 khách nhiều hội thoại)\n", multiConv)

	fmt.Println("\n--- 3. conversationId trùng (bản ghi duplicate) ---")
	fmt.Printf("  Số conversationId xuất hiện >1 lần: %d\n", dupConvId)

	fmt.Println("\n--- 4. Trong 1 conv có nhiều customer ID (panCakeData) ---")
	fmt.Printf("  Số conv có >1 ID (customers[].id, page_customer.id, customer_id...): %d\n", convMultiIdSamePerson)
	if len(multiIdSamples) > 0 {
		fmt.Println("  Mẫu:")
		for _, s := range multiIdSamples {
			fmt.Printf("    %s\n", s)
		}
	}

	fmt.Println("\n--- 5. Kết luận theo tài liệu ---")
	if matchPct < 50 {
		fmt.Printf("  ⚠️ Tỷ lệ khớp fb_customers thấp (%.1f%%) — kiểm tra panCakeData.customers[].id vs fb_customers.customerId.\n", matchPct)
	} else {
		fmt.Printf("  ✓ Liên kết fb_conversations -> fb_customers: %.1f%% khớp.\n", matchPct)
	}
	if dupConvId > 0 {
		fmt.Printf("  ⚠️ Có %d conversationId trùng (duplicate records).\n", dupConvId)
	} else {
		fmt.Println("  ✓ Không có conversationId trùng.")
	}
	fmt.Println("\n✓ Hoàn thành")
}
