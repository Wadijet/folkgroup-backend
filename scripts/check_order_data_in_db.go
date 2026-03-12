// Script kiểm tra logic order: query pc_pos_orders với filter giống aggregateOrderMetricsForCustomer.
// Kiểm tra TẤT CẢ journey stages (visitor, engaged, first, repeat, vip, inactive): orderCount có khớp đơn trong DB không.
//
// Chạy: go run scripts/check_order_data_in_db.go [ownerOrganizationId]
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

func getFromMap(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	v, _ := m[key]
	return v
}

func getString(m map[string]interface{}, key string) string {
	v := getFromMap(m, key)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func getInt(m map[string]interface{}, key string) int {
	v := getFromMap(m, key)
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}

// getFromNestedMetrics đọc từ currentMetrics (raw/layer1/layer2) — giống service.crm.snapshot.
func getFromNestedMetrics(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	for _, layer := range []string{"layer2", "layer1", "raw"} {
		if sub, ok := m[layer].(map[string]interface{}); ok {
			if v, ok := sub[key]; ok {
				return v
			}
		}
	}
	return nil
}

// getOrderCountFromDoc đọc orderCount giống GetOrderCountFromCustomer: currentMetrics (nested) trước, fallback top-level.
func getOrderCountFromDoc(doc map[string]interface{}) int {
	if cm, ok := doc["currentMetrics"].(map[string]interface{}); ok && cm != nil {
		if v := getFromNestedMetrics(cm, "orderCount"); v != nil {
			switch x := v.(type) {
			case int:
				return x
			case int64:
				return int(x)
			case float64:
				return int(x)
			}
		}
		return getInt(cm, "orderCount")
	}
	return getInt(doc, "orderCount")
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	ordersColl := db.Collection("pc_pos_orders")

	var orgID primitive.ObjectID
	if len(os.Args) >= 2 {
		orgID, err = primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
	} else {
		var doc struct {
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if crmColl.FindOne(ctx, bson.M{}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&doc) == nil {
			orgID = doc.OwnerOrganizationID
		} else {
			log.Fatal("Chạy với: go run scripts/check_order_data_in_db.go <ownerOrganizationId>")
		}
	}

	fmt.Printf("=== Kiểm tra logic order trong DB — org: %s ===\n\n", orgID.Hex())

	// Các journey stage cần kiểm tra
	stages := []string{"visitor", "engaged", "first", "repeat", "vip", "inactive"}
	cancelledStatuses := []int{6}

	// Helper: đếm đơn trong DB cho 1 customer (filter giống aggregateOrderMetricsForCustomer)
	countOrdersInDB := func(doc map[string]interface{}) (int, []string, []string) {
		sourceIds, _ := doc["sourceIds"].(map[string]interface{})
		posId := getString(sourceIds, "pos")
		fbId := getString(sourceIds, "fb")
		uid := getString(doc, "unifiedId")
		profile, _ := doc["profile"].(map[string]interface{})
		phones := []string{}
		if p, ok := profile["phoneNumbers"].([]interface{}); ok {
			for _, x := range p {
				if s, ok := x.(string); ok && s != "" {
					phones = append(phones, s)
				}
			}
		}
		if p := getString(profile, "phoneNumber"); p != "" {
			phones = append(phones, p)
		}
		phoneVariants := make([]string, 0, len(phones)*2)
		for _, p := range phones {
			if p != "" {
				phoneVariants = append(phoneVariants, p)
				if len(p) >= 3 && p[:2] == "84" {
					phoneVariants = append(phoneVariants, "0"+p[2:])
				} else if len(p) >= 10 && p[0] == '0' {
					phoneVariants = append(phoneVariants, "84"+p[1:])
				}
			}
		}
		ids := []string{}
		for _, id := range []string{posId, fbId, uid} {
			if id != "" {
				ids = append(ids, id)
			}
		}
		var orCond []bson.M
		if len(ids) > 0 {
			orCond = append(orCond,
				bson.M{"customerId": bson.M{"$in": ids}},
				bson.M{"posData.customer.id": bson.M{"$in": ids}},
				bson.M{"posData.customer_id": bson.M{"$in": ids}},
			)
		}
		if len(phoneVariants) > 0 {
			orCond = append(orCond,
				bson.M{"billPhoneNumber": bson.M{"$in": phoneVariants}},
				bson.M{"posData.bill_phone_number": bson.M{"$in": phoneVariants}},
			)
		}
		if len(orCond) == 0 {
			return 0, ids, phoneVariants
		}
		matchFilter := bson.M{
			"ownerOrganizationId": orgID,
			"$and": []bson.M{
				{"$or": orCond},
				{"status": bson.M{"$nin": cancelledStatuses}},
				{"posData.status": bson.M{"$nin": cancelledStatuses}},
			},
		}
		n, _ := ordersColl.CountDocuments(ctx, matchFilter)
		return int(n), ids, phoneVariants
	}

	for _, stage := range stages {
		cursor, _ := crmColl.Find(ctx, bson.M{
			"ownerOrganizationId": orgID,
			"journeyStage":        stage,
		}, options.Find().SetProjection(bson.M{"unifiedId": 1, "sourceIds": 1, "profile": 1, "currentMetrics": 1, "orderCount": 1}).SetLimit(50))
		var docs []map[string]interface{}
		cursor.All(ctx, &docs)
		cursor.Close(ctx)

		if len(docs) == 0 {
			fmt.Printf("--- %s: 0 khách (bỏ qua)\n", stage)
			continue
		}

		okCount, mismatchCount := 0, 0
		var mismatchSamples []string
		for _, doc := range docs {
			orderCount := getOrderCountFromDoc(doc)
			n, _, _ := countOrdersInDB(doc)
			if n == orderCount {
				okCount++
			} else {
				mismatchCount++
				if len(mismatchSamples) < 3 {
					uid := getString(doc, "unifiedId")
					if len(uid) > 8 {
						uid = uid[:8]
					}
					mismatchSamples = append(mismatchSamples, fmt.Sprintf("%s(orderCount=%d,DB=%d)", uid, orderCount, n))
				}
			}
		}
		fmt.Printf("--- %s: %d khách — OK: %d, Mismatch: %d", stage, len(docs), okCount, mismatchCount)
		if len(mismatchSamples) > 0 {
			fmt.Printf(" — mẫu: %v", mismatchSamples)
		}
		fmt.Println()
	}

	// Chi tiết mẫu mismatch (nếu có) — lấy từ stage bất kỳ
	fmt.Println("\n--- Chi tiết mẫu (nếu có mismatch) ---")
	for _, stage := range stages {
		cursor, _ := crmColl.Find(ctx, bson.M{
			"ownerOrganizationId": orgID,
			"journeyStage":        stage,
		}, options.Find().SetProjection(bson.M{"unifiedId": 1, "sourceIds": 1, "profile": 1, "currentMetrics": 1, "orderCount": 1}).SetLimit(100))
		var docs []map[string]interface{}
		cursor.All(ctx, &docs)
		cursor.Close(ctx)

		detailCount := 0
		for _, doc := range docs {
			orderCount := getOrderCountFromDoc(doc)
			n, ids, phones := countOrdersInDB(doc)
			if n != orderCount && detailCount < 2 {
				uid := getString(doc, "unifiedId")
				fmt.Printf("  [%s] %s: orderCount=%d, đơn DB=%d | ids=%v phones=%v\n", stage, uid, orderCount, n, ids, phones)
				detailCount++
			}
		}
	}

	fmt.Println("\n✓ Hoàn thành")
}
