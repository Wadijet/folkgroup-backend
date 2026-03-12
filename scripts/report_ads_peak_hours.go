// Script tạo báo cáo chi tiết khung giờ cao điểm cho điều chỉnh ngân sách Ads.
// Dữ liệu: thời gian gốc (panCakeData/posData), timezone Asia/Ho_Chi_Minh.
//
// Chạy: cd api && go run ../scripts/report_ads_peak_hours.go
// Output: scripts/reports/BAO_CAO_KHUNG_GIO_CAO_DIEM_YYYYMMDD.md
package main

import (
	"context"
	"fmt"
	"log"
	"meta_commerce/config"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	colFbConversations = "fb_conversations"
	colFbMessageItems  = "fb_message_items"
	colPcPosOrders     = "pc_pos_orders"
	tzVietnam          = "Asia/Ho_Chi_Minh"
)

var dayNames = map[int]string{1: "Chủ nhật", 2: "Thứ 2", 3: "Thứ 3", 4: "Thứ 4", 5: "Thứ 5", 6: "Thứ 6", 7: "Thứ 7"}

func main() {
	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(cfg.MongoDB_DBName_Auth)

	var sb strings.Builder
	now := time.Now().Format("2006-01-02 15:04")
	reportDate := time.Now().Format("20060102")

	// Header
	sb.WriteString("# BÁO CÁO KHUNG GIỜ CAO ĐIỂM — ĐIỀU CHỈNH NGÂN SÁCH ADS\n\n")
	sb.WriteString(fmt.Sprintf("**Ngày tạo:** %s  \n**Database:** %s  \n**Timezone:** Asia/Ho_Chi_Minh (UTC+7)\n\n", now, cfg.MongoDB_DBName_Auth))
	sb.WriteString("---\n\n")

	// Phương pháp
	sb.WriteString("## 1. Phương pháp phân tích\n\n")
	sb.WriteString("| Thành phần | Mô tả |\n|------------|-------|\n")
	sb.WriteString("| **Nguồn thời gian** | Dữ liệu gốc từ panCakeData.inserted_at, posData.inserted_at, MessageData.inserted_at (không dùng createdAt/updatedAt sync) |\n")
	sb.WriteString("| **Lưu trữ** | Unix timestamp (UTC) |\n")
	sb.WriteString("| **Hiển thị** | Asia/Ho_Chi_Minh (UTC+7) |\n\n")

	// Conversation
	convTsMs := buildConvTsMs()
	convTotal := getTotal(ctx, db.Collection(colFbConversations), bson.M{"$or": []bson.M{
		{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
		{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
		{"panCakeUpdatedAt": bson.M{"$gt": 0}},
	}})
	convByHour := runHourOnlyAggregation(ctx, db.Collection(colFbConversations), convTsMs)
	convByDay := runDayAggregation(ctx, db.Collection(colFbConversations), convTsMs)
	convByMonth := runMonthAggregation(ctx, db.Collection(colFbConversations), convTsMs)

	// Message
	msgTsMs := bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$insertedAt", 1e12}}, "$insertedAt", bson.M{"$multiply": bson.A{"$insertedAt", 1000}}}}
	msgTotal := getTotal(ctx, db.Collection(colFbMessageItems), bson.M{"insertedAt": bson.M{"$gt": 0}})
	msgByHour := runHourOnlyAggregationSimple(ctx, db.Collection(colFbMessageItems), msgTsMs, "insertedAt")
	msgByDay := runDayAggregationSimple(ctx, db.Collection(colFbMessageItems), msgTsMs, "insertedAt")
	msgByMonth := runMonthAggregationSimple(ctx, db.Collection(colFbMessageItems), msgTsMs, "insertedAt")

	// Orders
	orderTsMs := bson.M{
		"$cond": bson.A{
			bson.M{"$gt": bson.A{bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}}, 1e12}},
			bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}},
			bson.M{"$multiply": bson.A{bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}}, 1000}},
		},
	}
	orderTotal := getTotal(ctx, db.Collection(colPcPosOrders), bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}})
	orderByHour := runHourOnlyAggregationOrder(ctx, db.Collection(colPcPosOrders), orderTsMs)
	orderByDay := runDayAggregationOrder(ctx, db.Collection(colPcPosOrders), orderTsMs)
	orderByMonth := runMonthAggregationOrder(ctx, db.Collection(colPcPosOrders), orderTsMs)
	orderByHourNum := runHourOnlyAggregationOrderByNum(ctx, db.Collection(colPcPosOrders), orderTsMs)
	orderByDayNum := runDayAggregationOrderByNum(ctx, db.Collection(colPcPosOrders), orderTsMs)
	orderByMonthNum := runMonthAggregationOrderByNum(ctx, db.Collection(colPcPosOrders), orderTsMs)

	// Bảng gộp: Phân bố theo giờ (Conversation + Message + Đơn hàng, tách Đơn 1-9)
	sb.WriteString("## 2. Phân bố theo giờ trong ngày (gộp)\n\n")
	sb.WriteString(fmt.Sprintf("**Tổng:** %d hội thoại | %d tin nhắn | %d đơn hàng\n\n", convTotal, msgTotal, orderTotal))
	writeMergedHourSection(&sb, convByHour, msgByHour, orderByHour, orderByHourNum)

	// Bảng gộp: Phân bố theo ngày trong tuần
	sb.WriteString("## 3. Phân bố theo ngày trong tuần (gộp)\n\n")
	writeMergedDaySection(&sb, convByDay, msgByDay, orderByDay, orderByDayNum)

	// Bảng gộp: Xu hướng theo tháng
	sb.WriteString("## 4. Xu hướng theo tháng (gộp)\n\n")
	writeMergedMonthSection(&sb, convByMonth, msgByMonth, orderByMonth, orderByMonthNum)

	// Phân tích conversation trước chốt đơn
	convPerOrder := runConvPerOrderAnalysis(ctx, db)
	writeConvPerOrderSection(&sb, convPerOrder)

	// Phân tích thời gian quay lại mua đơn 2, 3...
	repeatPurchase := runRepeatPurchaseAnalysis(ctx, db)
	writeRepeatPurchaseSection(&sb, repeatPurchase)

	// Khuyến nghị
	sb.WriteString("## 7. KHUYẾN NGHỊ ĐIỀU CHỈNH NGÂN SÁCH ADS\n\n")
	writeRecommendations(&sb, convByHour, msgByHour, orderByHour, convByDay, orderByDay)

	// Ghi file: scripts/reports/ (từ thư mục gốc project)
	reportDir := filepath.Join("..", "scripts", "reports")
	if wd, _ := os.Getwd(); !strings.Contains(wd, "api") {
		reportDir = filepath.Join("scripts", "reports")
	}
	_ = os.MkdirAll(reportDir, 0755)
	outPath := filepath.Join(reportDir, fmt.Sprintf("BAO_CAO_KHUNG_GIO_CAO_DIEM_%s.md", reportDate))
	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		log.Fatalf("Ghi file: %v", err)
	}
	fmt.Printf("✅ Đã tạo báo cáo: %s\n", outPath)
}

