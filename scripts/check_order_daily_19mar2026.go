// Script kiểm tra order_daily ngày 19/3/2026: đếm đơn và doanh thu theo status.
// So sánh: TẤT CẢ đơn vs ĐÃ LOẠI TRỪ (status 6, 7) như logic report engine.
//
// Chạy: go run scripts/check_order_daily_19mar2026.go [ownerOrganizationId]
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

const reportTimezone = "Asia/Ho_Chi_Minh"
const periodKey = "2026-03-19"

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
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH (hoặc MONGODB_DBNAME)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	ordersColl := db.Collection("pc_pos_orders")

	// Khoảng thời gian 19/3/2026 theo Asia/Ho_Chi_Minh (giống report engine)
	loc, err := time.LoadLocation(reportTimezone)
	if err != nil {
		log.Fatalf("Load timezone: %v", err)
	}
	t, err := time.ParseInLocation("2006-01-02", periodKey, loc)
	if err != nil {
		log.Fatalf("Parse periodKey: %v", err)
	}
	startSec := t.Unix()
	endSec := t.AddDate(0, 0, 1).Unix() - 1
	timeFrom := startSec * 1000
	timeTo := endSec*1000 + 999

	// Lấy ownerOrganizationId — nếu có arg thì dùng, không thì lấy từ DB
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
		if ordersColl.FindOne(ctx, bson.M{}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&doc) == nil {
			orgID = doc.OwnerOrganizationID
		} else {
			log.Fatal("Chạy với: go run scripts/check_order_daily_19mar2026.go <ownerOrganizationId>")
		}
	}

	fmt.Printf("=== Kiểm tra order_daily ngày %s — org: %s ===\n", periodKey, orgID.Hex())
	fmt.Printf("Khoảng thời gian: posCreatedAt từ %d đến %d (ms)\n\n", timeFrom, timeTo)

	// Filter cơ bản: org + thời gian (posCreatedAt ms)
	baseFilter := bson.M{
		"ownerOrganizationId": orgID,
		"posCreatedAt":        bson.M{"$gte": timeFrom, "$lte": timeTo},
	}

	// 1. Tổng đơn TẤT CẢ (không loại trừ)
	totalAll, totalAmountAll := aggregateOrder(ctx, ordersColl, baseFilter)
	fmt.Printf("1. TẤT CẢ đơn (không loại trừ):\n   Số lượng: %d\n   Doanh thu: %.0f\n\n", totalAll, totalAmountAll)

	// 2. Đơn ĐÃ LOẠI TRỪ (posData.status $nin [6,7]) — giống report engine
	excludeFilter := bson.M{}
	for k, v := range baseFilter {
		excludeFilter[k] = v
	}
	excludeFilter["posData.status"] = bson.M{"$nin": []int{6, 7}}
	totalExcluded, totalAmountExcluded := aggregateOrder(ctx, ordersColl, excludeFilter)
	fmt.Printf("2. ĐÃ LOẠI TRỪ status 6,7 (posData.status $nin [6,7]):\n   Số lượng: %d\n   Doanh thu: %.0f\n\n", totalExcluded, totalAmountExcluded)

	// 3. Phân bố theo status
	fmt.Println("3. Phân bố theo posData.status:")
	pipeline := []bson.M{
		{"$match": baseFilter},
		{"$group": bson.M{
			"_id":   bson.M{"$ifNull": []interface{}{"$posData.status", -1}},
			"count": bson.M{"$sum": 1},
			"amount": bson.M{"$sum": bson.M{"$ifNull": []interface{}{"$posData.total_price_after_sub_discount", float64(0)}}},
		}},
		{"$sort": bson.M{"_id": 1}},
	}
	cursor, err := ordersColl.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Lỗi aggregate by status: %v", err)
	} else {
		statusLabels := map[string]string{
			"0": "Mới", "17": "Chờ xác nhận", "11": "Chờ hàng", "12": "Chờ in",
			"13": "Đã in", "20": "Đã đặt hàng", "1": "Đã xác nhận", "8": "Đang đóng hàng",
			"9": "Chờ lấy hàng", "2": "Đã giao hàng", "3": "Đã nhận hàng", "16": "Đã thu tiền",
			"4": "Đang trả hàng", "15": "Trả hàng một phần", "5": "Đã trả hàng",
			"6": "Đã hủy", "7": "Đã xóa gần đây", "-1": "Không xác định",
		}
		for cursor.Next(ctx) {
			var doc struct {
				ID     interface{} `bson:"_id"`
				Count  int64       `bson:"count"`
				Amount float64     `bson:"amount"`
			}
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			idStr := fmt.Sprintf("%v", doc.ID)
			label := statusLabels[idStr]
			if label == "" {
				label = idStr
			}
			excluded := ""
			if idStr == "6" || idStr == "7" {
				excluded = " [LOẠI TRỪ]"
			}
			fmt.Printf("   status %s (%s): %d đơn, %.0f VND%s\n", idStr, label, doc.Count, doc.Amount, excluded)
		}
		cursor.Close(ctx)
	}

	// 4. Chạy aggregation GIỐNG report engine (filter từ report_definitions)
	var engCount int64
	var engAmount float64
	reportDefColl := db.Collection("report_definitions")
	var fullDef struct {
		Key           string                 `bson:"key"`
		Metadata      map[string]interface{} `bson:"metadata"`
		TimeField     string                 `bson:"timeField"`
		TimeFieldUnit string                 `bson:"timeFieldUnit"`
	}
	if reportDefColl.FindOne(ctx, bson.M{"key": "order_daily"}).Decode(&fullDef) == nil {
		statusPath := ""
		if sd, ok := fullDef.Metadata["statusDimension"].(map[string]interface{}); ok {
			statusPath, _ = sd["fieldPath"].(string)
		}
		exclude := []interface{}{}
		rawEx := fullDef.Metadata["excludeStatuses"]
		if rawEx != nil {
			switch v := rawEx.(type) {
			case []interface{}:
				exclude = v
			case []int32:
				for _, x := range v {
					exclude = append(exclude, int(x))
				}
			case []int64:
				for _, x := range v {
					exclude = append(exclude, int(x))
				}
			case []int:
				for _, x := range v {
					exclude = append(exclude, x)
				}
			default:
				// BSON có thể decode thành primitive.A hoặc kiểu khác
				if arr, ok := rawEx.(primitive.A); ok {
					exclude = []interface{}(arr)
				}
			}
		}
		engineFilter := bson.M{
			"ownerOrganizationId": orgID,
			fullDef.TimeField:     bson.M{"$gte": timeFrom, "$lte": timeTo},
		}
		if statusPath != "" && len(exclude) > 0 {
			engineFilter[statusPath] = bson.M{"$nin": exclude}
		}
		engCount, engAmount = aggregateOrder(ctx, ordersColl, engineFilter)
		fmt.Printf("\n4. Aggregation GIỐNG engine (filter từ report_definitions):\n   excludeStatuses: %v, statusDimension.fieldPath: %q\n   Số lượng: %d, Doanh thu: %.0f\n", exclude, statusPath, engCount, engAmount)
	}

	// 5. Snapshot đã lưu trong report_snapshots (giá trị UI đang hiển thị)
	snapColl := db.Collection("report_snapshots")
	var snap struct {
		ReportKey  string                 `bson:"reportKey"`
		PeriodKey  string                 `bson:"periodKey"`
		Metrics    map[string]interface{} `bson:"metrics"`
		ComputedAt int64                  `bson:"computedAt"`
	}
	if snapColl.FindOne(ctx, bson.M{
		"reportKey":           "order_daily",
		"periodKey":            periodKey,
		"ownerOrganizationId": orgID,
	}, options.FindOne().SetProjection(bson.M{"reportKey": 1, "periodKey": 1, "metrics": 1, "computedAt": 1})).Decode(&snap) == nil {
		fmt.Printf("\n5. report_snapshots (giá trị UI đang hiển thị):\n")
		if snap.Metrics != nil {
			if total, ok := snap.Metrics["total"].(map[string]interface{}); ok {
				fmt.Printf("   metrics.total.orderCount: %v\n", total["orderCount"])
				fmt.Printf("   metrics.total.totalAmount: %v\n", total["totalAmount"])
			} else {
				fmt.Printf("   metrics: %v\n", snap.Metrics)
			}
		}
		fmt.Printf("   computedAt: %d\n", snap.ComputedAt)
	} else {
		fmt.Printf("\n5. report_snapshots: Không có snapshot order_daily %s cho org này\n", periodKey)
	}

	// Gợi ý: nếu snapshot khác với aggregation đúng → chạy recompute
	if snap.Metrics != nil && engCount > 0 {
		if total, ok := snap.Metrics["total"].(map[string]interface{}); ok {
			var snapCount int64
			switch v := total["orderCount"].(type) {
			case int64:
				snapCount = v
			case int32:
				snapCount = int64(v)
			case float64:
				snapCount = int64(v)
			}
			if engCount != snapCount {
				fmt.Printf("\n⚠ Snapshot (%d) khác với aggregation đúng (%d). Chạy recompute: POST /api/v1/reports/recompute body {\"reportKey\":\"order_daily\",\"from\":\"19-03-2026\",\"to\":\"19-03-2026\"}\n", snapCount, engCount)
			}
		}
	}

	fmt.Println("\n✓ Hoàn thành")
}

func aggregateOrder(ctx context.Context, coll *mongo.Collection, filter bson.M) (int64, float64) {
	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id":          nil,
			"orderCount":   bson.M{"$sum": 1},
			"totalAmount":  bson.M{"$sum": bson.M{"$ifNull": []interface{}{"$posData.total_price_after_sub_discount", float64(0)}}},
		}},
	}
	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0
	}
	defer cursor.Close(ctx)
	if cursor.Next(ctx) {
		var doc struct {
			OrderCount  int64   `bson:"orderCount"`
			TotalAmount float64 `bson:"totalAmount"`
		}
		if cursor.Decode(&doc) == nil {
			return doc.OrderCount, doc.TotalAmount
		}
	}
	return 0, 0
}
