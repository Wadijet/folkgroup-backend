// Script phân tích cao điểm conversation và đơn hàng theo thời gian.
// Phục vụ bài toán điều chỉnh ngân sách ads: biết ngày/giờ nào có nhiều conversation và đơn để tăng budget.
//
// Thời gian: Dùng thời gian GỐC từ dữ liệu nguồn (panCakeData.inserted_at, posData.inserted_at, MessageData.inserted_at),
// KHÔNG dùng thời gian đồng bộ (createdAt, updatedAt của document).
//
// Timezone: Asia/Ho_Chi_Minh (UTC+7) — giống report engine và ads scheduler.
//
// Chạy từ thư mục api: go run ../scripts/analyze_ads_peak_hours.go
package main

import (
	"context"
	"fmt"
	"log"
	"meta_commerce/config"
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

	// Timezone cho phân tích — giống report engine (Asia/Ho_Chi_Minh)
	tzVietnam = "Asia/Ho_Chi_Minh"
)

// dayOfWeekNames map 1=Chủ nhật, 2=Thứ 2, ..., 7=Thứ 7
var dayOfWeekNames = map[int]string{
	1: "Chủ nhật",
	2: "Thứ 2",
	3: "Thứ 3",
	4: "Thứ 4",
	5: "Thứ 5",
	6: "Thứ 6",
	7: "Thứ 7",
}

func main() {
	fmt.Println("=== Phân Tích Cao Điểm Conversation & Đơn Hàng (cho Ads Budget) ===\n")

	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình từ file env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Không thể kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Không thể ping MongoDB: %v", err)
	}

	db := client.Database(cfg.MongoDB_DBName_Auth)
	fmt.Printf("✓ Đã kết nối: %s\n\n", cfg.MongoDB_DBName_Auth)

	// 1. Phân tích fb_conversations theo ngày trong tuần và giờ
	analyzeConversationsByDayAndHour(ctx, db)
	// 2. Phân tích fb_message_items (số tin nhắn) theo ngày và giờ
	analyzeMessagesByDayAndHour(ctx, db)
	// 3. Phân tích đơn hàng (pc_pos_orders) theo ngày và giờ
	analyzeOrdersByDayAndHour(ctx, db)
	// 4. Phân tích theo tháng trong năm (xu hướng theo mùa)
	analyzeByMonth(ctx, db)
}

