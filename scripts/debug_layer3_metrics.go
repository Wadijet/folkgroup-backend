// Script debug Layer 3 metrics — kiểm tra crm_customers.currentMetrics, report_snapshots, activity history.
// Chạy: go run scripts/debug_layer3_metrics.go
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
	tryPaths := []string{".env", "api/.env", "config/env/development.env"}
	cwd, _ := os.Getwd()
	for _, p := range tryPaths {
		full := filepath.Join(cwd, p)
		if _, err := os.Stat(full); err == nil {
			_ = godotenv.Load(full)
			return
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(cwd), p)); err == nil {
			_ = godotenv.Load(filepath.Join(filepath.Dir(cwd), p))
			return
		}
	}
}

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionUri")
	}
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
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	fmt.Println("=== DEBUG LAYER 3 METRICS ===\n")

	// 1. crm_customers: có currentMetrics.layer3 không?
	crmColl := db.Collection("crm_customers")
	totalCrm, _ := crmColl.CountDocuments(ctx, bson.M{})
	withCurrentMetrics, _ := crmColl.CountDocuments(ctx, bson.M{"currentMetrics": bson.M{"$exists": true, "$ne": nil}})
	withLayer3, _ := crmColl.CountDocuments(ctx, bson.M{"currentMetrics.layer3": bson.M{"$exists": true, "$ne": nil}})
	withRaw, _ := crmColl.CountDocuments(ctx, bson.M{"currentMetrics.raw": bson.M{"$exists": true}})

	fmt.Printf("1. crm_customers:\n")
	fmt.Printf("   - Tổng: %d\n", totalCrm)
	fmt.Printf("   - Có currentMetrics: %d\n", withCurrentMetrics)
	fmt.Printf("   - Có currentMetrics.raw: %d\n", withRaw)
	fmt.Printf("   - Có currentMetrics.layer3: %d\n", withLayer3)

	// Mẫu 3 khách có orderCount>=1 xem currentMetrics
	var sampleCrm []bson.M
	cur, _ := crmColl.Find(ctx, bson.M{"orderCount": bson.M{"$gte": 1}}, options.Find().SetLimit(3).SetProjection(bson.M{
		"unifiedId": 1, "profile.name": 1, "orderCount": 1, "journeyStage": 1, "valueTier": 1, "lifecycleStage": 1,
		"currentMetrics": 1,
	}))
	_ = cur.All(ctx, &sampleCrm)
	fmt.Printf("\n   Mẫu 3 khách có đơn (orderCount>=1):\n")
	for i, c := range sampleCrm {
		name := "N/A"
		if p, ok := c["profile"].(map[string]interface{}); ok {
			if n, ok := p["name"].(string); ok {
				name = n
			}
		}
		cm, hasCm := c["currentMetrics"]
		hasL3 := false
		if cm != nil {
			if cmap, ok := cm.(map[string]interface{}); ok {
				_, hasL3 = cmap["layer3"]
			}
		}
		fmt.Printf("   [%d] %s | unifiedId=%v | orderCount=%v | journey=%v | valueTier=%v | currentMetrics=%v | layer3=%v\n",
			i+1, name, c["unifiedId"], c["orderCount"], c["journeyStage"], c["valueTier"], hasCm, hasL3)
	}

	// 2. crm_activity_history: có metadata.metricsSnapshot (raw, layer3)?
	actColl := db.Collection("crm_activity_history")
	withMetricsSnapshot, _ := actColl.CountDocuments(ctx, bson.M{"metadata.metricsSnapshot": bson.M{"$exists": true}})
	withMetricsRaw, _ := actColl.CountDocuments(ctx, bson.M{"metadata.metricsSnapshot.raw": bson.M{"$exists": true}})
	withMetricsLayer3, _ := actColl.CountDocuments(ctx, bson.M{"metadata.metricsSnapshot.layer3": bson.M{"$exists": true}})

	fmt.Printf("\n2. crm_activity_history:\n")
	fmt.Printf("   - Có metadata.metricsSnapshot: %d\n", withMetricsSnapshot)
	fmt.Printf("   - Có metricsSnapshot.raw: %d\n", withMetricsRaw)
	fmt.Printf("   - Có metricsSnapshot.layer3: %d\n", withMetricsLayer3)

	// 3. report_snapshots: có firstLayer3, repeatLayer3, vipLayer3, inactiveLayer3?
	reportColl := db.Collection("report_snapshots")
	var reportSample bson.M
	err = reportColl.FindOne(ctx, bson.M{"reportKey": "customer_daily"}).Decode(&reportSample)
	if err == nil && reportSample != nil {
		metrics, _ := reportSample["metrics"].(map[string]interface{})
		fmt.Printf("\n3. report_snapshots (customer_daily mẫu):\n")
		if metrics != nil {
			hasFirst := metrics["firstLayer3"] != nil
			hasRepeat := metrics["repeatLayer3"] != nil
			hasVip := metrics["vipLayer3"] != nil
			hasInactive := metrics["inactiveLayer3"] != nil
			fmt.Printf("   - firstLayer3: %v\n", hasFirst)
			fmt.Printf("   - repeatLayer3: %v\n", hasRepeat)
			fmt.Printf("   - vipLayer3: %v\n", hasVip)
			fmt.Printf("   - inactiveLayer3: %v\n", hasInactive)
		} else {
			fmt.Printf("   - metrics nil\n")
		}
	} else {
		fmt.Printf("\n3. report_snapshots: Không tìm thấy customer_daily\n")
	}

	fmt.Println("\n=== KẾT LUẬN ===")
	if withLayer3 == 0 && withCurrentMetrics > 0 {
		fmt.Println("⚠ crm_customers có currentMetrics nhưng KHÔNG có layer3. BuildCurrentMetricsFromOrderAndConv/buildMetricsSnapshot có thể thiếu layer3.")
	}
	if withMetricsLayer3 == 0 && withMetricsRaw > 0 {
		fmt.Println("⚠ Activity có metricsSnapshot.raw nhưng KHÔNG có layer3. Snapshot khi log activity có thể thiếu layer3.")
	}
}
