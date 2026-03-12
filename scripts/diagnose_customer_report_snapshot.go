// Script chẩn đoán report_snapshots customer: so sánh số dư từ snapshot vs CRM thực tế.
//
// Chạy: go run scripts/diagnose_customer_report_snapshot.go
// Hoặc: go run scripts/diagnose_customer_report_snapshot.go <ownerOrganizationId>
//
// So sánh:
// 1. Số dư từ report_snapshots (cộng dồn phát sinh in-out từ 2000 đến endMs)
// 2. Số dư thực tế từ crm_activity_history (GetLastSnapshotPerCustomerBeforeEndMs)
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

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

// extractJourneyStage lấy journeyStage từ metricsSnapshot (raw/layer1/layer2).
func extractJourneyStage(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	for _, layer := range []string{"layer1", "layer2", "raw"} {
		if sub, ok := m[layer].(map[string]interface{}); ok {
			if v, ok := sub["journeyStage"]; ok && v != nil {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func getStrFromNested(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if raw, ok := m["raw"]; ok {
		if r, ok := raw.(map[string]interface{}); ok {
			if v, ok := r[key]; ok && v != nil {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
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

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	snapColl := db.Collection("report_snapshots")
	actColl := db.Collection("crm_activity_history")

	var orgID primitive.ObjectID
	if len(os.Args) >= 2 {
		orgID, err = primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
		fmt.Printf("=== Chẩn đoán report_snapshots customer cho org: %s ===\n\n", os.Args[1])
	} else {
		var doc struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		err := snapColl.FindOne(ctx, bson.M{"reportKey": bson.M{"$regex": "^customer_"}}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1}))
		if err != nil {
			var crmDoc struct {
				OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
			}
			if db.Collection("crm_customers").FindOne(ctx, bson.M{}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&crmDoc) == nil {
				orgID = crmDoc.OwnerOrganizationID
			} else {
				log.Fatal("Không tìm thấy org. Chạy với: go run scripts/diagnose_customer_report_snapshot.go <ownerOrganizationId>")
			}
		} else {
			var snapDoc struct {
				OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
			}
			snapColl.FindOne(ctx, bson.M{"reportKey": bson.M{"$regex": "^customer_"}}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&snapDoc)
			orgID = snapDoc.OwnerOrganizationID
		}
		_ = doc
		fmt.Printf("=== Chẩn đoán report_snapshots customer cho org: %s ===\n\n", orgID.Hex())
	}

	if orgID.IsZero() {
		log.Fatal("Không xác định được orgID")
	}

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)
	endMs := endDate.UnixMilli()

	// 1. Số dư thực tế từ crm_activity_history (GetLastSnapshotPerCustomerBeforeEndMs)
	fmt.Println("--- 1. Số dư thực tế từ crm_activity_history (tại endMs) ---")
	pipe := []bson.M{
		{"$match": bson.M{
			"ownerOrganizationId":     orgID,
			"activityAt":              bson.M{"$lte": endMs},
			"metadata.metricsSnapshot": bson.M{"$exists": true},
		}},
		{"$sort": bson.M{"activityAt": -1}},
		{"$group": bson.M{
			"_id":             "$unifiedId",
			"metricsSnapshot": bson.M{"$first": "$metadata.metricsSnapshot"},
		}},
	}
	cursor, err := actColl.Aggregate(ctx, pipe)
	if err != nil {
		log.Printf("Lỗi aggregation activity: %v", err)
	} else {
		crmBalance := make(map[string]int64)
		for cursor.Next(ctx) {
			var doc struct {
				ID              string                 `bson:"_id"`
				MetricsSnapshot map[string]interface{} `bson:"metricsSnapshot"`
			}
			if cursor.Decode(&doc) != nil || doc.MetricsSnapshot == nil {
				continue
			}
			stage := extractJourneyStage(doc.MetricsSnapshot)
			if stage == "" {
				stage = "_unspecified"
			}
			crmBalance[stage]++
		}
		cursor.Close(ctx)
		fmt.Println("  journeyStage | count (CRM activity)")
		for _, stage := range []string{"visitor", "engaged", "first", "repeat", "vip", "inactive", "_unspecified"} {
			c := crmBalance[stage]
			fmt.Printf("  %-12s | %d\n", stage, c)
		}
		var total int64
		for _, c := range crmBalance {
			total += c
		}
		fmt.Printf("  TỔNG        | %d\n", total)
	}

	// 2. report_snapshots customer_* — có bao nhiêu, periodKey range
	fmt.Println("\n--- 2. report_snapshots customer_* — tổng quan ---")
	snapCount, _ := snapColl.CountDocuments(ctx, bson.M{
		"reportKey":            bson.M{"$regex": "^customer_"},
		"ownerOrganizationId": orgID,
	})
	fmt.Printf("  Tổng snapshot customer_*: %d\n", snapCount)

	var firstLast []struct {
		PeriodKey string `bson:"periodKey"`
		ReportKey string `bson:"reportKey"`
	}
	opts := options.Find().SetSort(bson.D{{Key: "periodKey", Value: 1}}).SetLimit(5)
	cursor, _ = snapColl.Find(ctx, bson.M{"reportKey": bson.M{"$regex": "^customer_"}, "ownerOrganizationId": orgID}, opts)
	cursor.All(ctx, &firstLast)
	cursor.Close(ctx)
	if len(firstLast) > 0 {
		fmt.Printf("  Mẫu periodKey đầu: %v\n", firstLast[0].PeriodKey)
	}
	opts = options.Find().SetSort(bson.D{{Key: "periodKey", Value: -1}}).SetLimit(5)
	cursor, _ = snapColl.Find(ctx, bson.M{"reportKey": bson.M{"$regex": "^customer_"}, "ownerOrganizationId": orgID}, opts)
	cursor.All(ctx, &firstLast)
	cursor.Close(ctx)
	if len(firstLast) > 0 {
		fmt.Printf("  Mẫu periodKey cuối: %v\n", firstLast[0].PeriodKey)
	}

	// 3. Cộng dồn phát sinh từ report_snapshots (layer1.journeyStage)
	fmt.Println("\n--- 3. Số dư từ cộng dồn report_snapshots (layer1.journeyStage in - out) ---")
	cursor, err = snapColl.Find(ctx, bson.M{
		"reportKey":            bson.M{"$regex": "^customer_"},
		"ownerOrganizationId": orgID,
		"periodKey":            bson.M{"$lte": now.Format("2006-01-02")},
	}, options.Find().SetSort(bson.D{{Key: "periodKey", Value: 1}}))
	if err != nil {
		log.Printf("Lỗi: %v", err)
	} else {
		journeyIn := make(map[string]int64)
		journeyOut := make(map[string]int64)
		count := 0
		for cursor.Next(ctx) {
			var doc struct {
				Metrics map[string]interface{} `bson:"metrics"`
			}
			if cursor.Decode(&doc) != nil || doc.Metrics == nil {
				continue
			}
			count++
			l1, ok := doc.Metrics["layer1"].(map[string]interface{})
			if !ok {
				continue
			}
			js, ok := l1["journeyStage"].(map[string]interface{})
			if !ok {
				continue
			}
			for stage, v := range js {
				if stage == "" {
					stage = "_unspecified"
				}
				if io, ok := v.(map[string]interface{}); ok {
					journeyIn[stage] += toInt64(io["in"])
					journeyOut[stage] += toInt64(io["out"])
				}
			}
		}
		cursor.Close(ctx)
		fmt.Printf("  Đã cộng dồn %d snapshots\n", count)
		fmt.Println("  journeyStage | in (tổng) | out (tổng) | balance (in-out)")
		snapBalance := make(map[string]int64)
		for _, stage := range []string{"visitor", "engaged", "first", "repeat", "vip", "inactive", "_unspecified", "_new"} {
			in := journeyIn[stage]
			out := journeyOut[stage]
			bal := in - out
			snapBalance[stage] = bal
			fmt.Printf("  %-12s | %9d | %9d | %d\n", stage, in, out, bal)
		}
		var totalSnap int64
		for _, b := range snapBalance {
			totalSnap += b
		}
		fmt.Printf("  TỔNG        |           |           | %d\n", totalSnap)
	}

	// 4. So sánh crm_customers trực tiếp (journeyStage)
	fmt.Println("\n--- 4. crm_customers — phân bố journeyStage (số thực tế hiện tại) ---")
	crmColl := db.Collection("crm_customers")
	journeyPipe := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"ownerOrganizationId": orgID}}},
		{{Key: "$group", Value: bson.M{"_id": bson.M{"$ifNull": []interface{}{"$journeyStage", "_unspecified"}}, "count": bson.M{"$sum": 1}}}},
	}
	cursor, _ = crmColl.Aggregate(ctx, journeyPipe)
	var crmJourney []struct {
		ID    string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	cursor.All(ctx, &crmJourney)
	cursor.Close(ctx)
	for _, r := range crmJourney {
		if r.ID == "" {
			r.ID = "_unspecified"
		}
		fmt.Printf("  %-12s | %d\n", r.ID, r.Count)
	}

	// 5. Kiểm tra snapshot có metrics đúng format không
	fmt.Println("\n--- 5. Mẫu 1 report_snapshot customer_daily (kiểm tra cấu trúc metrics) ---")
	var sample bson.M
	err = snapColl.FindOne(ctx, bson.M{
		"reportKey":            "customer_daily",
		"ownerOrganizationId": orgID,
	}, options.FindOne().SetSort(bson.D{{Key: "periodKey", Value: -1}})).Decode(&sample)
	if err == nil && sample["metrics"] != nil {
		m := toMap(sample["metrics"])
		fmt.Printf("  periodKey: %v\n", sample["periodKey"])
		if m != nil {
			if l1, ok := m["layer1"]; ok {
				l1m := toMap(l1)
				if l1m != nil {
					fmt.Printf("  layer1 keys: %v\n", getMapKeys(l1m))
					if js := toMap(l1m["journeyStage"]); js != nil {
						fmt.Printf("  layer1.journeyStage keys: %v\n", getMapKeys(js))
					}
				}
			}
		}
	} else {
		fmt.Println("  Không tìm thấy snapshot customer_daily")
	}

	fmt.Println("\n✓ Hoàn thành")
}

func toMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if m, ok := v.(bson.M); ok {
		return m
	}
	return nil
}

func getMapKeys(v interface{}) []string {
	var m map[string]interface{}
	switch x := v.(type) {
	case map[string]interface{}:
		m = x
	case bson.M:
		m = x
	default:
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