func buildConvTsMs() bson.M {
	tsFromInsertedAt := bson.M{
		"$switch": bson.M{
			"branches": []bson.M{
				{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "string"}},
					"then": bson.M{"$toLong": bson.M{"$dateFromString": bson.M{
						"dateString": bson.M{"$substr": bson.A{"$panCakeData.inserted_at", 0, 19}},
						"format": "%Y-%m-%dT%H:%M:%S", "onError": nil, "onNull": nil,
					}}}},
				{"case": bson.M{"$and": bson.A{bson.M{"$ne": bson.A{"$panCakeData.inserted_at", nil}}, bson.M{"$ne": bson.A{"$panCakeData.inserted_at", ""}}}},
					"then": bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1e12}}, bson.M{"$toLong": "$panCakeData.inserted_at"}, bson.M{"$multiply": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1000}}}}},
			},
			"default": nil,
		},
	}
	tsFromUpdatedAt := bson.M{
		"$cond": bson.A{
			bson.M{"$and": bson.A{bson.M{"$ne": bson.A{"$panCakeData.updated_at", nil}}, bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 0}}}},
			bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 1e12}}, bson.M{"$toLong": "$panCakeData.updated_at"}, bson.M{"$multiply": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 1000}}}},
			nil,
		},
	}
	panCakeUpdatedAtToMs := bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 1e12}}, "$panCakeUpdatedAt", bson.M{"$multiply": bson.A{"$panCakeUpdatedAt", 1000}}}}
	tsMs := bson.M{
		"$ifNull": bson.A{
			tsFromInsertedAt,
			bson.M{"$ifNull": bson.A{
				tsFromUpdatedAt,
				bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 0}}, panCakeUpdatedAtToMs, nil}},
			}},
		},
	}
	return tsMs
}

