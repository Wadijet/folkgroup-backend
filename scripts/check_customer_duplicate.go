// Script kiểm tra khách hàng trùng - Ann Le, phone 84909098999, unifiedId f2127553-...
// Chạy: go run scripts/check_customer_duplicate.go
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	// 1. crm_customers - tìm theo unifiedId, phone, name
	fmt.Println("=== CRM_CUSTOMERS ===\n")
	unifiedIdPrefix := "f2127553"
	phone := "84909098999"
	phoneVariants := []string{"84909098999", "0909098999"}

	// Tìm theo unifiedId chứa prefix
	var crm []bson.M
	cursor, _ := db.Collection("crm_customers").Find(ctx, bson.M{
		"$or": []bson.M{
			{"unifiedId": bson.M{"$regex": "^" + unifiedIdPrefix}},
			{"phoneNumbers": phone},
			{"phoneNumbers": bson.M{"$in": phoneVariants}},
			{"name": bson.M{"$regex": "Ann Le", "$options": "i"}},
		},
	})
	cursor.All(ctx, &crm)
	cursor.Close(ctx)

	fmt.Printf("Số bản ghi crm_customers match: %d\n", len(crm))
	for i, d := range crm {
		fmt.Printf("\n--- crm_customers #%d ---\n", i+1)
		fmt.Printf("  _id: %v\n", d["_id"])
		fmt.Printf("  unifiedId: %v\n", d["unifiedId"])
		fmt.Printf("  name: %v\n", d["name"])
		fmt.Printf("  phoneNumbers: %v\n", d["phoneNumbers"])
		fmt.Printf("  sourceIds: %v\n", d["sourceIds"])
		fmt.Printf("  ownerOrganizationId: %v\n", d["ownerOrganizationId"])
	}

	// 2. pc_pos_customers - tìm theo customerId
	fmt.Println("\n--- PC_POS_CUSTOMERS ---\n")
	var pos []bson.M
	cursor, _ = db.Collection("pc_pos_customers").Find(ctx, bson.M{
		"$or": []bson.M{
			{"customerId": bson.M{"$regex": "^" + unifiedIdPrefix}},
			{"phoneNumbers": phone},
			{"name": bson.M{"$regex": "Ann Le", "$options": "i"}},
		},
	})
	cursor.All(ctx, &pos)
	cursor.Close(ctx)

	fmt.Printf("Số bản ghi pc_pos_customers match: %d\n", len(pos))
	for i, d := range pos {
		fmt.Printf("\n--- pc_pos_customers #%d ---\n", i+1)
		fmt.Printf("  _id: %v\n", d["_id"])
		fmt.Printf("  customerId: %v\n", d["customerId"])
		fmt.Printf("  name: %v\n", d["name"])
		fmt.Printf("  phoneNumbers: %v\n", d["phoneNumbers"])
		fmt.Printf("  ownerOrganizationId: %v\n", d["ownerOrganizationId"])
	}

	// 3. fb_customers - tìm theo customerId, phone
	fmt.Println("\n--- FB_CUSTOMERS ---\n")
	var fb []bson.M
	cursor, _ = db.Collection("fb_customers").Find(ctx, bson.M{
		"$or": []bson.M{
			{"customerId": bson.M{"$regex": "^" + unifiedIdPrefix}},
			{"phoneNumbers": phone},
			{"name": bson.M{"$regex": "Ann Le", "$options": "i"}},
		},
	})
	cursor.All(ctx, &fb)
	cursor.Close(ctx)

	fmt.Printf("Số bản ghi fb_customers match: %d\n", len(fb))
	for i, d := range fb {
		fmt.Printf("\n--- fb_customers #%d ---\n", i+1)
		fmt.Printf("  _id: %v\n", d["_id"])
		fmt.Printf("  customerId: %v\n", d["customerId"])
		fmt.Printf("  name: %v\n", d["name"])
		fmt.Printf("  phoneNumbers: %v\n", d["phoneNumbers"])
		fmt.Printf("  ownerOrganizationId: %v\n", d["ownerOrganizationId"])
	}

	// 4. crm_activity_history - lịch sử của khách (nếu có unifiedId)
	if len(crm) > 0 {
		uid, _ := crm[0]["unifiedId"].(string)
		if uid != "" {
			fmt.Println("\n--- CRM_ACTIVITY_HISTORY (unifiedId) ---\n")
			var acts []bson.M
			cursor, _ = db.Collection("crm_activity_history").Find(ctx, bson.M{"unifiedId": uid})
			cursor.All(ctx, &acts)
			cursor.Close(ctx)
			fmt.Printf("Số activity: %d\n", len(acts))
			for i, a := range acts {
				fmt.Printf("  #%d: %v | %v | sourceRef=%v\n", i+1, a["activityType"], a["source"], a["sourceRef"])
			}
		}
	}

	// 5. Phân tích trùng
	fmt.Println("\n=== PHÂN TÍCH TRÙNG ===\n")
	if len(crm) > 1 {
		fmt.Printf("⚠️  Cảnh báo: %d bản ghi crm_customers trùng (cùng khách)\n", len(crm))
	}
	if len(pos) > 1 {
		fmt.Printf("⚠️  Cảnh báo: %d bản ghi pc_pos_customers trùng (cùng customerId)\n", len(pos))
	}
	if len(fb) > 1 {
		fmt.Printf("⚠️  Cảnh báo: %d bản ghi fb_customers trùng (cùng customerId)\n", len(fb))
	}
	if len(crm) <= 1 && len(pos) <= 1 && len(fb) <= 1 {
		fmt.Println("Không phát hiện trùng trong crm/pos/fb_customers.")
	}
	fmt.Println("\n✓ Hoàn thành")
}
