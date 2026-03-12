// Script kiểm tra fb_conversations: có conversation nào trùng customer không?
// Kiểm tra:
// 1. conversationId trùng (bản ghi conv bị duplicate)
// 2. Trong 1 conversation: nhiều customer ID khác nhau (customers[0].id, page_customer.id, customer_id) — có trùng nhau không?
//
// Chạy: go run scripts/check_fb_conversation_duplicate_customer.go [ownerOrganizationId]
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

func extractIdFromMap(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	if s, ok := m["id"].(string); ok && s != "" {
		return s
	}
	if n, ok := m["id"].(float64); ok {
		return fmt.Sprintf("%.0f", n)
	}
	if n, ok := m["id"].(int); ok {
		return fmt.Sprintf("%d", n)
	}
	return ""
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
			log.Fatal("Chạy với: go run scripts/check_fb_conversation_duplicate_customer.go <ownerOrganizationId>")
		}
	}

	ctx := context.Background()
	filter := bson.M{"ownerOrganizationId": orgID}
	opts := options.Find().SetProjection(bson.M{"conversationId": 1, "customerId": 1, "panCakeData": 1})

	cursor, err := convColl.Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("Find fb_conversations: %v", err)
	}
	defer cursor.Close(ctx)

	// 1. conversationId trùng (duplicate records)
	convIdCount := make(map[string]int)
	// 2. Trong 1 conv: tập customer IDs từ panCakeData
	// convId -> []string (các ID khác nhau trong cùng conv)
	convIdToCustomerIds := make(map[string]map[string]bool)
	totalConvs := 0

	for cursor.Next(ctx) {
		totalConvs++
		var doc struct {
			ConversationId string                 `bson:"conversationId"`
			CustomerId     string                 `bson:"customerId"`
			PanCakeData    map[string]interface{} `bson:"panCakeData"`
		}
		if cursor.Decode(&doc) != nil {
			continue
		}
		convId := strings.TrimSpace(doc.ConversationId)
		if convId != "" {
			convIdCount[convId]++
		}

		// Thu thập tất cả customer ID trong conv
		ids := make(map[string]bool)
		cid := strings.TrimSpace(doc.CustomerId)
		if cid != "" {
			ids[cid] = true
		}
		if doc.PanCakeData != nil {
			pd := doc.PanCakeData
			if arr, ok := pd["customers"].([]interface{}); ok && len(arr) > 0 {
				if m, ok := arr[0].(map[string]interface{}); ok {
					if id := extractIdFromMap(m); id != "" {
						ids[id] = true
					}
				}
			}
			if pc, ok := pd["page_customer"].(map[string]interface{}); ok {
				if id := extractIdFromMap(pc); id != "" {
					ids[id] = true
				}
			}
			if cust, ok := pd["customer"].(map[string]interface{}); ok {
				if id := extractIdFromMap(cust); id != "" {
					ids[id] = true
				}
			}
			if s, ok := pd["customer_id"].(string); ok && s != "" {
				ids[s] = true
			}
			if n, ok := pd["customer_id"].(float64); ok {
				ids[fmt.Sprintf("%.0f", n)] = true
			}
		}
		if len(ids) > 0 {
			convIdToCustomerIds[convId] = ids
		}
	}

	// Phân tích
	dupConvIds := 0
	var dupConvSamples []string
	for cid, c := range convIdCount {
		if c > 1 {
			dupConvIds++
			if len(dupConvSamples) < 5 {
				dupConvSamples = append(dupConvSamples, fmt.Sprintf("%s (%d lần)", cid, c))
			}
		}
	}

	// Conv có nhiều customer ID khác nhau (có thể cùng 1 người nhưng nhiều định danh)
	multiIdConvs := 0
	var multiIdSamples []string
	for convId, ids := range convIdToCustomerIds {
		if len(ids) > 1 {
			multiIdConvs++
			var idList []string
			for id := range ids {
				idList = append(idList, id)
			}
			if len(multiIdSamples) < 5 {
				multiIdSamples = append(multiIdSamples, fmt.Sprintf("%s: %v", convId, idList))
			}
		}
	}

	fmt.Println("=== Kiểm tra fb_conversations trùng customer ===\n")
	fmt.Printf("Org: %s\n\n", orgID.Hex())
	fmt.Printf("Tổng bản ghi fb_conversations: %d\n", totalConvs)
	fmt.Printf("Số conversationId duy nhất: %d\n\n", len(convIdCount))

	fmt.Println("--- 1. conversationId trùng (bản ghi duplicate) ---")
	fmt.Printf("  Số conversationId xuất hiện >1 lần: %d\n", dupConvIds)
	if len(dupConvSamples) > 0 {
		fmt.Println("  Mẫu:")
		for _, s := range dupConvSamples {
			fmt.Printf("    %s\n", s)
		}
	}

	fmt.Println("\n--- 2. Trong 1 conversation có nhiều customer ID khác nhau ---")
	fmt.Printf("  Số conv có >1 customer ID (customers[0].id, page_customer.id, customer_id...): %d\n", multiIdConvs)
	if len(multiIdSamples) > 0 {
		fmt.Println("  Mẫu (có thể cùng 1 người nhưng nhiều định danh):")
		for _, s := range multiIdSamples {
			fmt.Printf("    %s\n", s)
		}
	}

	fmt.Println("\n--- 3. Kết luận ---")
	if dupConvIds > 0 {
		fmt.Printf("  ⚠️ Có %d conversationId bị trùng (duplicate records).\n", dupConvIds)
	} else {
		fmt.Println("  ✓ Không có conversationId trùng.")
	}
	if multiIdConvs > 0 {
		fmt.Printf("  ℹ️ Có %d conv chứa nhiều customer ID — có thể cùng 1 người (Pancake dùng nhiều field).\n", multiIdConvs)
	}
	fmt.Println("\n✓ Hoàn thành")
}