func runHourOnlyAggregation(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []struct{ Hour int; Count int64 } {
	match := bson.M{"$or": []bson.M{
		{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
		{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
		{"panCakeUpdatedAt": bson.M{"$gt": 0}},
	}}
	pipe := []bson.M{
		{"$match": match},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{"_id": bson.M{"$hour": bson.M{"date": "$dt", "timezone": tzVietnam}}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id": 1}},
	}
	return runHourOnlyPipe(ctx, coll, pipe)
}

func runHourOnlyAggregationSimple(ctx context.Context, coll *mongo.Collection, tsMs bson.M, matchField string) []struct{ Hour int; Count int64 } {
	pipe := []bson.M{
		{"$match": bson.M{matchField: bson.M{"$gt": 0, "$exists": true}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{"_id": bson.M{"$hour": bson.M{"date": "$dt", "timezone": tzVietnam}}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id": 1}},
	}
	return runHourOnlyPipe(ctx, coll, pipe)
}

func runHourOnlyAggregationOrder(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []struct{ Hour int; Count int64 } {
	pipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{"_id": bson.M{"$hour": bson.M{"date": "$dt", "timezone": tzVietnam}}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id": 1}},
	}
	return runHourOnlyPipe(ctx, coll, pipe)
}

// orderByNumRow chứa số đơn theo thứ tự (Đơn 1, 2, 3... 9) cho mỗi khung thời gian
type orderByNumRow struct {
	Key     interface{} // Hour (int), Day (int), hoặc "YYYY-MM" (string)
	Order1  int64
	Order2  int64
	Order3  int64
	Order4  int64
	Order5  int64
	Order6  int64
	Order7  int64
	Order8  int64
	Order9  int64
	Order9P int64 // Đơn 10+
}

func runHourOnlyAggregationOrderByNum(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []orderByNumRow {
	custIdExpr := bson.M{"$toString": bson.M{"$ifNull": bson.A{"$customerId", bson.M{"$ifNull": bson.A{"$posData.customer.id", "$posData.customer_id"}}}}}
	pipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{"tsMs": tsMs, "custId": custIdExpr}},
		{"$match": bson.M{"$and": []bson.M{
			{"tsMs": bson.M{"$gt": 0}},
			{"custId": bson.M{"$nin": bson.A{"", "null"}}},
		}}},
		{"$sort": bson.M{"custId": 1, "tsMs": 1}},
		{"$group": bson.M{"_id": "$custId", "times": bson.M{"$push": "$tsMs"}}},
		{"$project": bson.M{
			"orders": bson.M{
				"$map": bson.M{
					"input": bson.M{"$range": bson.A{0, bson.M{"$size": "$times"}}},
					"as":    "i",
					"in": bson.M{
						"ts": bson.M{"$arrayElemAt": bson.A{"$times", "$$i"}},
						"orderNum": bson.M{
							"$cond": bson.M{
								"if":   bson.M{"$gte": bson.A{bson.M{"$add": bson.A{"$$i", 1}}, 10}},
								"then": 10,
								"else": bson.M{"$add": bson.A{"$$i", 1}},
							},
						},
					},
				},
			},
		}},
		{"$unwind": "$orders"},
		{"$addFields": bson.M{
			"dt":       bson.M{"$toDate": "$orders.ts"},
			"orderNum": "$orders.orderNum",
		}},
		{"$addFields": bson.M{"hour": bson.M{"$hour": bson.M{"date": "$dt", "timezone": tzVietnam}}}},
		{"$group": bson.M{
			"_id":      bson.M{"hour": "$hour", "orderNum": "$orderNum"},
			"count":    bson.M{"$sum": 1},
		}},
	}
	opts := options.Aggregate().SetAllowDiskUse(true)
	cursor, err := coll.Aggregate(ctx, pipe, opts)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	type aggDoc struct {
		ID     struct { Hour int `bson:"hour"`; OrderNum int `bson:"orderNum"` } `bson:"_id"`
		Count  int64 `bson:"count"`
	}
	hourOrderCount := make(map[int]*orderByNumRow)
	for cursor.Next(ctx) {
		var d aggDoc
		if cursor.Decode(&d) != nil {
			continue
		}
		h := d.ID.Hour
		if hourOrderCount[h] == nil {
			hourOrderCount[h] = &orderByNumRow{Key: h}
		}
		switch d.ID.OrderNum {
		case 1: hourOrderCount[h].Order1 += d.Count
		case 2: hourOrderCount[h].Order2 += d.Count
		case 3: hourOrderCount[h].Order3 += d.Count
		case 4: hourOrderCount[h].Order4 += d.Count
		case 5: hourOrderCount[h].Order5 += d.Count
		case 6: hourOrderCount[h].Order6 += d.Count
		case 7: hourOrderCount[h].Order7 += d.Count
		case 8: hourOrderCount[h].Order8 += d.Count
		case 9: hourOrderCount[h].Order9 += d.Count
		default: hourOrderCount[h].Order9P += d.Count
		}
	}
	var out []orderByNumRow
	for h := 0; h < 24; h++ {
		if r, ok := hourOrderCount[h]; ok {
			out = append(out, *r)
		} else {
			out = append(out, orderByNumRow{Key: h})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key.(int) < out[j].Key.(int) })
	return out
}

func runHourOnlyPipe(ctx context.Context, coll *mongo.Collection, pipe []bson.M) []struct{ Hour int; Count int64 } {
	var out []struct{ Hour int; Count int64 }
	cursor, err := coll.Aggregate(ctx, pipe)
	if err != nil {
		return out
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var r struct {
			ID    int   `bson:"_id"`
			Count int64 `bson:"count"`
		}
		if cursor.Decode(&r) == nil {
			out = append(out, struct{ Hour int; Count int64 }{r.ID, r.Count})
		}
	}
	return out
}

func runDayAggregation(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []struct{ Day int; Count int64 } {
	match := bson.M{"$or": []bson.M{
		{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
		{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
		{"panCakeUpdatedAt": bson.M{"$gt": 0}},
	}}
	pipe := []bson.M{
		{"$match": match},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{"_id": bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id": 1}},
	}
	return runDayPipe(ctx, coll, pipe)
}

func runDayAggregationSimple(ctx context.Context, coll *mongo.Collection, tsMs bson.M, matchField string) []struct{ Day int; Count int64 } {
	pipe := []bson.M{
		{"$match": bson.M{matchField: bson.M{"$gt": 0, "$exists": true}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{"_id": bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id": 1}},
	}
	return runDayPipe(ctx, coll, pipe)
}

func runDayAggregationOrder(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []struct{ Day int; Count int64 } {
	pipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{"_id": bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id": 1}},
	}
	return runDayPipe(ctx, coll, pipe)
}

func runDayAggregationOrderByNum(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []orderByNumRow {
	custIdExpr := bson.M{"$toString": bson.M{"$ifNull": bson.A{"$customerId", bson.M{"$ifNull": bson.A{"$posData.customer.id", "$posData.customer_id"}}}}}
	pipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{"tsMs": tsMs, "custId": custIdExpr}},
		{"$match": bson.M{"$and": []bson.M{{"tsMs": bson.M{"$gt": 0}}, {"custId": bson.M{"$nin": bson.A{"", "null"}}}}}},
		{"$sort": bson.M{"custId": 1, "tsMs": 1}},
		{"$group": bson.M{"_id": "$custId", "times": bson.M{"$push": "$tsMs"}}},
		{"$project": bson.M{
			"orders": bson.M{
				"$map": bson.M{
					"input": bson.M{"$range": bson.A{0, bson.M{"$size": "$times"}}},
					"as":    "i",
					"in": bson.M{
						"ts": bson.M{"$arrayElemAt": bson.A{"$times", "$$i"}},
						"orderNum": bson.M{
							"$cond": bson.M{
								"if":   bson.M{"$gte": bson.A{bson.M{"$add": bson.A{"$$i", 1}}, 10}},
								"then": 10,
								"else": bson.M{"$add": bson.A{"$$i", 1}},
							},
						},
					},
				},
			},
		}},
		{"$unwind": "$orders"},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$orders.ts"}, "orderNum": "$orders.orderNum"}},
		{"$addFields": bson.M{"day": bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}}}},
		{"$group": bson.M{"_id": bson.M{"day": "$day", "orderNum": "$orderNum"}, "count": bson.M{"$sum": 1}}},
	}
	opts := options.Aggregate().SetAllowDiskUse(true)
	cursor, err := coll.Aggregate(ctx, pipe, opts)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	type aggDoc struct {
		ID    struct { Day int `bson:"day"`; OrderNum int `bson:"orderNum"` } `bson:"_id"`
		Count int64 `bson:"count"`
	}
	dayOrderCount := make(map[int]*orderByNumRow)
	for cursor.Next(ctx) {
		var d aggDoc
		if cursor.Decode(&d) != nil {
			continue
		}
		day := d.ID.Day
		if dayOrderCount[day] == nil {
			dayOrderCount[day] = &orderByNumRow{Key: day}
		}
		switch d.ID.OrderNum {
		case 1: dayOrderCount[day].Order1 += d.Count
		case 2: dayOrderCount[day].Order2 += d.Count
		case 3: dayOrderCount[day].Order3 += d.Count
		case 4: dayOrderCount[day].Order4 += d.Count
		case 5: dayOrderCount[day].Order5 += d.Count
		case 6: dayOrderCount[day].Order6 += d.Count
		case 7: dayOrderCount[day].Order7 += d.Count
		case 8: dayOrderCount[day].Order8 += d.Count
		case 9: dayOrderCount[day].Order9 += d.Count
		default: dayOrderCount[day].Order9P += d.Count
		}
	}
	var out []orderByNumRow
	for d := 1; d <= 7; d++ {
		if r, ok := dayOrderCount[d]; ok {
			out = append(out, *r)
		} else {
			out = append(out, orderByNumRow{Key: d})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key.(int) < out[j].Key.(int) })
	return out
}

func runMonthAggregationOrderByNum(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []orderByNumRow {
	custIdExpr := bson.M{"$toString": bson.M{"$ifNull": bson.A{"$customerId", bson.M{"$ifNull": bson.A{"$posData.customer.id", "$posData.customer_id"}}}}}
	pipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{"tsMs": tsMs, "custId": custIdExpr}},
		{"$match": bson.M{"$and": []bson.M{{"tsMs": bson.M{"$gt": 0}}, {"custId": bson.M{"$nin": bson.A{"", "null"}}}}}},
		{"$sort": bson.M{"custId": 1, "tsMs": 1}},
		{"$group": bson.M{"_id": "$custId", "times": bson.M{"$push": "$tsMs"}}},
		{"$project": bson.M{
			"orders": bson.M{
				"$map": bson.M{
					"input": bson.M{"$range": bson.A{0, bson.M{"$size": "$times"}}},
					"as":    "i",
					"in": bson.M{
						"ts": bson.M{"$arrayElemAt": bson.A{"$times", "$$i"}},
						"orderNum": bson.M{
							"$cond": bson.M{
								"if":   bson.M{"$gte": bson.A{bson.M{"$add": bson.A{"$$i", 1}}, 10}},
								"then": 10,
								"else": bson.M{"$add": bson.A{"$$i", 1}},
							},
						},
					},
				},
			},
		}},
		{"$unwind": "$orders"},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$orders.ts"}, "orderNum": "$orders.orderNum"}},
		{"$addFields": bson.M{
			"year":  bson.M{"$year": bson.M{"date": "$dt", "timezone": tzVietnam}},
			"month": bson.M{"$month": bson.M{"date": "$dt", "timezone": tzVietnam}},
		}},
		{"$group": bson.M{"_id": bson.M{"year": "$year", "month": "$month", "orderNum": "$orderNum"}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id.year": 1, "_id.month": 1}},
		{"$limit": 300},
	}
	opts := options.Aggregate().SetAllowDiskUse(true)
	cursor, err := coll.Aggregate(ctx, pipe, opts)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	type aggDoc struct {
		ID    struct { Year int `bson:"year"`; Month int `bson:"month"`; OrderNum int `bson:"orderNum"` } `bson:"_id"`
		Count int64 `bson:"count"`
	}
	monthOrderCount := make(map[string]*orderByNumRow)
	for cursor.Next(ctx) {
		var d aggDoc
		if cursor.Decode(&d) != nil {
			continue
		}
		key := fmt.Sprintf("%d-%02d", d.ID.Year, d.ID.Month)
		if monthOrderCount[key] == nil {
			monthOrderCount[key] = &orderByNumRow{Key: key}
		}
		switch d.ID.OrderNum {
		case 1: monthOrderCount[key].Order1 += d.Count
		case 2: monthOrderCount[key].Order2 += d.Count
		case 3: monthOrderCount[key].Order3 += d.Count
		case 4: monthOrderCount[key].Order4 += d.Count
		case 5: monthOrderCount[key].Order5 += d.Count
		case 6: monthOrderCount[key].Order6 += d.Count
		case 7: monthOrderCount[key].Order7 += d.Count
		case 8: monthOrderCount[key].Order8 += d.Count
		case 9: monthOrderCount[key].Order9 += d.Count
		default: monthOrderCount[key].Order9P += d.Count
		}
	}
	var months []string
	for k := range monthOrderCount {
		months = append(months, k)
	}
	sort.Strings(months)
	var out []orderByNumRow
	for _, k := range months {
		out = append(out, *monthOrderCount[k])
	}
	return out
}

func runDayPipe(ctx context.Context, coll *mongo.Collection, pipe []bson.M) []struct{ Day int; Count int64 } {
	var out []struct{ Day int; Count int64 }
	cursor, _ := coll.Aggregate(ctx, pipe)
	if cursor == nil {
		return out
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var r struct { ID int `bson:"_id"`; Count int64 `bson:"count"` }
		if cursor.Decode(&r) == nil {
			out = append(out, struct{ Day int; Count int64 }{r.ID, r.Count})
		}
	}
	return out
}

func runMonthAggregation(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []struct{ Year int; Month int; Count int64 } {
	match := bson.M{"$or": []bson.M{
		{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
		{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
		{"panCakeUpdatedAt": bson.M{"$gt": 0}},
	}}
	pipe := []bson.M{
		{"$match": match},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id":   bson.M{"year": bson.M{"$year": bson.M{"date": "$dt", "timezone": tzVietnam}}, "month": bson.M{"$month": bson.M{"date": "$dt", "timezone": tzVietnam}}},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id.year": 1, "_id.month": 1}},
		{"$limit": 24},
	}
	return runMonthPipe(ctx, coll, pipe)
}

func runMonthAggregationSimple(ctx context.Context, coll *mongo.Collection, tsMs bson.M, matchField string) []struct{ Year int; Month int; Count int64 } {
	pipe := []bson.M{
		{"$match": bson.M{matchField: bson.M{"$gt": 0, "$exists": true}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id":   bson.M{"year": bson.M{"$year": bson.M{"date": "$dt", "timezone": tzVietnam}}, "month": bson.M{"$month": bson.M{"date": "$dt", "timezone": tzVietnam}}},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id.year": 1, "_id.month": 1}},
		{"$limit": 24},
	}
	return runMonthPipe(ctx, coll, pipe)
}

func runMonthAggregationOrder(ctx context.Context, coll *mongo.Collection, tsMs bson.M) []struct{ Year int; Month int; Count int64 } {
	pipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id":   bson.M{"year": bson.M{"$year": bson.M{"date": "$dt", "timezone": tzVietnam}}, "month": bson.M{"$month": bson.M{"date": "$dt", "timezone": tzVietnam}}},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id.year": 1, "_id.month": 1}},
		{"$limit": 24},
	}
	return runMonthPipe(ctx, coll, pipe)
}

func runMonthPipe(ctx context.Context, coll *mongo.Collection, pipe []bson.M) []struct{ Year int; Month int; Count int64 } {
	var out []struct{ Year int; Month int; Count int64 }
	cursor, _ := coll.Aggregate(ctx, pipe)
	if cursor == nil {
		return out
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var r struct {
			ID    struct { Year int `bson:"year"`; Month int `bson:"month"` } `bson:"_id"`
			Count int64 `bson:"count"`
		}
		if cursor.Decode(&r) == nil {
			out = append(out, struct{ Year int; Month int; Count int64 }{r.ID.Year, r.ID.Month, r.Count})
		}
	}
	return out
}

func getTotal(ctx context.Context, coll *mongo.Collection, filter bson.M) int64 {
	n, _ := coll.CountDocuments(ctx, filter)
	return n
}

// convPerOrderResult kết quả phân tích conversation trước chốt đơn
type convPerOrderResult struct {
	OrdersWithCustomer   int64
	OrdersWithConvBefore int64
	AvgConvPerOrder      float64
	MedianConvPerOrder   int
	OrdersWithConvLink   int64   // Đơn có posData.conversation_id link được
	AvgHoursToOrder      float64 // TB giờ từ conv đầu → chốt đơn (chăm khách bao lâu)
	MedianHoursToOrder   float64
	HoursDistribution   []struct{ Bucket string; Count int }
	Distribution        []struct{ Bucket string; Count int }
}

// runConvPerOrderAnalysis phân tích: với mỗi đơn, có bao nhiêu conversation trước khi chốt, thời gian từ conv đầu đến đơn.
func runConvPerOrderAnalysis(ctx context.Context, db *mongo.Database) convPerOrderResult {
	var out convPerOrderResult
	ordersColl := db.Collection(colPcPosOrders)

	// Lấy orderTs (ms) và customerId từ orders
	orderTsMs := bson.M{
		"$cond": bson.A{
			bson.M{"$gt": bson.A{bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}}, 1e12}},
			bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}},
			bson.M{"$multiply": bson.A{bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}}, 1000}},
		},
	}
	// convTsMs: Ưu tiên inserted_at (bắt đầu conv) — KHÔNG dùng panCakeUpdatedAt vì đó là lần cập nhật cuối, có thể SAU khi đặt hàng → ra số âm
	// Cả order và conv đều Unix timestamp (s hoặc ms) — không parse ISO để tránh lệch múi giờ
	convTsMs := bson.M{
		"$cond": bson.A{
			bson.M{"$and": bson.A{bson.M{"$ne": bson.A{"$panCakeData.inserted_at", nil}}, bson.M{"$ne": bson.A{"$panCakeData.inserted_at", ""}}}},
			bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "string"}},
				bson.M{"$toLong": bson.M{"$dateFromString": bson.M{"dateString": bson.M{"$substr": bson.A{"$panCakeData.inserted_at", 0, 19}}, "format": "%Y-%m-%dT%H:%M:%S", "onError": nil, "onNull": nil}}},
				bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1e12}}, bson.M{"$toLong": "$panCakeData.inserted_at"}, bson.M{"$multiply": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1000}}}},
			}},
			bson.M{"$cond": bson.A{
				bson.M{"$and": bson.A{bson.M{"$ne": bson.A{"$panCakeData.updated_at", nil}}, bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 0}}}},
				bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 1e12}}, bson.M{"$toLong": "$panCakeData.updated_at"}, bson.M{"$multiply": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 1000}}}},
				bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 0}}, bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 1e12}}, "$panCakeUpdatedAt", bson.M{"$multiply": bson.A{"$panCakeUpdatedAt", 1000}}}}, nil}},
			}},
		},
	}

	// Phân tích theo posData.conversation_id: CHỈ đơn đầu tiên của mỗi khách — thời gian từ conv đầu đến đơn đầu tiên
	// Loại đơn 2, 3... để tránh số giờ bị kéo dài (conv từ tháng trước vẫn còn link)
	custIdExpr := bson.M{"$toString": bson.M{"$ifNull": bson.A{"$customerId", bson.M{"$ifNull": bson.A{"$posData.customer.id", "$posData.customer_id"}}}}}
	pipeConvId := []bson.M{
		{"$match": bson.M{"$and": []bson.M{
			{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}},
			{"$or": []bson.M{
				{"posData.conversation_id": bson.M{"$exists": true, "$ne": ""}},
				{"posData.conversationId": bson.M{"$exists": true, "$ne": ""}},
				{"posData.conversation_link": bson.M{"$exists": true, "$ne": ""}},
			}},
		}}},
		{"$addFields": bson.M{
			"orderTsMs": orderTsMs,
			"convId":    bson.M{"$ifNull": bson.A{"$posData.conversation_id", bson.M{"$ifNull": bson.A{"$posData.conversationId", "$posData.conversation_link"}}}},
			"custId":    custIdExpr,
		}},
		{"$match": bson.M{"$expr": bson.M{"$and": []bson.M{
			{"$ne": bson.A{"$custId", ""}},
			{"$ne": bson.A{"$custId", "null"}},
		}}}},
		// Chỉ giữ đơn đầu tiên: không có đơn nào trước đó của cùng khách
		{"$lookup": bson.M{
			"from": colPcPosOrders,
			"let":  bson.M{"custId": "$custId", "orderTs": "$orderTsMs", "oid": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{"$ne": bson.A{"$_id", "$$oid"}}}},
				{"$addFields": bson.M{
					"oTs": orderTsMs,
					"cId": custIdExpr,
				}},
				{"$match": bson.M{"$expr": bson.M{"$and": []bson.M{
					{"$eq": bson.A{"$cId", "$$custId"}},
					{"$lt": bson.A{"$oTs", "$$orderTs"}},
				}}}},
				{"$limit": 1},
			},
			"as": "prevOrders",
		}},
		{"$match": bson.M{"$expr": bson.M{"$eq": bson.A{bson.M{"$size": "$prevOrders"}, 0}}}},
		{"$limit": 5000},
		{"$lookup": bson.M{
			"from": "fb_conversations",
			"let":  bson.M{"cid": "$convId"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{"$or": []bson.M{
					{"$eq": bson.A{"$conversationId", "$$cid"}},
					{"$eq": bson.A{bson.M{"$ifNull": bson.A{"$panCakeData.id", ""}}, "$$cid"}},
				}}}},
				{"$limit": 1},
				{"$addFields": bson.M{"tsMs": convTsMs}},
				{"$match": bson.M{"$expr": bson.M{"$gt": bson.A{"$tsMs", 0}}}},
				{"$project": bson.M{"tsMs": 1}},
			},
			"as": "conv",
		}},
		{"$addFields": bson.M{
			"convTs": bson.M{"$arrayElemAt": bson.A{bson.M{"$map": bson.M{"input": "$conv", "as": "c", "in": "$$c.tsMs"}}, 0}},
		}},
		{"$match": bson.M{"convTs": bson.M{"$gt": 0}}},
		{"$addFields": bson.M{
			"hoursToOrder": bson.M{"$divide": bson.A{bson.M{"$subtract": bson.A{"$orderTsMs", "$convTs"}}, 3600000}},
		}},
		{"$match": bson.M{"hoursToOrder": bson.M{"$gt": 0}}},
		{"$group": bson.M{
			"_id":   nil,
			"count": bson.M{"$sum": 1},
			"hours": bson.M{"$push": "$hoursToOrder"},
		}},
	}
	if cur, err := ordersColl.Aggregate(ctx, pipeConvId); err == nil {
		var agg struct {
			Count int64         `bson:"count"`
			Hours []interface{} `bson:"hours"`
		}
		if cur.Next(ctx) {
			_ = cur.Decode(&agg)
		}
		cur.Close(ctx)
		out.OrdersWithConvLink = agg.Count
		var hoursList []float64
		for _, h := range agg.Hours {
			if h == nil {
				continue
			}
			switch v := h.(type) {
			case float64:
				hoursList = append(hoursList, v)
			case int:
				hoursList = append(hoursList, float64(v))
			case int32:
				hoursList = append(hoursList, float64(v))
			case int64:
				hoursList = append(hoursList, float64(v))
			}
		}
		if len(hoursList) > 0 {
			sort.Float64s(hoursList)
			var sum float64
			for _, v := range hoursList {
				sum += v
			}
			out.AvgHoursToOrder = sum / float64(len(hoursList))
			out.MedianHoursToOrder = hoursList[len(hoursList)/2]
			buckets := map[string]int{"<1h": 0, "1-4h": 0, "4-24h": 0, "1-3 ngày": 0, "3-7 ngày": 0, ">7 ngày": 0}
			for _, h := range hoursList {
				if h < 1 {
					buckets["<1h"]++
				} else if h < 4 {
					buckets["1-4h"]++
				} else if h < 24 {
					buckets["4-24h"]++
				} else if h < 72 {
					buckets["1-3 ngày"]++
				} else if h < 168 {
					buckets["3-7 ngày"]++
				} else {
					buckets[">7 ngày"]++
				}
			}
			out.HoursDistribution = []struct{ Bucket string; Count int }{
				{"<1h", buckets["<1h"]},
				{"1-4h", buckets["1-4h"]},
				{"4-24h", buckets["4-24h"]},
				{"1-3 ngày", buckets["1-3 ngày"]},
				{"3-7 ngày", buckets["3-7 ngày"]},
				{">7 ngày", buckets[">7 ngày"]},
			}
		}
	}

	// Pipeline Part 5 — order.customerId, posData.customer.id, fallback posData.customer_id; fb_conversations.customerId (Pancake UUID)
	pipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{
			"orderTsMs": orderTsMs,
			"custId":    bson.M{"$toString": bson.M{"$ifNull": bson.A{"$customerId", bson.M{"$ifNull": bson.A{"$posData.customer.id", "$posData.customer_id"}}}}},
		}},
		{"$match": bson.M{"$expr": bson.M{"$and": []bson.M{
			{"$gt": bson.A{"$orderTsMs", 0}},
			{"$ne": bson.A{"$custId", ""}},
			{"$ne": bson.A{"$custId", "null"}},
		}}}},
		{"$sort": bson.M{"orderTsMs": -1}},
		{"$limit": 5000},
		{"$lookup": bson.M{
			"from": "fb_conversations",
			"let":  bson.M{"oid": "$orderTsMs", "cid": "$custId"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{"$and": []bson.M{
					{"$ne": bson.A{"$$cid", ""}},
					{"$eq": bson.A{bson.M{"$toString": bson.M{"$ifNull": bson.A{"$customerId", ""}}}, "$$cid"}},
				}}}},
				{"$addFields": bson.M{"tsMs": convTsMs}},
				{"$match": bson.M{"$expr": bson.M{"$and": []bson.M{
					{"$gt": bson.A{"$tsMs", 0}},
					{"$lte": bson.A{"$tsMs", "$$oid"}},
				}}}},
				{"$group": bson.M{
					"_id":     nil,
					"count":   bson.M{"$sum": 1},
					"firstTs": bson.M{"$min": "$tsMs"},
				}},
			},
			"as": "convStats",
		}},
		{"$addFields": bson.M{
			"firstDoc": bson.M{"$arrayElemAt": bson.A{"$convStats", 0}},
		}},
		{"$addFields": bson.M{
			"convCount":   bson.M{"$ifNull": bson.A{"$firstDoc.count", 0}},
			"firstConvTs": bson.M{"$ifNull": bson.A{"$firstDoc.firstTs", 0}},
		}},
		{"$addFields": bson.M{
			"hoursToOrder": bson.M{"$cond": bson.A{
				bson.M{"$gt": bson.A{"$firstConvTs", 0}},
				bson.M{"$divide": bson.A{bson.M{"$subtract": bson.A{"$orderTsMs", "$firstConvTs"}}, 3600000}},
				nil,
			}},
		}},
		{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": 1},
			"withConv": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$convCount", 0}}, 1, 0}}},
			"sumConv": bson.M{"$sum": "$convCount"},
			"convCounts": bson.M{"$push": "$convCount"},
			"hoursList": bson.M{"$push": bson.M{"$cond": bson.A{bson.M{"$ne": bson.A{"$hoursToOrder", nil}}, "$hoursToOrder", nil}}},
		}},
	}
	cursor, err := ordersColl.Aggregate(ctx, pipe)
	if err != nil {
		return out
	}
	defer cursor.Close(ctx)
	var agg struct {
		Total    int64         `bson:"total"`
		WithConv int64         `bson:"withConv"`
		SumConv  int64         `bson:"sumConv"`
		Counts   []int         `bson:"convCounts"`
		Hours    []interface{} `bson:"hoursList"`
	}
	if cursor.Next(ctx) {
		_ = cursor.Decode(&agg)
	}
	out.OrdersWithCustomer = agg.Total
	out.OrdersWithConvBefore = agg.WithConv
	if agg.Total > 0 {
		out.AvgConvPerOrder = float64(agg.SumConv) / float64(agg.Total)
	}
	if len(agg.Counts) > 0 {
		sort.Ints(agg.Counts)
		out.MedianConvPerOrder = agg.Counts[len(agg.Counts)/2]
	}
	var hoursList []float64
	for _, h := range agg.Hours {
		if h == nil {
			continue
		}
		switch v := h.(type) {
		case float64:
			hoursList = append(hoursList, v)
		case int:
			hoursList = append(hoursList, float64(v))
		case int32:
			hoursList = append(hoursList, float64(v))
		case int64:
			hoursList = append(hoursList, float64(v))
		}
	}
	if len(hoursList) > 0 {
		sort.Float64s(hoursList)
		var sum float64
		for _, v := range hoursList {
			sum += v
		}
		out.AvgHoursToOrder = sum / float64(len(hoursList))
		out.MedianHoursToOrder = hoursList[len(hoursList)/2]
	}
	// Phân bố số conv theo bucket
	bucketCounts := make(map[string]int)
	for _, c := range agg.Counts {
		b := "0"
		if c >= 1 && c <= 2 {
			b = "1-2"
		} else if c >= 3 && c <= 5 {
			b = "3-5"
		} else if c >= 6 && c <= 10 {
			b = "6-10"
		} else if c > 10 {
			b = "11+"
		}
		bucketCounts[b]++
	}
	out.Distribution = []struct{ Bucket string; Count int }{
		{"0", bucketCounts["0"]},
		{"1-2", bucketCounts["1-2"]},
		{"3-5", bucketCounts["3-5"]},
		{"6-10", bucketCounts["6-10"]},
		{"11+", bucketCounts["11+"]},
	}
	return out
}

