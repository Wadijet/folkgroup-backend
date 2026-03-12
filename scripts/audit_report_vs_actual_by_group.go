// Script kiểm tra chênh lệch số liệu report vs thực tế theo từng nhóm khách hàng.
//
// Chạy: go run scripts/audit_report_vs_actual_by_group.go [ownerOrganizationId]
//
// So sánh 3 nguồn:
// 1. crm_customers — số thực tế hiện tại (journeyStage / currentMetrics)
// 2. crm_activity_history — snapshot cuối mỗi khách tại endMs (point-in-time)
// 3. report_snapshots — số dư cộng dồn từ phát sinh (in - out) theo period
//
// Giải thích chênh lệch:
// - Report dùng crm_activity_history; nếu activity thiếu metricsSnapshot → report không đếm
// - crm_customers có thể mới hơn activity (recalc/merge chưa log activity)
// - report_snapshots thiếu period → chưa chạy report_dirty
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

func getJourneyFromMetrics(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	for _, layer := range []string{"layer1", "layer2", "raw"} {
		if sub, ok := m[layer].(map[string]interface{}); ok && sub != nil {
			if v, ok := sub["journeyStage"]; ok && v != nil {
				if s, ok := v.(string); ok && s != "" {
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
	crmColl := db.Collection("crm_customers")
	actColl := db.Collection("crm_activity_history")
	snapColl := db.Collection("report_snapshots")

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
			log.Fatal("Không tìm thấy org. Chạy với: go run scripts/audit_report_vs_actual_by_group.go <ownerOrganizationId>")
		}
	}

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)
	endMs := endDate.UnixMilli()

	stages := []string{"visitor", "engaged", "first", "repeat", "vip", "inactive", "_unspecified"}

	fmt.Println("=== Kiểm tra chênh lệch số liệu report vs thực tế theo nhóm khách hàng ===")
	fmt.Printf("Org: %s | endMs: %d (%s)\n\n", orgID.Hex(), endMs, endDate.Format("2006-01-02 15:04"))

	// 1. crm_customers — journeyStage (top-level) hoặc currentMetrics.layer1.journeyStage
	fmt.Println("--- 1. crm_customers (số thực tế hiện tại) ---")
	crmCounts := make(map[string]int64)
	cursor, err := crmColl.Find(ctx, bson.M{"ownerOrganizationId": orgID},
		options.Find().SetProjection(bson.M{"journeyStage": 1, "currentMetrics": 1, "orderCount": 1, "hasConversation": 1}))
	if err != nil {
		log.Printf("Lỗi: %v", err)
	} else {
		for cursor.Next(ctx) {
			var doc struct {
				JourneyStage   string                 `bson:"journeyStage"`
				CurrentMetrics map[string]interface{} `bson:"currentMetrics"`
				OrderCount     int                    `bson:"orderCount"`
				HasConversation bool                  `bson:"hasConversation"`
			}
			if cursor.Decode(&doc) != nil {
				continue
			}
			stage := doc.JourneyStage
			if stage == "" {
				stage = getJourneyFromMetrics(doc.CurrentMetrics)
			}
			if stage == "" {
				// Fallback: derive từ orderCount + hasConversation
				if doc.OrderCount == 0 {
					if doc.HasConversation {
						stage = "engaged"
					} else {
						stage = "visitor"
					}
				} else {
					stage = "_unspecified"
				}
			}
			if stage == "" {
				stage = "_unspecified"
			}
			crmCounts[stage]++
		}
		cursor.Close(ctx)
	}

	var crmTotal int64
	for _, s := range stages {
		c := crmCounts[s]
		crmTotal += c
		fmt.Printf("  %-12s | %d\n", s, c)
	}
	fmt.Printf("  %-12s | %d\n", "TỔNG", crmTotal)

	// 2. crm_activity_history — last metricsSnapshot per customer (point-in-time tại endMs)
	fmt.Println("\n--- 2. crm_activity_history (snapshot cuối mỗi khách trước endMs) ---")
	pipe := []bson.M{
		{"$match": bson.M{
			"ownerOrganizationId":      orgID,
			"activityAt":               bson.M{"$lte": endMs},
			"metadata.metricsSnapshot": bson.M{"$exists": true, "$ne": nil},
		}},
		{"$sort": bson.M{"activityAt": -1}},
		{"$group": bson.M{
			"_id":             "$unifiedId",
			"metricsSnapshot": bson.M{"$first": "$metadata.metricsSnapshot"},
		}},
	}
	cursor, err = actColl.Aggregate(ctx, pipe)
	if err != nil {
		log.Printf("Lỗi aggregation: %v", err)
	} else {
		actCounts := make(map[string]int64)
		for cursor.Next(ctx) {
			var doc struct {
				ID              string                 `bson:"_id"`
				MetricsSnapshot map[string]interface{} `bson:"metricsSnapshot"`
			}
			if cursor.Decode(&doc) != nil || doc.MetricsSnapshot == nil {
				continue
			}
			stage := getJourneyFromMetrics(doc.MetricsSnapshot)
			if stage == "" {
				stage = "_unspecified"
			}
			actCounts[stage]++
		}
		cursor.Close(ctx)

		var actTotal int64
		for _, s := range stages {
			c := actCounts[s]
			actTotal += c
			fmt.Printf("  %-12s | %d\n", s, c)
		}
		fmt.Printf("  %-12s | %d\n", "TỔNG", actTotal)

		// 3. report_snapshots — cộng dồn phát sinh (in - out) từ customer_daily
		fmt.Println("\n--- 3. report_snapshots (số dư cộng dồn từ phát sinh customer_daily) ---")
		snapCounts := make(map[string]int64)
		cursor, err = snapColl.Find(ctx, bson.M{
			"reportKey":            "customer_daily",
			"ownerOrganizationId": orgID,
			"periodKey":           bson.M{"$lte": now.Format("2006-01-02")},
		}, options.Find().SetSort(bson.D{{Key: "periodKey", Value: 1}}))
		if err != nil {
			log.Printf("Lỗi: %v", err)
		} else {
			journeyIn := make(map[string]int64)
			journeyOut := make(map[string]int64)
			nSnap := 0
			for cursor.Next(ctx) {
				var doc struct {
					Metrics map[string]interface{} `bson:"metrics"`
				}
				if cursor.Decode(&doc) != nil || doc.Metrics == nil {
					continue
				}
				nSnap++
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

			for _, s := range stages {
				snapCounts[s] = journeyIn[s] - journeyOut[s]
			}
			var snapTotal int64
			for _, s := range stages {
				c := snapCounts[s]
				snapTotal += c
				fmt.Printf("  %-12s | %d (in=%d out=%d)\n", s, c, journeyIn[s], journeyOut[s])
			}
			fmt.Printf("  %-12s | %d\n", "TỔNG", snapTotal)
			fmt.Printf("  (Đã cộng dồn %d snapshots customer_daily)\n", nSnap)
		}

		// 4. Bảng so sánh và phân tích
		fmt.Println("\n--- 4. Bảng so sánh và chênh lệch ---")
		fmt.Println("  Nhóm       | crm_customers | crm_activity | report_snap | CRM-Act | Act-Snap")
		fmt.Println("  -----------|---------------|--------------|--------------|---------|---------")
		for _, s := range stages {
			crm := crmCounts[s]
			act := actCounts[s]
			snap := snapCounts[s]
			diffCRMAct := crm - act
			diffActSnap := act - snap
			fmt.Printf("  %-11s | %13d | %12d | %12d | %7d | %7d\n",
				s, crm, act, snap, diffCRMAct, diffActSnap)
		}
		var snapTotal int64
		for _, c := range snapCounts {
			snapTotal += c
		}
		fmt.Printf("  %-11s | %13d | %12d | %12d |\n", "TỔNG", crmTotal, actTotal, snapTotal)

		// 5. Phân tích nguyên nhân
		fmt.Println("\n--- 5. Phân tích nguyên nhân chênh lệch ---")
		// Đếm: khách có trong crm_customers nhưng không có activity với metricsSnapshot
		actUnifiedIds := make(map[string]bool)
		cursor, _ = actColl.Aggregate(ctx, pipe)
		for cursor.Next(ctx) {
			var d struct {
				ID string `bson:"_id"`
			}
			if cursor.Decode(&d) == nil {
				actUnifiedIds[d.ID] = true
			}
		}
		cursor.Close(ctx)

		crmOnlyCount := 0
		cursor, _ = crmColl.Find(ctx, bson.M{"ownerOrganizationId": orgID}, options.Find().SetProjection(bson.M{"unifiedId": 1}))
		for cursor.Next(ctx) {
			var d struct {
				UnifiedId string `bson:"unifiedId"`
			}
			if cursor.Decode(&d) == nil && !actUnifiedIds[d.UnifiedId] {
				crmOnlyCount++
			}
		}
		cursor.Close(ctx)

		fmt.Printf("  - Khách có trong crm_customers nhưng KHÔNG có activity metricsSnapshot: %d\n", crmOnlyCount)
		fmt.Printf("    → Có thể do: recalc/merge chưa log activity, hoặc khách mới chưa có sự kiện\n")
		fmt.Printf("  - Report dùng crm_activity_history; nếu thiếu activity → report không đếm khách đó\n")
		fmt.Printf("  - report_snapshots thiếu period → chạy report_dirty worker để cập nhật\n")
	}

	fmt.Println("\n✓ Hoàn thành")
}
