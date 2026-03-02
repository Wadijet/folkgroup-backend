// Script chẩn đoán crm_pending_ingest đã xử lý nhưng không tạo customer.
// Tìm các job processedAt có nhưng không tương ứng với crm_customers.
// Chạy: go run scripts/diagnose_ingest_no_customer.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func splitFirst(s, sep string) []string {
	return strings.SplitN(s, sep, 2)
}

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
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH (hoặc MONGODB_DBNAME)")
	}
	if dbName == "" {
		dbName = os.Getenv("MONGODB_DBNAME")
	}
	if dbName == "" {
		dbName = os.Getenv("MONGODB_DBNAME_AUTH")
	}
	if dbName == "" {
		log.Fatal("Cần MONGODB_DBNAME hoặc MONGODB_DBNAME_AUTH trong .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	ingestColl := db.Collection("crm_pending_ingest")
	crmColl := db.Collection("crm_customers")

	// Lấy tất cả ingest đã xử lý (processedAt != null)
	filter := bson.M{"processedAt": bson.M{"$exists": true, "$ne": nil}}
	opts := options.Find().SetSort(bson.D{{Key: "processedAt", Value: -1}}).SetLimit(100)
	cursor, err := ingestColl.Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("Query lỗi: %v", err)
	}
	defer cursor.Close(ctx)

	type IngestDoc struct {
		ID             primitive.ObjectID `bson:"_id"`
		CollectionName string             `bson:"collectionName"`
		BusinessKey    string             `bson:"businessKey"`
		Operation      string             `bson:"operation"`
		OwnerOrgID     primitive.ObjectID `bson:"ownerOrganizationId"`
		ProcessedAt    *int64             `bson:"processedAt"`
		ProcessError   string             `bson:"processError"`
		Document       bson.M             `bson:"document"`
	}

	var list []IngestDoc
	if err := cursor.All(ctx, &list); err != nil {
		log.Fatalf("Decode lỗi: %v", err)
	}

	fmt.Printf("=== CRM_PENDING_INGEST ĐÃ XỬ LÝ (tổng: %d) ===\n\n", len(list))

	noCustomerCount := 0
	for i, item := range list {
		reason := ""
		customerIdFromDoc := ""

		// collectionName có thể không có trong doc (EnqueueCrmIngest không set) — lấy từ businessKey
		collectionName := item.CollectionName
		if collectionName == "" && len(item.BusinessKey) > 0 {
			parts := splitFirst(item.BusinessKey, "|")
			if len(parts) >= 1 {
				collectionName = parts[0]
			}
		}

		switch collectionName {
		case "pc_pos_customers", "fb_customers":
			reason = "Merge từ POS/FB — luôn tạo/cập nhật customer"
		case "pc_pos_orders":
			if doc, ok := item.Document["posData"].(map[string]interface{}); ok {
				if c, ok := doc["customer"].(map[string]interface{}); ok {
					if id, ok := c["id"].(string); ok {
						customerIdFromDoc = id
					}
				}
			}
			if customerIdFromDoc == "" {
				if id, ok := item.Document["customerId"].(string); ok {
					customerIdFromDoc = id
				}
			}
			if customerIdFromDoc == "" {
				reason = "Order không có customerId — worker không tạo customer (return nil)"
				noCustomerCount++
			} else {
				reason = fmt.Sprintf("Order có customerId=%s — cần kiểm tra crm_customers", customerIdFromDoc)
			}
		case "fb_conversations":
			// extractConversationCustomerId: customers[0].id, page_customer.id, customer_id
			if doc, ok := item.Document["panCakeData"].(map[string]interface{}); ok {
				if arr, ok := doc["customers"].([]interface{}); ok && len(arr) > 0 {
					if m, ok := arr[0].(map[string]interface{}); ok {
						if id, ok := m["id"].(string); ok {
							customerIdFromDoc = id
						}
					}
				}
				if customerIdFromDoc == "" {
					if m, ok := doc["page_customer"].(map[string]interface{}); ok {
						if id, ok := m["id"].(string); ok {
							customerIdFromDoc = id
						}
					}
				}
				if customerIdFromDoc == "" {
					if id, ok := doc["customer_id"].(string); ok {
						customerIdFromDoc = id
					}
				}
			}
			if customerIdFromDoc == "" {
				if id, ok := item.Document["customerId"].(string); ok {
					customerIdFromDoc = id
				}
			}
			if customerIdFromDoc == "" {
				reason = "Conversation không có customerId (customers[0].id, page_customer.id, customer_id) — worker không tạo"
				noCustomerCount++
			} else {
				reason = fmt.Sprintf("Conversation có customerId=%s — cần kiểm tra crm_customers", customerIdFromDoc)
			}
		case "crm_notes":
			reason = "Note — không tạo customer mới, cần customerId có sẵn"
		default:
			reason = ""
		}

		// Kiểm tra crm_customers có tồn tại không (cho orders/conversations với customerId)
		// UpsertMinimalFromFbId tạo với unifiedId=customerId và sourceIds.fb=customerId
		hasCustomer := false
		if customerIdFromDoc != "" && !item.OwnerOrgID.IsZero() {
			filter := bson.M{
				"ownerOrganizationId": item.OwnerOrgID,
				"$or": []bson.M{
					{"unifiedId": customerIdFromDoc},
					{"sourceIds.pos": customerIdFromDoc},
					{"sourceIds.fb": customerIdFromDoc},
				},
			}
			n, _ := crmColl.CountDocuments(ctx, filter)
			hasCustomer = n > 0
		}

		// In ra chi tiết
		status := "✓"
		if !hasCustomer && (collectionName == "pc_pos_orders" || collectionName == "fb_conversations") && customerIdFromDoc != "" {
			status = "⚠ KHÔNG CÓ CUSTOMER"
			noCustomerCount++
		} else if customerIdFromDoc == "" && (collectionName == "pc_pos_orders" || collectionName == "fb_conversations") {
			status = "⚠ KHÔNG CÓ CUSTOMER_ID"
		}

		fmt.Printf("[%d] %s %s | %s | %s\n", i+1, status, collectionName, item.Operation, item.BusinessKey)
		fmt.Printf("    Org: %s | ProcessedAt: %v\n", item.OwnerOrgID.Hex(), item.ProcessedAt)
		if item.ProcessError != "" {
			fmt.Printf("    ProcessError: %s\n", item.ProcessError)
		}
		fmt.Printf("    %s\n", reason)
		if customerIdFromDoc != "" {
			fmt.Printf("    customerIdFromDoc: %s | hasCustomer: %v\n", customerIdFromDoc, hasCustomer)
		}
		fmt.Println()
	}

	fmt.Printf("=== TỔNG KẾT ===\n")
	fmt.Printf("Số ingest không có customerId (order/conversation): %d\n", noCustomerCount)
}