func writeConvPerOrderSection(sb *strings.Builder, r convPerOrderResult) {
	sb.WriteString("## 5. PHÂN TÍCH CONVERSATION TRƯỚC CHỐT ĐƠN\n\n")
	sb.WriteString("**Chăm khách bao lâu mới ra đơn đầu tiên?** — Số giờ từ lúc bắt đầu có conversation (link qua posData.conversation_id) đến khi khách đặt đơn đầu tiên.\n\n")
	sb.WriteString("*Chỉ phân tích đơn đầu tiên của mỗi khách* (loại đơn 2, 3... để tránh số giờ bị kéo dài do conversation từ tháng trước).\n\n")
	sb.WriteString(fmt.Sprintf("| Chỉ số | Giá trị |\n|--------|--------|\n"))
	sb.WriteString(fmt.Sprintf("| Đơn đầu tiên có conversation_id link được | %d |\n", r.OrdersWithConvLink))
	sb.WriteString(fmt.Sprintf("| **TB giờ chăm khách → chốt đơn** | **%.1f giờ** |\n", r.AvgHoursToOrder))
	sb.WriteString(fmt.Sprintf("| **Trung vị giờ chăm khách → chốt đơn** | **%.1f giờ** |\n", r.MedianHoursToOrder))
	if len(r.HoursDistribution) > 0 {
		sb.WriteString("\n**Phân bố thời gian chăm khách (từ conv đầu → chốt đơn):**\n\n")
		sb.WriteString("| Khoảng thời gian | Số đơn |\n|------------------|--------|\n")
		for _, d := range r.HoursDistribution {
			sb.WriteString(fmt.Sprintf("| %s | %d |\n", d.Bucket, d.Count))
		}
	}
	sb.WriteString(fmt.Sprintf("\n*Phụ: Số conv trước chốt (link customerId, mẫu 2000 đơn):* Đơn có customerId %d | Có ≥1 conv %d | TB conv/đơn %.1f\n\n", r.OrdersWithCustomer, r.OrdersWithConvBefore, r.AvgConvPerOrder))
}

