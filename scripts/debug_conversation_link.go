// Script debug link conversation với crm_customer — kiểm tra vì sao recalculate không thấy hội thoại.
// Chạy: go run scripts/debug_conversation_link.go
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	fmt.Println("=== 1. FB_CONVERSATIONS — Mẫu 5 bản ghi có customerId !== \"\" ===\n")
	var convs []bson.M
	cursor, _ := db.Collection("fb_conversations").Find(ctx, bson.M{"customerId": bson.M{"$exists": true, "$ne": ""}}, options.Find().SetLimit(5))
	cursor.All(ctx, &convs)
	cursor.Close(ctx)

	for i, c := range convs {
		fmt.Printf("--- fb_conversations #%d ---\n", i+1)
		fmt.Printf("  conversationId: %v\n", c["conversationId"])
		fmt.Printf("  customerId (root): %v (type: %T)\n", c["customerId"], c["customerId"])
		fmt.Printf("  ownerOrganizationId: %v\n", c["ownerOrganizationId"])
		if pd, ok := c["panCakeData"].(bson.M); ok && pd != nil {
			fmt.Printf("  panCakeData.customer_id: %v\n", pd["customer_id"])
			fmt.Printf("  panCakeData.customer: %v\n", pd["customer"])
			if cust, ok := pd["customer"].(bson.M); ok {
				fmt.Printf("    customer.id: %v\n", cust["id"])
				fmt.Printf("    customer.name: %v\n", cust["name"])
			}
			if arr, ok := pd["customers"].(bson.A); ok && len(arr) > 0 {
				if m, ok := arr[0].(bson.M); ok {
					fmt.Printf("  panCakeData.customers[0].id: %v\n", m["id"])
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("\n=== 2. CRM_CUSTOMERS — Mẫu 5 bản ghi (có profile hoặc sourceIds) ===\n")
	var crm []bson.M
	cursor, _ = db.Collection("crm_customers").Find(ctx, bson.M{}, options.Find().SetLimit(5).SetSort(bson.M{"updatedAt": -1}))
	cursor.All(ctx, &crm)
	cursor.Close(ctx)

	for i, c := range crm {
		fmt.Printf("--- crm_customers #%d ---\n", i+1)
		fmt.Printf("  unifiedId: %v\n", c["unifiedId"])
		fmt.Printf("  sourceIds: %v\n", c["sourceIds"])
		if si, ok := c["sourceIds"].(bson.M); ok {
			fmt.Printf("    pos: %v\n", si["pos"])
			fmt.Printf("    fb: %v\n", si["fb"])
		}
		if p, ok := c["profile"].(bson.M); ok && p != nil {
			fmt.Printf("  profile.phoneNumbers: %v\n", p["phoneNumbers"])
		}
		fmt.Printf("  phoneNumbers (legacy): %v\n", c["phoneNumbers"])
		fmt.Printf("  ownerOrganizationId: %v\n", c["ownerOrganizationId"])
		fmt.Printf("  currentMetrics (hasConversation): %v\n", getNested(c, "currentMetrics", "raw", "hasConversation"))
		fmt.Println()
	}

	fmt.Println("\n=== 3. SO SÁNH — customerId trong fb_conversations vs crm_customers ===\n")
	// Lấy tất cả customerId unique từ fb_conversations
	var convCustomerIds []string
	cursor, _ = db.Collection("fb_conversations").Find(ctx, bson.M{"customerId": bson.M{"$exists": true, "$ne": ""}}, options.Find().SetProjection(bson.M{"customerId": 1}))
	for cursor.Next(ctx) {
		var doc bson.M
		if cursor.Decode(&doc) == nil {
			if id, ok := doc["customerId"].(string); ok && id != "" {
				convCustomerIds = append(convCustomerIds, id)
			}
		}
	}
	cursor.Close(ctx)

	// Lấy tất cả ids từ crm_customers (unifiedId, sourceIds.pos, sourceIds.fb)
	crmIdSet := make(map[string]bool)
	cursor, _ = db.Collection("crm_customers").Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"unifiedId": 1, "sourceIds": 1}))
	for cursor.Next(ctx) {
		var doc bson.M
		if cursor.Decode(&doc) == nil {
			if id, ok := doc["unifiedId"].(string); ok && id != "" {
				crmIdSet[id] = true
			}
			if si, ok := doc["sourceIds"].(bson.M); ok {
				if p, ok := si["pos"].(string); ok && p != "" {
					crmIdSet[p] = true
				}
				if f, ok := si["fb"].(string); ok && f != "" {
					crmIdSet[f] = true
				}
			}
		}
	}
	cursor.Close(ctx)

	matched := 0
	unmatched := 0
	for _, convId := range convCustomerIds {
		if crmIdSet[convId] {
			matched++
		} else {
			unmatched++
		}
	}
	fmt.Printf("Số customerId trong fb_conversations: %d\n", len(convCustomerIds))
	fmt.Printf("  - Match với crm_customers (unifiedId/sourceIds): %d\n", matched)
	fmt.Printf("  - KHÔNG match: %d\n", unmatched)

	// Lấy vài customerId KHÔNG match để kiểm tra
	fmt.Println("\n--- Mẫu customerId trong fb_conversations KHÔNG có trong crm ---")
	shown := 0
	for _, convId := range convCustomerIds {
		if !crmIdSet[convId] && shown < 5 {
			fmt.Printf("  %v\n", convId)
			shown++
		}
	}

	fmt.Println("\n=== 4. FB_CUSTOMERS — customerId format (mẫu 3) ===\n")
	var fb []bson.M
	cursor, _ = db.Collection("fb_customers").Find(ctx, bson.M{}, options.Find().SetLimit(3).SetProjection(bson.M{"customerId": 1, "phoneNumbers": 1, "name": 1}))
	cursor.All(ctx, &fb)
	cursor.Close(ctx)
	for i, f := range fb {
		fmt.Printf("  #%d customerId=%v phoneNumbers=%v\n", i+1, f["customerId"], f["phoneNumbers"])
	}

	fmt.Println("\n=== 5. KIỂM TRA panCakeData.customer_id vs customerId root ===\n")
	var convCheck []bson.M
	cursor, _ = db.Collection("fb_conversations").Find(ctx, bson.M{}, options.Find().SetLimit(3).SetProjection(bson.M{"customerId": 1, "panCakeData.customer_id": 1, "panCakeData.customer": 1}))
	cursor.All(ctx, &convCheck)
	cursor.Close(ctx)
	for i, c := range convCheck {
		pd, _ := c["panCakeData"].(bson.M)
		custId := c["customerId"]
		panCustId := ""
		if pd != nil {
			panCustId, _ = pd["customer_id"].(string)
		}
		fmt.Printf("  #%d customerId(root)=%v panCakeData.customer_id=%v (cùng? %v)\n", i+1, custId, panCustId, custId == panCustId)
	}

	fmt.Println("\n✓ Hoàn thành debug")
}

func getNested(m bson.M, keys ...string) interface{} {
	for _, k := range keys {
		if m == nil {
			return nil
		}
		v, ok := m[k]
		if !ok {
			return nil
		}
		if next, ok := v.(bson.M); ok {
			m = next
		} else {
			return v
		}
	}
	return m
}
