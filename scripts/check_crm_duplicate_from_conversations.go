// Script kiểm tra crm_customers có bị tạo trùng do nhiều conversation cho cùng 1 người không.
// Kiểm tra: crm_customers trùng SĐT (cùng người có thể bị tạo nhiều customer từ nhiều conv với customerId khác nhau).
//
// Chạy: go run scripts/check_crm_duplicate_from_conversations.go [ownerOrganizationId]
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

func normalizePhone(p string) string {
	s := strings.TrimSpace(p)
	s = strings.TrimPrefix(s, "+84")
	s = strings.TrimPrefix(s, "84")
	if len(s) == 0 {
		return ""
	}
	if s[0] == '0' {
		return s
	}
	return "0" + s
}

func getPhonesFromProfile(profile interface{}) []string {
	if profile == nil {
		return nil
	}
	m, ok := profile.(map[string]interface{})
	if !ok {
		return nil
	}
	phones, ok := m["phoneNumbers"]
	if !ok || phones == nil {
		return nil
	}
	arr, ok := phones.(bson.A)
	if !ok {
		if arr2, ok := phones.([]interface{}); ok {
			arr = arr2
		} else {
			return nil
		}
	}
	var out []string
	for _, v := range arr {
		if s, ok := v.(string); ok && s != "" {
			norm := normalizePhone(s)
			if norm != "" {
				out = append(out, norm)
			}
		}
	}
	return out
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
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
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
		if crmColl.FindOne(context.Background(), bson.M{}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&doc) == nil {
			orgID = doc.OwnerOrganizationID
		} else {
			log.Fatal("Chạy với: go run scripts/check_crm_duplicate_from_conversations.go <ownerOrganizationId>")
		}
	}

	ctx := context.Background()
	filter := bson.M{"ownerOrganizationId": orgID}
	opts := options.Find().SetProjection(bson.M{"unifiedId": 1, "sourceIds": 1, "primarySource": 1, "profile": 1})

	cursor, err := crmColl.Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("Find crm_customers: %v", err)
	}
	// phone -> []unifiedId
	phoneToCrm := make(map[string][]string)
	var totalCrm int
	for cursor.Next(ctx) {
		totalCrm++
		var doc struct {
			UnifiedId    string      `bson:"unifiedId"`
			SourceIds    interface{} `bson:"sourceIds"`
			PrimarySource string     `bson:"primarySource"`
			Profile      interface{} `bson:"profile"`
		}
		if cursor.Decode(&doc) != nil {
			continue
		}
		phones := getPhonesFromProfile(doc.Profile)
		for _, p := range phones {
			if p != "" {
				phoneToCrm[p] = append(phoneToCrm[p], doc.UnifiedId)
			}
		}
	}
	cursor.Close(ctx)

	// 1. CRM trùng SĐT (cùng SĐT có >1 crm)
	dupByPhone := 0
	var dupSamples [][]string
	for phone, ids := range phoneToCrm {
		unique := make(map[string]bool)
		for _, id := range ids {
			unique[id] = true
		}
		if len(unique) > 1 {
			dupByPhone++
			var idList []string
			for id := range unique {
				idList = append(idList, id)
			}
			if len(dupSamples) < 5 {
				dupSamples = append(dupSamples, append([]string{phone}, idList...))
			}
		}
	}

	// 2. fb_conversations: thống kê customerId
	// customerId trong conv -> có khớp crm_customers.unifiedId hoặc sourceIds.fb không?
	convCursor, err := convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID}, options.Find().SetProjection(bson.M{"conversationId": 1, "customerId": 1, "panCakeData": 1}))
	if err != nil {
		log.Printf("Find fb_conversations: %v", err)
		fmt.Println("=== Kiểm tra crm_customers trùng do nhiều conversation ===\n")
		fmt.Printf("Org: %s\n\n", orgID.Hex())
		fmt.Println("--- 1. CRM trùng SĐT ---")
		fmt.Printf("  Tổng crm_customers: %d\n", totalCrm)
		fmt.Printf("  Số SĐT có >1 crm (trùng): %d\n", dupByPhone)
		if len(dupSamples) > 0 {
			fmt.Println("  Mẫu trùng SĐT:")
			for _, s := range dupSamples {
				fmt.Printf("    SĐT %s -> crm: %v\n", s[0], s[1:])
			}
		}
	} else {
		// customerId từ conv -> số conv
		customerIdToConvCount := make(map[string]int)
		// customerId từ panCakeData (customers[0].id, page_customer.id, customer_id)
		customerIdFromPanCake := make(map[string]int)
		for convCursor.Next(ctx) {
			var doc struct {
				ConversationId string                 `bson:"conversationId"`
				CustomerId     string                 `bson:"customerId"`
				PanCakeData    map[string]interface{} `bson:"panCakeData"`
			}
			if convCursor.Decode(&doc) != nil {
				continue
			}
			cid := strings.TrimSpace(doc.CustomerId)
			if cid != "" {
				customerIdToConvCount[cid]++
			}
			// Extract từ panCakeData (giống extractConversationCustomerId)
			if doc.PanCakeData != nil {
				pd := doc.PanCakeData
				if arr, ok := pd["customers"].([]interface{}); ok && len(arr) > 0 {
					if m, ok := arr[0].(map[string]interface{}); ok {
						if id, ok := m["id"].(string); ok && id != "" {
							customerIdFromPanCake[id]++
						}
					}
				}
				if pc, ok := pd["page_customer"].(map[string]interface{}); ok {
					if id, ok := pc["id"].(string); ok && id != "" {
						customerIdFromPanCake[id]++
					}
				}
				if s, ok := pd["customer_id"].(string); ok && s != "" {
					customerIdFromPanCake[s]++
				}
			}
		}
		convCursor.Close(ctx)

		// Số customerId duy nhất trong conv
		uniqueConvCustomerIds := len(customerIdToConvCount)
		uniquePanCakeIds := len(customerIdFromPanCake)
		// Có customerId nào trong conv có nhiều conv không?
		multiConvPerId := 0
		for _, c := range customerIdToConvCount {
			if c > 1 {
				multiConvPerId++
			}
		}

		fmt.Println("=== Kiểm tra crm_customers trùng do nhiều conversation ===\n")
		fmt.Printf("Org: %s\n\n", orgID.Hex())
		fmt.Println("--- 1. CRM trùng SĐT (cùng SĐT có >1 crm_customer) ---")
		fmt.Printf("  Tổng crm_customers: %d\n", totalCrm)
		fmt.Printf("  Số SĐT có >1 crm (trùng): %d\n", dupByPhone)
		if len(dupSamples) > 0 {
			fmt.Println("  Mẫu trùng SĐT:")
			for _, s := range dupSamples {
				fmt.Printf("    SĐT %s -> crm: %v\n", s[0], s[1:])
			}
		}
		fmt.Println("\n--- 2. fb_conversations: thống kê customerId ---")
		fmt.Printf("  Số customerId duy nhất (từ fb_conversations.customerId): %d\n", uniqueConvCustomerIds)
		fmt.Printf("  Số ID duy nhất từ panCakeData (customers[0].id, page_customer.id, customer_id): %d\n", uniquePanCakeIds)
		fmt.Printf("  Số customerId có nhiều conv (>1): %d\n", multiConvPerId)
	}

	fmt.Println("\n--- 3. Kết luận ---")
	if dupByPhone > 0 {
		fmt.Printf("  ⚠️ Có %d SĐT trùng nhiều crm_customer — có thể do nhiều conv với customerId khác nhau tạo nhiều crm.\n", dupByPhone)
	} else {
		fmt.Println("  ✓ Không phát hiện crm trùng SĐT (cùng SĐT có >1 crm).")
	}
	fmt.Println("\n✓ Hoàn thành")
}