func writeRepeatPurchaseSection(sb *strings.Builder, r repeatPurchaseResult) {
	sb.WriteString("## 6. PHÂN TÍCH THỜI GIAN QUAY LẠI MUA (ĐƠN 2, 3...)\n\n")
	sb.WriteString("Khoảng thời gian (ngày) giữa các đơn hàng liên tiếp của cùng khách.\n\n")
	sb.WriteString(fmt.Sprintf("| Chỉ số | Giá trị |\n|--------|--------|\n"))
	sb.WriteString(fmt.Sprintf("| Khách có ≥2 đơn | %d |\n", r.CustomersWith2Plus))
	if len(r.Intervals) > 0 {
		sb.WriteString("\n| Khoảng | Số lần | TB ngày | Trung vị ngày |\n|--------|--------|---------|---------------|\n")
		for _, i := range r.Intervals {
			sb.WriteString(fmt.Sprintf("| %s | %d | %.1f | %.1f |\n", i.From, i.Count, i.AvgDays, i.MedianDays))
		}
	}
	sb.WriteString("\n")
}

// repeatPurchaseResult kết quả phân tích thời gian quay lại mua đơn 2, 3...
type repeatPurchaseResult struct {
	CustomersWith2Plus  int
	Intervals           []struct{ From, To string; Count int; AvgDays, MedianDays float64 }
}

