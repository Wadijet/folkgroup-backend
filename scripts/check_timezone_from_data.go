// Script kiểm tra múi giờ của dữ liệu gốc qua phân bố conversation theo giờ.
// So sánh UTC vs Asia/Ho_Chi_Minh: nếu peak giờ VN (8-17) khớp với giờ làm việc → data có thể đã đúng UTC.
// Nếu peak UTC (8-17) mà peak VN lệch 7h → data có thể là Vietnam time bị parse nhầm thành UTC.
//
// Chạy từ thư mục api: go run ../scripts/check_timezone_from_data.go
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
	tzVietnam           = "Asia/Ho_Chi_Minh"
)

func main() {
	fmt.Println("=== Kiểm Tra Múi Giờ Dữ Liệu Gốc (qua Conversation) ===\n")
	fmt.Println("Bạn đang ở UTC+7. So sánh phân bố theo giờ:")
	fmt.Println("  • UTC: nếu peak 01:00-06:00 → tương ứng 08:00-13:00 VN (hợp lý)")
	fmt.Println("  • VN: nếu peak 08:00-17:00 → giờ làm việc (hợp lý)")
	fmt.Println("  • Nếu peak UTC 08:00-13:00 mà peak VN 15:00-20:00 → có thể data là VN time bị parse nhầm UTC\n")

	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(cfg.MongoDB_DBName_Auth)
	coll := db.Collection(colFbConversations)

	// tsMs: lấy từ panCakeData.inserted_at, updated_at, panCakeUpdatedAt
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
	panCakeUpdatedAtToMs := bson.M{
		"$cond": bson.A{bson.M{"$gt": bson.A{"$panCakeUpdatedAt", 1e12}}, "$panCakeUpdatedAt", bson.M{"$multiply": bson.A{"$panCakeUpdatedAt", 1000}}},
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

	// 1. Phân bố theo giờ UTC (không timezone)
	pipeUTC := []bson.M{
		{"$match": bson.M{"$or": []bson.M{
			{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
			{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
			{"panCakeUpdatedAt": bson.M{"$gt": 0}},
		}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{"_id": bson.M{"$hour": "$dt"}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id": 1}},
	}

	// 2. Phân bố theo giờ Asia/Ho_Chi_Minh
	pipeVN := []bson.M{
		{"$match": bson.M{"$or": []bson.M{
			{"panCakeData.inserted_at": bson.M{"$exists": true, "$ne": nil}},
			{"panCakeData.updated_at": bson.M{"$exists": true, "$ne": nil}},
			{"panCakeUpdatedAt": bson.M{"$gt": 0}},
		}}},
		{"$addFields": bson.M{"tsMs": tsMs}},
		{"$match": bson.M{"tsMs": bson.M{"$gt": 0, "$ne": nil}}},
		{"$addFields": bson.M{"dt": bson.M{"$toDate": "$tsMs"}}},
		{"$group": bson.M{"_id": bson.M{"$hour": bson.M{"date": "$dt", "timezone": tzVietnam}}, "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"_id": 1}},
	}

	// Chạy cả hai
	hourCountUTC := make(map[int]int64)
	cursorUTC, _ := coll.Aggregate(ctx, pipeUTC)
	for cursorUTC.Next(ctx) {
		var r struct {
			ID    int   `bson:"_id"`
			Count int64 `bson:"count"`
		}
		if err := cursorUTC.Decode(&r); err == nil {
			hourCountUTC[r.ID] = r.Count
		}
	}
	cursorUTC.Close(ctx)

	hourCountVN := make(map[int]int64)
	cursorVN, _ := coll.Aggregate(ctx, pipeVN)
	for cursorVN.Next(ctx) {
		var r struct {
			ID    int   `bson:"_id"`
			Count int64 `bson:"count"`
		}
		if err := cursorVN.Decode(&r); err == nil {
			hourCountVN[r.ID] = r.Count
		}
	}
	cursorVN.Close(ctx)

	// In bảng so sánh
	fmt.Println(strings.Repeat("=", 75))
	fmt.Printf("%-8s %12s %12s %-40s\n", "Giờ", "UTC", "VN (+7)", "Ghi chú")
	fmt.Println(strings.Repeat("=", 75))

	var totalUTC, totalVN int64
	for h := 0; h < 24; h++ {
		cUTC := hourCountUTC[h]
		cVN := hourCountVN[h]
		totalUTC += cUTC
		totalVN += cVN

		note := ""
		if h >= 1 && h <= 6 {
			note = "← 01-06 UTC = 08-13 VN (sáng)"
		} else if h >= 8 && h <= 17 {
			note = "← Giờ làm việc VN"
		} else if h >= 0 && h <= 7 {
			note = "← Đêm/sáng sớm VN"
		} else if h >= 18 && h <= 23 {
			note = "← Tối VN"
		}

		fmt.Printf("%02d:00    %12d %12d %s\n", h, cUTC, cVN, note)
	}
	fmt.Println(strings.Repeat("-", 75))
	fmt.Printf("%-8s %12d %12d\n", "Tổng", totalUTC, totalVN)
	fmt.Println(strings.Repeat("=", 75))

	// Tìm peak
	var peakUTC, peakVN int
	var maxUTC, maxVN int64
	for h := 0; h < 24; h++ {
		if hourCountUTC[h] > maxUTC {
			maxUTC = hourCountUTC[h]
			peakUTC = h
		}
		if hourCountVN[h] > maxVN {
			maxVN = hourCountVN[h]
			peakVN = h
		}
	}

	fmt.Println("\n📌 PHÂN TÍCH:")
	fmt.Printf("  • Peak giờ UTC:     %02d:00 (%d conversations)\n", peakUTC, maxUTC)
	fmt.Printf("  • Peak giờ VN (+7): %02d:00 (%d conversations)\n", peakVN, maxVN)

	diff := (peakVN - peakUTC + 24) % 24
	if diff > 12 {
		diff = 24 - diff
	}
	fmt.Printf("  • Chênh lệch:       %d giờ (VN = UTC + 7)\n", diff)

	fmt.Println("\n📋 KẾT LUẬN:")
	if peakVN >= 8 && peakVN <= 17 {
		fmt.Println("  ✅ Peak giờ VN (08-17) nằm trong giờ làm việc → dữ liệu có vẻ ĐÚNG.")
		fmt.Println("     Unix lưu đúng UTC, hiển thị Asia/Ho_Chi_Minh cho kết quả hợp lý.")
	} else if peakUTC >= 8 && peakUTC <= 17 && (peakVN < 8 || peakVN > 17) {
		fmt.Println("  ⚠️  Peak UTC (08-17) nhưng peak VN lệch → có thể dữ liệu gốc là giờ VN")
		fmt.Println("     bị parse nhầm thành UTC. Cần ParseInLocation(Asia/Ho_Chi_Minh) khi sync.")
	} else if peakUTC >= 1 && peakUTC <= 6 {
		fmt.Println("  ✅ Peak UTC 01-06 tương ứng 08-13 VN → dữ liệu có vẻ đúng UTC.")
	} else {
		fmt.Println("  ❓ Phân bố không rõ ràng. Kiểm tra thêm mẫu raw inserted_at trong DB.")
	}
}
