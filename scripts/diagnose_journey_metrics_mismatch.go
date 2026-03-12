// Script chẩn đoán: tại sao currentMetrics (crm_customers) và metricsSnapshot (activity hành trình) chênh lệch.
//
// Chạy: go run scripts/diagnose_journey_metrics_mismatch.go [ownerOrganizationId] [--limit N]
//
// Phân tích:
// 1. So sánh currentMetrics trên crm_customers với metricsSnapshot của activity mới nhất
// 2. Phân loại: expected (có order/conv mới sau activity) vs unexpected (snapshot > current hoặc journeyStage khác)
// 3. Kiểm tra nguyên nhân: conversationIds thiếu, asOf logic, recalc chưa chạy
package main

import (
	"context"
	"flag"
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

// getFromNested lấy giá trị từ map nested (raw/layer1/layer2).
func getFromNested(m map[string]interface{}, keys ...string) interface{} {
	if m == nil {
		return nil
	}
	for i, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			return nil
		}
		if i == len(keys)-1 {
			return v
		}
		mm, ok := v.(map[string]interface{})
		if !ok {
			return nil
		}
		m = mm
	}
	return nil
}

func toInt(v interface{}) int {
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

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// getOrderCount lấy orderCount từ metrics (currentMetrics hoặc metricsSnapshot).
func getOrderCount(m map[string]interface{}) int {
	if v := getFromNested(m, "raw", "orderCount"); v != nil {
		return toInt(v)
	}
	return toInt(getFromNested(m, "layer1", "orderCount"))
}

// getTotalSpent lấy totalSpent từ raw.
func getTotalSpent(m map[string]interface{}) float64 {
	return toFloat64(getFromNested(m, "raw", "totalSpent"))
}

// getConversationCount lấy conversationCount từ raw.
func getConversationCount(m map[string]interface{}) int {
	return toInt(getFromNested(m, "raw", "conversationCount"))
}

// getJourneyStage lấy journeyStage từ layer1, layer2, hoặc raw.
func getJourneyStage(m map[string]interface{}) string {
	for _, layer := range []string{"layer1", "layer2", "raw"} {
		if v := getFromNested(m, layer, "journeyStage"); v != nil {
			return toString(v)
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

	limit := flag.Int("limit", 20, "Số khách mẫu chi tiết (0 = không in chi tiết)")
	flag.Parse()
	args := flag.Args()

	orgIDStr := "69a655f0088600c32e62f955"
	if len(args) >= 1 {
		orgIDStr = args[0]
	}
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	actColl := db.Collection("crm_activity_history")
	orderColl := db.Collection("pc_pos_orders")
	convColl := db.Collection("fb_conversations")

	fmt.Println("=== Chẩn đoán chênh lệch currentMetrics vs metricsSnapshot (hành trình khách hàng) ===\n")
	fmt.Printf("Org: %s | Limit chi tiết: %d\n\n", orgIDStr, *limit)

	// 1. Lấy last activity (có metricsSnapshot) per customer
	pipe := []bson.M{
		{"$match": bson.M{
			"ownerOrganizationId":     orgID,
			"metadata.metricsSnapshot": bson.M{"$exists": true, "$ne": nil},
		}},
		{"$sort": bson.M{"activityAt": -1}},
		{"$group": bson.M{
			"_id":             "$unifiedId",
			"activityAt":      bson.M{"$first": "$activityAt"},
			"activityType":    bson.M{"$first": "$activityType"},
			"metricsSnapshot": bson.M{"$first": "$metadata.metricsSnapshot"},
		}},
	}
	cursor, err := actColl.Aggregate(ctx, pipe)
	if err != nil {
		log.Fatalf("Aggregate activity: %v", err)
	}

	type lastActivity struct {
		UnifiedId       string                 `bson:"_id"`
		ActivityAt      int64                  `bson:"activityAt"`
		ActivityType    string                 `bson:"activityType"`
		MetricsSnapshot map[string]interface{} `bson:"metricsSnapshot"`
	}
	var lastActivities []lastActivity
	cursor.All(ctx, &lastActivities)
	cursor.Close(ctx)

	fmt.Printf("📊 Số khách có activity với metricsSnapshot: %d\n\n", len(lastActivities))

	// 2. Lấy currentMetrics từ crm_customers
	crmFilter := bson.M{"ownerOrganizationId": orgID, "unifiedId": bson.M{"$in": make([]string, 0)}}
	unifiedIds := make([]string, 0, len(lastActivities))
	for _, la := range lastActivities {
		unifiedIds = append(unifiedIds, la.UnifiedId)
	}
	crmFilter["unifiedId"] = bson.M{"$in": unifiedIds}

	cur, err := crmColl.Find(ctx, crmFilter, options.Find().SetProjection(bson.M{
		"unifiedId": 1, "sourceIds": 1, "primarySource": 1,
		"currentMetrics": 1, "orderCount": 1, "totalSpent": 1,
		"updatedAt": 1, "mergedAt": 1,
	}))
	if err != nil {
		log.Fatalf("Find crm: %v", err)
	}

	type crmDoc struct {
		UnifiedId      string                 `bson:"unifiedId"`
		SourceIds      struct{ Pos, Fb string } `bson:"sourceIds"`
		PrimarySource  string                 `bson:"primarySource"`
		CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
		OrderCount     int                    `bson:"orderCount"`
		TotalSpent     float64                `bson:"totalSpent"`
		UpdatedAt      int64                  `bson:"updatedAt"`
		MergedAt       int64                  `bson:"mergedAt"`
	}
	crmMap := make(map[string]*crmDoc)
	for cur.Next(ctx) {
		var d crmDoc
		if cur.Decode(&d) == nil {
			crmMap[d.UnifiedId] = &d
		}
	}
	cur.Close(ctx)

	// 3. So sánh và phân loại
	type mismatchCase struct {
		UnifiedId         string
		ActivityAt        int64
		ActivityType      string
		SnapshotOrderCount int
		CurrentOrderCount  int
		SnapshotTotalSpent float64
		CurrentTotalSpent  float64
		SnapshotConvCount  int
		CurrentConvCount   int
		SnapshotJourney    string
		CurrentJourney     string
		Kind               string // "expected" | "snapshot_higher" | "journey_mismatch" | "conv_mismatch"
		Detail             string
	}

	var mismatches []mismatchCase
	expectedHigher := 0 // current > snapshot (có order/conv mới sau activity — bình thường)

	for _, la := range lastActivities {
		crm := crmMap[la.UnifiedId]
		if crm == nil || crm.CurrentMetrics == nil {
			continue
		}
		snap := la.MetricsSnapshot
		current := crm.CurrentMetrics

		snapOC := getOrderCount(snap)
		currOC := getOrderCount(current)
		snapTS := getTotalSpent(snap)
		currTS := getTotalSpent(current)
		snapConv := getConversationCount(snap)
		currConv := getConversationCount(current)
		snapJS := getJourneyStage(snap)
		currJS := getJourneyStage(current)

		// Phân loại
		if currOC > snapOC || currTS > snapTS || currConv > snapConv {
			// current cao hơn — có thể có order/conv mới sau activityAt (expected)
			expectedHigher++
			continue
		}
		if currOC < snapOC || currTS < snapTS {
			// snapshot cao hơn current — BUG, không hợp lý
			mismatches = append(mismatches, mismatchCase{
				UnifiedId:           la.UnifiedId,
				ActivityAt:          la.ActivityAt,
				ActivityType:        la.ActivityType,
				SnapshotOrderCount:  snapOC,
				CurrentOrderCount:   currOC,
				SnapshotTotalSpent:  snapTS,
				CurrentTotalSpent:   currTS,
				SnapshotConvCount:   snapConv,
				CurrentConvCount:    currConv,
				SnapshotJourney:     snapJS,
				CurrentJourney:      currJS,
				Kind:                "snapshot_higher",
				Detail:              "Activity snapshot > currentMetrics — cần điều tra",
			})
			continue
		}
		if snapJS != currJS {
			mismatches = append(mismatches, mismatchCase{
				UnifiedId:           la.UnifiedId,
				ActivityAt:          la.ActivityAt,
				ActivityType:        la.ActivityType,
				SnapshotOrderCount:  snapOC,
				CurrentOrderCount:   currOC,
				SnapshotTotalSpent:  snapTS,
				CurrentTotalSpent:   currTS,
				SnapshotConvCount:   snapConv,
				CurrentConvCount:    currConv,
				SnapshotJourney:     snapJS,
				CurrentJourney:      currJS,
				Kind:                "journey_mismatch",
				Detail:              fmt.Sprintf("journeyStage: snapshot=%s vs current=%s", snapJS, currJS),
			})
			continue
		}
		if snapConv != currConv {
			mismatches = append(mismatches, mismatchCase{
				UnifiedId:           la.UnifiedId,
				ActivityAt:          la.ActivityAt,
				ActivityType:        la.ActivityType,
				SnapshotOrderCount:  snapOC,
				CurrentOrderCount:   currOC,
				SnapshotTotalSpent:  snapTS,
				CurrentTotalSpent:   currTS,
				SnapshotConvCount:   snapConv,
				CurrentConvCount:    currConv,
				SnapshotJourney:     snapJS,
				CurrentJourney:      currJS,
				Kind:                "conv_mismatch",
				Detail:              fmt.Sprintf("conversationCount: snapshot=%d vs current=%d", snapConv, currConv),
			})
		}
	}

	fmt.Printf("📈 Thống kê:\n")
	fmt.Printf("   - Số khách có current > snapshot (có order/conv mới — bình thường): %d\n", expectedHigher)
	fmt.Printf("   - Số khách chênh lệch bất thường: %d\n\n", len(mismatches))

	byKind := make(map[string]int)
	for _, m := range mismatches {
		byKind[m.Kind]++
	}
	fmt.Printf("📋 Phân loại chênh lệch bất thường:\n")
	for k, n := range byKind {
		fmt.Printf("   - %s: %d\n", k, n)
	}
	fmt.Println()

	if *limit > 0 && len(mismatches) > 0 {
		show := mismatches
		if len(show) > *limit {
			show = show[:*limit]
		}
		fmt.Printf("--- Chi tiết %d mẫu ---\n\n", len(show))

		for i, m := range show {
			fmt.Printf("【%d】 %s (kind=%s)\n", i+1, m.UnifiedId, m.Kind)
			fmt.Printf("    Activity: %s @ %d (%s)\n", m.ActivityType, m.ActivityAt, time.UnixMilli(m.ActivityAt).Format("2006-01-02 15:04"))
			fmt.Printf("    OrderCount:   snapshot=%d vs current=%d\n", m.SnapshotOrderCount, m.CurrentOrderCount)
			fmt.Printf("    TotalSpent:   snapshot=%.0f vs current=%.0f\n", m.SnapshotTotalSpent, m.CurrentTotalSpent)
			fmt.Printf("    ConvCount:     snapshot=%d vs current=%d\n", m.SnapshotConvCount, m.CurrentConvCount)
			fmt.Printf("    JourneyStage:  snapshot=%s vs current=%s\n", m.SnapshotJourney, m.CurrentJourney)
			fmt.Printf("    %s\n", m.Detail)

			crm := crmMap[m.UnifiedId]
			if crm != nil {
				// Kiểm tra: có order mới sau activityAt không?
				nOrdersAfter, _ := orderColl.CountDocuments(ctx, bson.M{
					"ownerOrganizationId": orgID,
					"$or": []bson.M{
						{"customerId": m.UnifiedId},
						{"posData.customer.id": m.UnifiedId},
						{"posData.customer_id": m.UnifiedId},
						{"posData.page_customer.id": m.UnifiedId},
					},
					"$expr": bson.M{"$gt": bson.A{
						bson.M{"$ifNull": bson.A{"$insertedAt", bson.M{"$ifNull": bson.A{"$posCreatedAt", int64(0)}}}},
						m.ActivityAt,
					}},
				})
				// Có conv match customer không?
				ids := []string{m.UnifiedId, crm.SourceIds.Pos, crm.SourceIds.Fb}
				var convOr []bson.M
				for _, id := range ids {
					if id != "" {
						convOr = append(convOr,
							bson.M{"customerId": id},
							bson.M{"panCakeData.customer_id": id},
							bson.M{"panCakeData.customer.id": id},
							bson.M{"panCakeData.page_customer.id": id},
						)
					}
				}
				nConv, _ := convColl.CountDocuments(ctx, bson.M{
					"ownerOrganizationId": orgID,
					"$or": convOr,
				})
				fmt.Printf("    DB: orders sau activity=%d | convs match customer=%d\n", nOrdersAfter, nConv)
			}
			fmt.Println()
		}
	}

	// 4. Kiểm tra activity thiếu metricsSnapshot
	noSnapshot, _ := actColl.CountDocuments(ctx, bson.M{
		"ownerOrganizationId": orgID,
		"$or": []bson.M{
			{"metadata.metricsSnapshot": bson.M{"$exists": false}},
			{"metadata.metricsSnapshot": nil},
		},
	})
	withSnapshot, _ := actColl.CountDocuments(ctx, bson.M{
		"ownerOrganizationId":     orgID,
		"metadata.metricsSnapshot": bson.M{"$exists": true, "$ne": nil},
	})
	fmt.Printf("📌 Activity: có metricsSnapshot=%d | thiếu=%d (thiếu bị bỏ qua trong so sánh)\n", withSnapshot, noSnapshot)

	fmt.Println("\n✓ Hoàn thành")
}