// runRepeatPurchaseAnalysis phân tích thời gian giữa đơn 1→2, 2→3, 3→4...
func runRepeatPurchaseAnalysis(ctx context.Context, db *mongo.Database) repeatPurchaseResult {
	var out repeatPurchaseResult
	ordersColl := db.Collection(colPcPosOrders)
	orderTsMs := bson.M{
		"$cond": bson.A{
			bson.M{"$gt": bson.A{bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}}, 1e12}},
			bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}},
			bson.M{"$multiply": bson.A{bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}}, 1000}},
		},
	}
	// Pipeline: group by customer — order.customerId, posData.customer.id, fallback posData.customer_id
	custIdExpr := bson.M{"$ifNull": bson.A{"$customerId", bson.M{"$ifNull": bson.A{"$posData.customer.id", "$posData.customer_id"}}}}
	pipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{
			"orderTsMs": orderTsMs,
			"custId":    bson.M{"$toString": custIdExpr},
		}},
		{"$match": bson.M{"$and": []bson.M{
			{"orderTsMs": bson.M{"$gt": 0}},
			{"custId": bson.M{"$nin": bson.A{"", "null"}}},
		}}},
		{"$sort": bson.M{"orderTsMs": 1}},
		{"$group": bson.M{
			"_id":   "$custId",
			"times": bson.M{"$push": "$orderTsMs"},
		}},
		{"$match": bson.M{"$expr": bson.M{"$gte": bson.A{bson.M{"$size": "$times"}, 2}}}},
		{"$project": bson.M{
			"intervals": bson.M{
				"$map": bson.M{
					"input": bson.M{"$range": bson.A{1, bson.M{"$size": "$times"}}},
					"as":    "i",
					"in": bson.M{
						"idx": "$$i",
						"days": bson.M{"$divide": bson.A{
							bson.M{"$subtract": bson.A{
								bson.M{"$arrayElemAt": bson.A{"$times", "$$i"}},
								bson.M{"$arrayElemAt": bson.A{"$times", bson.M{"$subtract": bson.A{"$$i", 1}}}},
							}},
							86400000,
						}},
					},
				},
			},
		}},
		{"$unwind": "$intervals"},
		{"$group": bson.M{
			"_id":   "$intervals.idx",
			"days":   bson.M{"$push": "$intervals.days"},
			"count":  bson.M{"$sum": 1},
		}},
	}
	opts := options.Aggregate().SetAllowDiskUse(true)
	cursor, err := ordersColl.Aggregate(ctx, pipe, opts)
	if err != nil {
		return out
	}
	defer cursor.Close(ctx)
	type intervalDoc struct {
		ID    int       `bson:"_id"`
		Days  []float64 `bson:"days"`
		Count int       `bson:"count"`
	}
	var docs []intervalDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return out
	}
	// Đếm khách có 2+ đơn — order.customerId, posData.customer.id, fallback posData.customer_id
	custIdExpr2 := bson.M{"$ifNull": bson.A{"$customerId", bson.M{"$ifNull": bson.A{"$posData.customer.id", "$posData.customer_id"}}}}
	custPipe := []bson.M{
		{"$match": bson.M{"$or": []bson.M{{"insertedAt": bson.M{"$gt": 0}}, {"posCreatedAt": bson.M{"$gt": 0}}}}},
		{"$addFields": bson.M{"custId": bson.M{"$toString": custIdExpr2}}},
		{"$match": bson.M{"custId": bson.M{"$nin": bson.A{"", "null"}}}},
		{"$group": bson.M{"_id": "$custId", "cnt": bson.M{"$sum": 1}}},
		{"$match": bson.M{"cnt": bson.M{"$gte": 2}}},
		{"$count": "n"},
	}
	if c, _ := ordersColl.Aggregate(ctx, custPipe, options.Aggregate().SetAllowDiskUse(true)); c.Next(ctx) {
		var cDoc struct{ N int `bson:"n"` }
		_ = c.Decode(&cDoc)
		out.CustomersWith2Plus = cDoc.N
		c.Close(ctx)
	}
	// Sắp xếp theo idx (1→2, 2→3, 3→4...)
	sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
	labels := map[int]string{1: "Đơn 1→2", 2: "Đơn 2→3", 3: "Đơn 3→4", 4: "Đơn 4→5", 5: "Đơn 5→6"}
	for _, d := range docs {
		if len(d.Days) == 0 {
			continue
		}
		sort.Float64s(d.Days)
		var sum float64
		for _, v := range d.Days {
			sum += v
		}
		label := labels[d.ID]
		if label == "" {
			label = fmt.Sprintf("Đơn %d→%d", d.ID, d.ID+1)
		}
		out.Intervals = append(out.Intervals, struct{ From, To string; Count int; AvgDays, MedianDays float64 }{
			label, "", d.Count, sum / float64(len(d.Days)), d.Days[len(d.Days)/2],
		})
	}
	return out
}