func analyzeConversationsByDayAndHour(ctx context.Context, db *mongo.Database) {
	coll := db.Collection(colFbConversations)
	count, _ := coll.CountDocuments(ctx, bson.M{})
	if count == 0 {
		fmt.Printf("⚠ %s: không có dữ liệu\n\n", colFbConversations)
		return
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("📊 1. CONVERSATION (fb_conversations) - Theo ngày trong tuần & giờ")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Tổng số conversation: %d\n", count)
	fmt.Printf("⏱️  Thời gian gốc: panCakeData.inserted_at → updated_at → panCakeUpdatedAt | Timezone: %s\n\n", tzVietnam)

	// Thời gian GỐC: ưu tiên panCakeData.inserted_at (hội thoại bắt đầu), panCakeData.updated_at, panCakeUpdatedAt. Không dùng createdAt (sync).
	// panCakeData.inserted_at có thể là string ISO hoặc number (Unix sec/ms)
	tsFromInsertedAt := bson.M{
		"$switch": bson.M{
			"branches": []bson.M{
				{
					"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "string"}},
					"then": bson.M{"$toLong": bson.M{"$dateFromString": bson.M{
						"dateString": bson.M{"$substr": bson.A{"$panCakeData.inserted_at", 0, 19}},
						"format":     "%Y-%m-%dT%H:%M:%S",
						"onError":    nil,
						"onNull":     nil,
					}}},
				},
				{
					"case": bson.M{"$and": bson.A{
						bson.M{"$ne": bson.A{"$panCakeData.inserted_at", nil}},
						bson.M{"$ne": bson.A{"$panCakeData.inserted_at", ""}},
					}},
					"then": bson.M{
						"$cond": bson.A{
							bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1e12}},
							bson.M{"$toLong": "$panCakeData.inserted_at"},
							bson.M{"$multiply": bson.A{bson.M{"$toLong": "$panCakeData.inserted_at"}, 1000}},
						},
					},
				},
			},
			"default": nil,
		},
	}
	panCakeUpdatedAtToMs := bson.M{
		"$cond": bson.A{
			bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 1e12}},
			"$panCakeUpdatedAt",
			bson.M{"$multiply": bson.A{"$panCakeUpdatedAt", 1000}},
		},
	}
	tsFromUpdatedAt := bson.M{
		"$cond": bson.A{
			bson.M{"$and": bson.A{
				bson.M{"$ne": bson.A{"$panCakeData.updated_at", nil}},
				bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 0}},
			}},
			bson.M{
				"$cond": bson.A{
					bson.M{"$gt": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 1e12}},
					bson.M{"$toLong": "$panCakeData.updated_at"},
					bson.M{"$multiply": bson.A{bson.M{"$toLong": "$panCakeData.updated_at"}, 1000}},
				},
			},
			nil,
		},
	}
	tsMs := bson.M{
		"$ifNull": bson.A{
			tsFromInsertedAt,
			bson.M{"$ifNull": bson.A{
				tsFromUpdatedAt,
				bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 0}}, panCakeUpdatedAtToMs, nil}},
			}},
		},
	}

	dayHourExpr := bson.M{
		"dayOfWeek": bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}},
		"hour":      bson.M{"$hour": bson.M{"date": "$dt", "timezone": tzVietnam}},
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			"$or": []bson.M{
				{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
				{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
				{"panCakeUpdatedAt": bson.M{"$gt": 0}},
			},
		}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id":   dayHourExpr,
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 30},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		fmt.Printf("⚠ Lỗi aggregate: %v\n\n", err)
		return
	}
	defer cursor.Close(ctx)

	fmt.Println("Top 30 khung giờ có nhiều conversation nhất:")
	fmt.Printf("%-15s %-12s %10s\n", "Ngày", "Giờ", "Số lượng")
	fmt.Println(strings.Repeat("-", 40))
	for cursor.Next(ctx) {
		var r struct {
			ID    struct { DayOfWeek int `bson:"dayOfWeek"`; Hour int `bson:"hour"` } `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		dayName := dayOfWeekNames[r.ID.DayOfWeek]
		if dayName == "" {
			dayName = fmt.Sprintf("Thứ %d", r.ID.DayOfWeek)
		}
		fmt.Printf("%-15s %02d:00-%02d:59   %10d\n", dayName, r.ID.Hour, r.ID.Hour, r.Count)
	}

	// Thống kê theo ngày trong tuần (gộp giờ) — dùng cùng tsMs logic
	pipeline2 := []bson.M{
		{"$match": bson.M{
			"$or": []bson.M{
				{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
				{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
				{"panCakeUpdatedAt": bson.M{"$gt": 0}},
			},
		}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id":   bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id": 1}},
	}
	cursor2, _ := coll.Aggregate(ctx, pipeline2)
	fmt.Println("\n📅 Conversation theo ngày trong tuần:")
	for cursor2.Next(ctx) {
		var r struct {
			ID    int `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cursor2.Decode(&r); err != nil {
			continue
		}
		dayName := dayOfWeekNames[r.ID]
		if dayName == "" {
			dayName = fmt.Sprintf("Thứ %d", r.ID)
		}
		fmt.Printf("  %s: %d\n", dayName, r.Count)
	}
	cursor2.Close(ctx)
	fmt.Println()
}

