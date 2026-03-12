// Script chẩn đoán tại sao có nhiều khách "new" trong crm_customers.
// Phân tích: valueTier, journeyStage, orderCount, totalSpent, primarySource.
//
// Chạy: go run scripts/diagnose_new_customers.go
// Hoặc chỉ 1 org: go run scripts/diagnose_new_customers.go <ownerOrganizationId>
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

	// Filter: toàn bộ hoặc chỉ 1 org
	var filter bson.M
	if len(os.Args) >= 2 {
		orgID, err := primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
		filter = bson.M{"ownerOrganizationId": orgID}
		fmt.Printf("=== Phân tích CRM cho org: %s ===\n\n", os.Args[1])
	} else {
		filter = bson.M{}
		fmt.Println("=== Phân tích CRM (toàn bộ org) ===\n")
	}

	// 1. Tổng số khách
	total, _ := crmColl.CountDocuments(ctx, filter)
	fmt.Printf("Tổng crm_customers: %d\n\n", total)
	if total == 0 {
		// Liệt kê org có dữ liệu
		orgPipe := mongo.Pipeline{
			{{Key: "$group", Value: bson.M{"_id": "$ownerOrganizationId", "count": bson.M{"$sum": 1}}}},
			{{Key: "$sort", Value: bson.M{"count": -1}}},
			{{Key: "$limit", Value: 5}},
		}
		cur, _ := crmColl.Aggregate(ctx, orgPipe)
		var orgResults []bson.M
		cur.All(ctx, &orgResults)
		cur.Close(ctx)
		if len(orgResults) > 0 {
			fmt.Println("Các org có dữ liệu (top 5):")
			for _, r := range orgResults {
				fmt.Printf("  %v: %v khách\n", r["_id"], r["count"])
			}
		}
		fmt.Println("\nKhông có dữ liệu cho filter đã chọn.")
		return
	}

	// 2. Phân bố valueTier
	fmt.Println("--- Phân bố valueTier (vip|high|medium|low|new) ---")
	valueTierPipe := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": "$valueTier", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
	}
	cur, _ := crmColl.Aggregate(ctx, valueTierPipe)
	var valueTierResults []bson.M
	cur.All(ctx, &valueTierResults)
	cur.Close(ctx)
	for _, r := range valueTierResults {
		tier := r["_id"]
		if tier == nil || tier == "" {
			tier = "(rỗng)"
		}
		fmt.Printf("  %v: %v\n", tier, r["count"])
	}

	// 3. Phân bố journeyStage
	fmt.Println("\n--- Phân bố journeyStage (visitor|engaged|first|repeat|vip|inactive) ---")
	journeyPipe := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": "$journeyStage", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
	}
	cur, _ = crmColl.Aggregate(ctx, journeyPipe)
	var journeyResults []bson.M
	cur.All(ctx, &journeyResults)
	cur.Close(ctx)
	for _, r := range journeyResults {
		stage := r["_id"]
		if stage == nil || stage == "" {
			stage = "(rỗng)"
		}
		fmt.Printf("  %v: %v\n", stage, r["count"])
	}

	// 4. Phân bố orderCount
	fmt.Println("\n--- Phân bố orderCount ---")
	orderCountPipe := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": "$orderCount", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	cur, _ = crmColl.Aggregate(ctx, orderCountPipe)
	var orderCountResults []bson.M
	cur.All(ctx, &orderCountResults)
	cur.Close(ctx)
	for _, r := range orderCountResults {
		oc := r["_id"]
		if oc == nil {
			oc = "nil"
		}
		fmt.Printf("  orderCount=%v: %v\n", oc, r["count"])
	}

	// 5. Phân bố primarySource (pos vs fb)
	fmt.Println("\n--- Phân bố primarySource ---")
	sourcePipe := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{"_id": "$primarySource", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
	}
	cur, _ = crmColl.Aggregate(ctx, sourcePipe)
	var sourceResults []bson.M
	cur.All(ctx, &sourceResults)
	cur.Close(ctx)
	for _, r := range sourceResults {
		src := r["_id"]
		if src == nil || src == "" {
			src = "(rỗng)"
		}
		fmt.Printf("  %v: %v\n", src, r["count"])
	}

	// 6. Khách valueTier=new: breakdown theo orderCount
	fmt.Println("\n--- Khách valueTier=new: breakdown theo orderCount ---")
	newFilter := bson.M{"valueTier": "new"}
	for k, v := range filter {
		newFilter[k] = v
	}
	newOrderPipe := mongo.Pipeline{
		{{Key: "$match", Value: newFilter}},
		{{Key: "$group", Value: bson.M{"_id": "$orderCount", "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}
	cur, _ = crmColl.Aggregate(ctx, newOrderPipe)
	var newOrderResults []bson.M
	cur.All(ctx, &newOrderResults)
	cur.Close(ctx)
	for _, r := range newOrderResults {
		oc := r["_id"]
		if oc == nil {
			oc = "nil"
		}
		fmt.Printf("  orderCount=%v: %v\n", oc, r["count"])
	}

	// 7. Khách valueTier=new: breakdown theo totalSpent
	fmt.Println("\n--- Khách valueTier=new: breakdown theo khoảng totalSpent (VNĐ) ---")
	newSpentPipe := mongo.Pipeline{
		{{Key: "$match", Value: newFilter}},
		{{Key: "$bucket", Value: bson.M{
			"groupBy": "$totalSpent",
			"boundaries": []float64{0, 100000, 500000, 1000000, 5000000, 20000000, 50000000, 1e12},
			"default": "other",
			"output": bson.M{"count": bson.M{"$sum": 1}},
		}}},
	}
	cur, _ = crmColl.Aggregate(ctx, newSpentPipe)
	var newSpentResults []bson.M
	cur.All(ctx, &newSpentResults)
	cur.Close(ctx)
	for _, r := range newSpentResults {
		fmt.Printf("  [%v]: %v\n", r["_id"], r["count"])
	}

	// 8. Mẫu khách valueTier=new có orderCount > 1 (bất thường?)
	fmt.Println("\n--- Mẫu khách valueTier=new nhưng orderCount >= 2 (bất thường?) ---")
	abnormalFilter := bson.M{"valueTier": "new", "orderCount": bson.M{"$gte": 2}}
	for k, v := range filter {
		abnormalFilter[k] = v
	}
	var samples []bson.M
	cur, _ = crmColl.Find(ctx, abnormalFilter, options.Find().SetLimit(5))
	cur.All(ctx, &samples)
	cur.Close(ctx)
	if len(samples) > 0 {
		for i, s := range samples {
			fmt.Printf("  [%d] unifiedId=%v orderCount=%v totalSpent=%v journeyStage=%v\n",
				i+1, s["unifiedId"], s["orderCount"], s["totalSpent"], s["journeyStage"])
		}
	} else {
		fmt.Println("  Không có.")
	}

	// 9. Khách valueTier=new: có currentMetrics không?
	fmt.Println("\n--- Khách valueTier=new: có currentMetrics không? ---")
	withMetricsFilter := bson.M{"valueTier": "new", "currentMetrics": bson.M{"$exists": true, "$ne": nil}}
	withoutMetricsFilter := bson.M{"valueTier": "new", "$or": []bson.M{
		{"currentMetrics": bson.M{"$exists": false}},
		{"currentMetrics": nil},
	}}
	for k, v := range filter {
		withMetricsFilter[k] = v
		withoutMetricsFilter[k] = v
	}
	withMetrics, _ := crmColl.CountDocuments(ctx, withMetricsFilter)
	withoutMetrics, _ := crmColl.CountDocuments(ctx, withoutMetricsFilter)
	fmt.Printf("  Có currentMetrics: %d\n", withMetrics)
	fmt.Printf("  Không có currentMetrics: %d\n", withoutMetrics)

	// 10. Khách journeyStage=first: có phải đa số đúng là orderCount=1?
	fmt.Println("\n--- Khách journeyStage=first: orderCount ---")
	firstFilter1 := bson.M{"journeyStage": "first", "orderCount": 1}
	firstFilter2 := bson.M{"journeyStage": "first", "orderCount": bson.M{"$ne": 1}}
	for k, v := range filter {
		firstFilter1[k] = v
		firstFilter2[k] = v
	}
	firstCount, _ := crmColl.CountDocuments(ctx, firstFilter1)
	firstOther, _ := crmColl.CountDocuments(ctx, firstFilter2)
	fmt.Printf("  orderCount=1: %d\n", firstCount)
	fmt.Printf("  orderCount!=1: %d\n", firstOther)

	// 11. Khách orderCount=0: valueTier và journeyStage
	fmt.Println("\n--- Khách orderCount=0: valueTier và journeyStage ---")
	zeroOrderMatch := bson.M{"orderCount": 0}
	for k, v := range filter {
		zeroOrderMatch[k] = v
	}
	zeroOrderPipe := mongo.Pipeline{
		{{Key: "$match", Value: zeroOrderMatch}},
		{{Key: "$group", Value: bson.M{"_id": bson.M{"valueTier": "$valueTier", "journeyStage": "$journeyStage"}, "count": bson.M{"$sum": 1}}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
		{{Key: "$limit", Value: 10}},
	}
	cur, _ = crmColl.Aggregate(ctx, zeroOrderPipe)
	var zeroOrderResults []bson.M
	cur.All(ctx, &zeroOrderResults)
	cur.Close(ctx)
	for _, r := range zeroOrderResults {
		id := r["_id"].(bson.M)
		fmt.Printf("  valueTier=%v journeyStage=%v: %v\n", id["valueTier"], id["journeyStage"], r["count"])
	}

	fmt.Println("\n✓ Hoàn thành")
}