// formatCR trả về tỷ lệ chuyển đổi (conversation → đơn hàng) dạng "X.XX%", hoặc "—" nếu không có conversation.
func formatCR(conv, order int64) string {
	if conv <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.2f%%", float64(order)/float64(conv)*100)
}

// formatMessPerOrder trả về số message/đơn, hoặc "—" nếu không có đơn.
func formatMessPerOrder(msg, order int64) string {
	if order <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f", float64(msg)/float64(order))
}

func writeMergedDaySection(sb *strings.Builder, conv, msg, order []struct{ Day int; Count int64 }, orderByNum []orderByNumRow) {
	convMap := make(map[int]int64)
	for _, r := range conv {
		convMap[r.Day] = r.Count
	}
	msgMap := make(map[int]int64)
	for _, r := range msg {
		msgMap[r.Day] = r.Count
	}
	orderMap := make(map[int]int64)
	for _, r := range order {
		orderMap[r.Day] = r.Count
	}
	orderNumMap := make(map[int]orderByNumRow)
	for _, r := range orderByNum {
		if k, ok := r.Key.(int); ok {
			orderNumMap[k] = r
		}
	}
	sb.WriteString("| Ngày | Conversation | Message | Đơn 1 | Đơn 2 | Đơn 3 | Đơn 4 | Đơn 5 | Đơn 6 | Đơn 7 | Đơn 8 | Đơn 9 | 9+ | Mess/đơn | CR (%) |\n")
	sb.WriteString("|------|--------------|---------|-------|-------|-------|-------|-------|-------|-------|-------|-------|-----|---------|--------|\n")
	for d := 1; d <= 7; d++ {
		dayName := dayNames[d]
		if dayName == "" {
			dayName = fmt.Sprintf("Thứ %d", d)
		}
		cv := convMap[d]
		mg := msgMap[d]
		od := orderMap[d]
		on := orderNumMap[d]
		mpo := formatMessPerOrder(mg, od)
		cr := formatCR(cv, od)
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %s | %s |\n",
			dayName, cv, mg, on.Order1, on.Order2, on.Order3, on.Order4, on.Order5, on.Order6, on.Order7, on.Order8, on.Order9, on.Order9P, mpo, cr))
	}
	sb.WriteString("\n")
}