func analyzeMessagesByDayAndHour(ctx context.Context, db *mongo.Database) {
	coll := db.Collection(colFbMessageItems)
	count, _ := coll.CountDocuments(ctx, bson.M{})
	if count == 0 {
		fmt.Printf("⚠ %s: không có dữ liệu\n\n", colFbMessageItems)
		return
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("📊 2. MESSAGE (fb_message_items) - Theo ngày trong tuần & giờ")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("Tổng số message: %d\n", count)
	fmt.Printf("⏱️  Thời gian gốc: insertedAt (từ MessageData.inserted_at) | Timezone: %s\n\n", tzVietnam)

	// Thời gian GỐC: insertedAt từ MessageData.inserted_at. KHÔNG dùng createdAt (sync).
	timeField := "$insertedAt"
	tsMs := bson.M{
		"$cond": bson.A{
			bson.M{"$gt": bson.A{timeField, 1e12}},
			timeField,
			bson.M{"$multiply": bson.A{timeField, 1000}},
		},
	}

	pipeline := []bson.M{
		{"$match": bson.M{"insertedAt": bson.M{"$gt": 0}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id": bson.M{
				"dayOfWeek": bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}},
				"hour":      bson.M{"$hour": bson.M{"date": "$dt", "timezone": tzVietnam}},
			},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 30},
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		fmt.Printf("⚠ Lỗi aggregate: %v\n\n", err)
		return
	}
	defer cursor.Close(ctx)

	fmt.Println("Top 30 khung giờ có nhiều message nhất:")
	fmt.Printf("%-15s %-12s %10s\n", "Ngày", "Giờ", "Số lượng")
	fmt.Println(strings.Repeat("-", 40))
	for cursor.Next(ctx) {
		var r struct {
			ID    struct { DayOfWeek int `bson:"dayOfWeek"`; Hour int `bson:"hour"` } `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		dayName := dayOfWeekNames[r.ID.DayOfWeek]
		if dayName == "" {
			dayName = fmt.Sprintf("Thứ %d", r.ID.DayOfWeek)
		}
		fmt.Printf("%-15s %02d:00-%02d:59   %10d\n", dayName, r.ID.Hour, r.ID.Hour, r.Count)
	}
	fmt.Println()
}

func analyzeOrdersByDayAndHour(ctx context.Context, db *mongo.Database) {
	posColl := db.Collection(colPcPosOrders)
	posCount, _ := posColl.CountDocuments(ctx, bson.M{})

	if posCount == 0 {
		fmt.Printf("⚠ Không có dữ liệu đơn hàng (pc_pos_orders)\n\n")
		return
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("📊 3. ĐƠN HÀNG (pc_pos_orders) - Theo ngày & giờ")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("pc_pos_orders: %d\n", posCount)
	fmt.Printf("⏱️  Thời gian gốc: posData.inserted_at (qua insertedAt/posCreatedAt) | Timezone: %s\n\n", tzVietnam)

	// Thời gian GỐC: insertedAt, posCreatedAt extract từ posData.inserted_at. KHÔNG dùng createdAt (sync).
	timeField := bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}}
	tsMs := bson.M{
		"$cond": bson.A{
			bson.M{"$gt": bson.A{timeField, 1e12}},
			timeField,
			bson.M{"$multiply": bson.A{timeField, 1000}},
		},
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			"$or": []bson.M{
				{"insertedAt": bson.M{"$gt": 0}},
				{"posCreatedAt": bson.M{"$gt": 0}},
			},
		}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id": bson.M{
				"dayOfWeek": bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}},
				"hour":      bson.M{"$hour": bson.M{"date": "$dt", "timezone": tzVietnam}},
			},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 30},
	}

	cursor, err := posColl.Aggregate(ctx, pipeline)
	if err != nil {
		fmt.Printf("⚠ Lỗi aggregate pc_pos_orders: %v\n\n", err)
		return
	}
	defer cursor.Close(ctx)

	fmt.Println("Top 30 khung giờ có nhiều đơn hàng nhất (pc_pos_orders):")
	fmt.Printf("%-15s %-12s %10s\n", "Ngày", "Giờ", "Số lượng")
	fmt.Println(strings.Repeat("-", 40))
	for cursor.Next(ctx) {
		var r struct {
			ID    struct { DayOfWeek int `bson:"dayOfWeek"`; Hour int `bson:"hour"` } `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		dayName := dayOfWeekNames[r.ID.DayOfWeek]
		if dayName == "" {
			dayName = fmt.Sprintf("Thứ %d", r.ID.DayOfWeek)
		}
		fmt.Printf("%-15s %02d:00-%02d:59   %10d\n", dayName, r.ID.Hour, r.ID.Hour, r.Count)
	}

	// Theo ngày trong tuần — dùng cùng tsMs, timezone
	pipeline2 := []bson.M{
		{"$match": bson.M{
			"$or": []bson.M{
				{"insertedAt": bson.M{"$gt": 0}},
				{"posCreatedAt": bson.M{"$gt": 0}},
			},
		}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id":   bson.M{"$dayOfWeek": bson.M{"date": "$dt", "timezone": tzVietnam}},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id": 1}},
	}
	cursor2, _ := posColl.Aggregate(ctx, pipeline2)
	fmt.Println("\n📅 Đơn hàng theo ngày trong tuần:")
	for cursor2.Next(ctx) {
		var r struct {
			ID    int `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cursor2.Decode(&r); err != nil {
			continue
		}
		dayName := dayOfWeekNames[r.ID]
		if dayName == "" {
			dayName = fmt.Sprintf("Thứ %d", r.ID)
		}
		fmt.Printf("  %s: %d\n", dayName, r.Count)
	}
	cursor2.Close(ctx)
	fmt.Println()
}

func analyzeByMonth(ctx context.Context, db *mongo.Database) {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("📊 4. XU HƯỚNG THEO THÁNG (Conversation & Đơn hàng)")
	fmt.Println(strings.Repeat("=", 70))

	// Conversation theo tháng — thời gian gốc, timezone VN
	coll := db.Collection(colFbConversations)
	tsFromInsertedAt := bson.M{
		"$switch": bson.M{
			"branches": []bson.M{
				{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "string"}},
					"then": bson.M{"$toLong": bson.M{"$dateFromString": bson.M{
						"dateString": bson.M{"$substr": bson.A{"$panCakeData.inserted_at", 0, 19}},
						"format":     "%Y-%m-%dT%H:%M:%S",
						"onError":    nil,
						"onNull":     nil,
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
	panCakeUpdatedAtToMsMonth := bson.M{
		"$cond": bson.A{
			bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 1e12}},
			"$panCakeUpdatedAt",
			bson.M{"$multiply": bson.A{"$panCakeUpdatedAt", 1000}},
		},
	}
	tsMsConv := bson.M{
		"$ifNull": bson.A{
			tsFromInsertedAt,
			bson.M{"$ifNull": bson.A{
				tsFromUpdatedAt,
				bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 0}}, panCakeUpdatedAtToMsMonth, nil}},
			}},
		},
	}
	pipeConv := []bson.M{
		{"$match": bson.M{"$or": []bson.M{
			{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
			{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
			{"panCakeUpdatedAt": bson.M{"$gt": 0}},
		}}},
		{"$addFields": bson.M{"tsMs": tsMsConv}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": bson.M{"date": "$dt", "timezone": tzVietnam}},
				"month": bson.M{"$month": bson.M{"date": "$dt", "timezone": tzVietnam}},
			},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id.year": 1, "_id.month": 1}},
		{"$limit": 24},
	}
	cursor, _ := coll.Aggregate(ctx, pipeConv)
	fmt.Println("\nConversation theo tháng (24 tháng gần nhất):")
	for cursor.Next(ctx) {
		var r struct {
			ID    struct { Year int `bson:"year"`; Month int `bson:"month"` } `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cursor.Decode(&r); err != nil {
			continue
		}
		fmt.Printf("  %d-%02d: %d\n", r.ID.Year, r.ID.Month, r.Count)
	}
	cursor.Close(ctx)

	// Đơn hàng theo tháng — thời gian gốc, timezone VN
	posColl := db.Collection(colPcPosOrders)
	timeFieldOrder := bson.M{"$ifNull": bson.A{"$insertedAt", "$posCreatedAt"}}
	tsMsOrder := bson.M{
		"$cond": bson.A{
			bson.M{"$gt": bson.A{timeFieldOrder, 1e12}},
			timeFieldOrder,
			bson.M{"$multiply": bson.A{timeFieldOrder, 1000}},
		},
	}
	pipeOrder := []bson.M{
		{"$match": bson.M{"$or": []bson.M{
			{"insertedAt": bson.M{"$gt": 0}},
			{"posCreatedAt": bson.M{"$gt": 0}},
		}}},
		{"$addFields": bson.M{"tsMs": tsMsOrder}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{
			"_id": bson.M{
				"year":  bson.M{"$year": bson.M{"date": "$dt", "timezone": tzVietnam}},
				"month": bson.M{"$month": bson.M{"date": "$dt", "timezone": tzVietnam}},
			},
			"count": bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"_id.year": 1, "_id.month": 1}},
		{"$limit": 24},
	}
	cursor2, _ := posColl.Aggregate(ctx, pipeOrder)
	fmt.Println("\nĐơn hàng theo tháng (24 tháng gần nhất):")
	for cursor2.Next(ctx) {
		var r struct {
			ID    struct { Year int `bson:"year"`; Month int `bson:"month"` } `bson:"_id"`
			Count int `bson:"count"`
		}
		if err := cursor2.Decode(&r); err != nil {
			continue
		}
		fmt.Printf("  %d-%02d: %d\n", r.ID.Year, r.ID.Month, r.Count)
	}
	cursor2.Close(ctx)

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("✅ KẾT LUẬN: Dùng kết quả trên để cấu hình Event Calendar / Ads Budget")
	fmt.Println("   - Tăng budget vào các ngày/giờ cao điểm conversation & đơn")
	fmt.Println("   - Giảm budget vào các khung giờ thấp điểm")
	fmt.Println(strings.Repeat("=", 70))
}