func writeMergedMonthSection(sb *strings.Builder, conv, msg, order []struct{ Year int; Month int; Count int64 }, orderByNum []orderByNumRow) {
	convMap := make(map[string]int64)
	for _, r := range conv {
		convMap[fmt.Sprintf("%d-%02d", r.Year, r.Month)] = r.Count
	}
	msgMap := make(map[string]int64)
	for _, r := range msg {
		msgMap[fmt.Sprintf("%d-%02d", r.Year, r.Month)] = r.Count
	}
	orderMap := make(map[string]int64)
	for _, r := range order {
		orderMap[fmt.Sprintf("%d-%02d", r.Year, r.Month)] = r.Count
	}
	orderNumMap := make(map[string]orderByNumRow)
	for _, r := range orderByNum {
		if k, ok := r.Key.(string); ok {
			orderNumMap[k] = r
		}
	}
	// Gộp tất cả tháng, sắp theo thứ tự thời gian
	seen := make(map[string]bool)
	var months []string
	for k := range convMap {
		if !seen[k] {
			seen[k] = true
			months = append(months, k)
		}
	}
	for k := range msgMap {
		if !seen[k] {
			seen[k] = true
			months = append(months, k)
		}
	}
	for k := range orderMap {
		if !seen[k] {
			seen[k] = true
			months = append(months, k)
		}
	}
	// Sắp xếp theo thứ tự thời gian (YYYY-MM)
	sort.Strings(months)
	sb.WriteString("| Tháng | Conversation | Message | Đơn 1 | Đơn 2 | Đơn 3 | Đơn 4 | Đơn 5 | Đơn 6 | Đơn 7 | Đơn 8 | Đơn 9 | 9+ | Mess/đơn | CR (%) |\n")
	sb.WriteString("|-------|--------------|---------|-------|-------|-------|-------|-------|-------|-------|-------|-------|-----|---------|--------|\n")
	for _, m := range months {
		cv := convMap[m]
		mg := msgMap[m]
		od := orderMap[m]
		on := orderNumMap[m]
		mpo := formatMessPerOrder(mg, od)
		cr := formatCR(cv, od)
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %s | %s |\n",
			m, cv, mg, on.Order1, on.Order2, on.Order3, on.Order4, on.Order5, on.Order6, on.Order7, on.Order8, on.Order9, on.Order9P, mpo, cr))
	}
	sb.WriteString("\n")
}

func writeMergedHourSection(sb *strings.Builder, conv, msg, order []struct{ Hour int; Count int64 }, orderByNum []orderByNumRow) {
	convMap := make(map[int]int64)
	for _, r := range conv {
		convMap[r.Hour] = r.Count
	}
	msgMap := make(map[int]int64)
	for _, r := range msg {
		msgMap[r.Hour] = r.Count
	}
	orderMap := make(map[int]int64)
	for _, r := range order {
		orderMap[r.Hour] = r.Count
	}
	orderNumMap := make(map[int]orderByNumRow)
	for _, r := range orderByNum {
		if k, ok := r.Key.(int); ok {
			orderNumMap[k] = r
		}
	}
	sb.WriteString("| # | Khung giờ (VN) | Conversation | Message | Đơn 1 | Đơn 2 | Đơn 3 | Đơn 4 | Đơn 5 | Đơn 6 | Đơn 7 | Đơn 8 | Đơn 9 | 9+ | Mess/đơn | CR (%) |\n")
	sb.WriteString("|---|----------------|--------------|---------|-------|-------|-------|-------|-------|-------|-------|-------|-------|-----|---------|--------|\n")
	for h := 0; h < 24; h++ {
		cv := convMap[h]
		mg := msgMap[h]
		od := orderMap[h]
		on := orderNumMap[h]
		mpo := formatMessPerOrder(mg, od)
		cr := formatCR(cv, od)
		sb.WriteString(fmt.Sprintf("| %d | %02d:00-%02d:59 | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %d | %s | %s |\n",
			h+1, h, h, cv, mg, on.Order1, on.Order2, on.Order3, on.Order4, on.Order5, on.Order6, on.Order7, on.Order8, on.Order9, on.Order9P, mpo, cr))
	}
	sb.WriteString("\n")
}

func writeRecommendations(sb *strings.Builder, conv, msg, order []struct{ Hour int; Count int64 }, convDay, orderDay []struct{ Day int; Count int64 }) {
	sb.WriteString("### 7.1. Đề xuất theo khung giờ (24h)\n\n")
	// Tính điểm cho mỗi giờ: conv + msg/10 + order*100
	hourScores := make(map[int]int64)
	for _, r := range conv {
		hourScores[r.Hour] += r.Count
	}
	for _, r := range msg {
		hourScores[r.Hour] += r.Count / 10
	}
	for _, r := range order {
		hourScores[r.Hour] += r.Count * 100
	}
	// Sắp xếp theo điểm để xác định phân vị
	var byScore []struct{ H int; Score int64 }
	for h := 0; h < 24; h++ {
		byScore = append(byScore, struct{ H int; Score int64 }{h, hourScores[h]})
	}
	for i := 0; i < len(byScore)-1; i++ {
		for j := i + 1; j < len(byScore); j++ {
			if byScore[j].Score > byScore[i].Score {
				byScore[i], byScore[j] = byScore[j], byScore[i]
			}
		}
	}
	// Phân loại: top 1/3 = Tăng, bottom 1/3 = Hạ ads, giữa = Giảm
	actionByHour := make(map[int]string)
	for i, s := range byScore {
		switch {
		case i < 8:
			actionByHour[s.H] = "Tăng"
		case i >= 16:
			actionByHour[s.H] = "Hạ ads"
		default:
			actionByHour[s.H] = "Giảm"
		}
	}
	sb.WriteString("| # | Khung giờ (VN) | Đề xuất |\n|---|----------------|----------|\n")
	for h := 0; h < 24; h++ {
		action := actionByHour[h]
		sb.WriteString(fmt.Sprintf("| %d | %02d:00-%02d:59 | %s |\n", h+1, h, h, action))
	}
	sb.WriteString("\n### 7.2. Ngày trong tuần (tăng budget)\n\n")
	dayPeaks := make(map[int]int64)
	for _, r := range convDay {
		dayPeaks[r.Day] += r.Count
	}
	for _, r := range orderDay {
		dayPeaks[r.Day] += r.Count * 50
	}
	var sortedDay []struct{ D int; Score int64 }
	for d, s := range dayPeaks {
		sortedDay = append(sortedDay, struct{ D int; Score int64 }{d, s})
	}
	for i := 0; i < len(sortedDay)-1; i++ {
		for j := i + 1; j < len(sortedDay); j++ {
			if sortedDay[j].Score > sortedDay[i].Score {
				sortedDay[i], sortedDay[j] = sortedDay[j], sortedDay[i]
			}
		}
	}
	sb.WriteString("| Ưu tiên | Ngày |\n|---------|------|\n")
	for i := 0; i < 7 && i < len(sortedDay); i++ {
		dn := dayNames[sortedDay[i].D]
		if dn == "" {
			dn = fmt.Sprintf("Thứ %d", sortedDay[i].D)
		}
		sb.WriteString(fmt.Sprintf("| %d | %s |\n", i+1, dn))
	}
	sb.WriteString("\n### 7.3. Chú thích đề xuất\n\n")
	sb.WriteString("- **Tăng**: Khung giờ cao điểm — tăng budget ads.\n")
	sb.WriteString("- **Giảm**: Khung giờ trung bình — giảm budget.\n")
	sb.WriteString("- **Hạ ads**: Khung giờ thấp — hạ mức ads hoặc tạm dừng.\n")
	sb.WriteString("- Cấu hình **Event Calendar** trong ads_meta_config theo bảng trên.\n")
}
