// Script chẩn đoán: Kiểm tra pc_pos_orders và mapping variation_id cho Inventory API.
// Chạy từ thư mục api: go run ../scripts/diagnose_inventory_orders.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"meta_commerce/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	colOrders     = "pc_pos_orders"
	colVariations = "pc_pos_variations"
)

func main() {
	fmt.Println("=== Chẩn đoán Inventory Orders ===\n")

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

	// Orders có thể nằm trong folkform_auth (theo user)
	dbName := cfg.MongoDB_DBName_Auth
	if dbName == "" {
		dbName = cfg.MongoDB_DBName_Data
	}
	db := client.Database(dbName)
	fmt.Printf("   DB: %s\n", dbName)
	ordersColl := db.Collection(colOrders)
	variationsColl := db.Collection(colVariations)

	// 1. Lấy 1 order mẫu, in cấu trúc posData.items
	fmt.Println("1. Cấu trúc 1 order mẫu (posData.items):")
	var sampleOrder struct {
		ID       primitive.ObjectID     `bson:"_id"`
		OrderItems interface{}          `bson:"orderItems"`
		PosData   map[string]interface{} `bson:"posData"`
	}
	err = ordersColl.FindOne(ctx, bson.M{}).Decode(&sampleOrder)
	if err != nil {
		fmt.Printf("   Lỗi: %v\n", err)
	} else {
		fmt.Printf("   orderItems type: %T, is nil: %v\n", sampleOrder.OrderItems, sampleOrder.OrderItems == nil)
		// Kiểm tra orderItems (dùng trước posData.items trong API)
		var arr []interface{}
		switch v := sampleOrder.OrderItems.(type) {
		case []interface{}:
			arr = v
		case primitive.A:
			arr = v
		default:
			fmt.Printf("   orderItems không phải []interface{}: %T\n", sampleOrder.OrderItems)
		}
		if len(arr) > 0 {
			first := arr[0]
			m := toMap(first)
			if m != nil {
				fmt.Println("   orderItems[0] keys:")
				for k := range m {
					fmt.Printf("     - %q\n", k)
				}
				vid := getVariationIdFromItem(m)
				fmt.Printf("   getVariationIdFromItem => %q\n", vid)
			} else {
				fmt.Printf("   orderItems[0] type: %T (không convert được)\n", first)
			}
		} else {
			fmt.Printf("   orderItems len=%d\n", len(arr))
		}
		if sampleOrder.PosData != nil {
			items, _ := sampleOrder.PosData["items"]
			fmt.Printf("   posData.items type: %T\n", items)
		}
	}

	// 2. Đếm orders có ownerOrganizationId
	fmt.Println("\n2. Thống kê orders theo ownerOrganizationId:")
	pipe := []bson.M{
		{"$group": bson.M{"_id": "$ownerOrganizationId", "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"count": -1}},
		{"$limit": 5},
	}
	cursor, err := ordersColl.Aggregate(ctx, pipe)
	if err != nil {
		fmt.Printf("   Lỗi: %v\n", err)
	} else {
		for cursor.Next(ctx) {
			var r bson.M
			if err := cursor.Decode(&r); err == nil {
				fmt.Printf("   org %v: %v orders\n", r["_id"], r["count"])
			}
		}
		cursor.Close(ctx)
	}

	// 3. Với productId Khăn Cố Đô Liên Họa, lấy variation IDs
	productID := "69ea0157-1753-4c78-8d0b-72b0fbe1f73b"
	fmt.Printf("\n3. Variations của product %s:\n", productID)
	var variations []struct {
		VariationId           string              `bson:"variationId"`
		Sku                   string              `bson:"sku"`
		OwnerOrganizationId   primitive.ObjectID  `bson:"ownerOrganizationId"`
	}
	cur, err := variationsColl.Find(ctx, bson.M{"productId": productID}, options.Find().SetProjection(bson.M{"variationId": 1, "sku": 1, "ownerOrganizationId": 1}))
	if err != nil {
		fmt.Printf("   Lỗi: %v\n", err)
	} else {
		_ = cur.All(ctx, &variations)
		cur.Close(ctx)
		variationIDs := make([]string, 0, len(variations)) // local for step 4
		for _, v := range variations {
			variationIDs = append(variationIDs, v.VariationId)
			fmt.Printf("   - %s (sku: %s) ownerOrg: %s\n", v.VariationId, v.Sku, v.OwnerOrganizationId.Hex())
		}

		// 4. Tìm orders có chứa variation_id trong posData.items
		if len(variationIDs) > 0 {
			fmt.Printf("\n4. Tìm orders có variation_id trong posData.items (không filter org):\n")
			// Aggregate: unwind items, match variation_id
			matchVar := bson.M{"$in": variationIDs}
			pipe2 := []bson.M{
				{"$match": bson.M{"posData.items": bson.M{"$exists": true, "$ne": nil}}},
				{"$unwind": "$posData.items"},
				{"$match": bson.M{"posData.items.variation_id": matchVar}},
				{"$group": bson.M{"_id": "$_id", "orderId": bson.M{"$first": "$orderId"}, "ownerOrg": bson.M{"$first": "$ownerOrganizationId"}}},
				{"$limit": 5},
			}
			cursor2, err := ordersColl.Aggregate(ctx, pipe2)
			if err != nil {
				fmt.Printf("   Lỗi: %v\n", err)
			} else {
				count := 0
				for cursor2.Next(ctx) {
					var r bson.M
					if err := cursor2.Decode(&r); err == nil {
						count++
						b, _ := json.MarshalIndent(r, "   ", "  ")
						fmt.Printf("   %s\n", string(b))
					}
				}
				cursor2.Close(ctx)
				if count == 0 {
					fmt.Println("   Không tìm thấy order nào có variation_id trong posData.items")
					// Thử với variationId (camelCase)
					pipe3 := []bson.M{
						{"$match": bson.M{"posData.items": bson.M{"$exists": true, "$ne": nil}}},
						{"$unwind": "$posData.items"},
						{"$match": bson.M{"posData.items.variationId": matchVar}},
						{"$limit": 5},
					}
					cursor3, _ := ordersColl.Aggregate(ctx, pipe3)
					if cursor3 != nil {
						cnt := 0
						for cursor3.Next(ctx) {
							cnt++
						}
						cursor3.Close(ctx)
						if cnt > 0 {
							fmt.Printf("   Nhưng tìm thấy %d với posData.items.variationId (camelCase)\n", cnt)
						}
					}
				}
			}
		}
	}

	// 5. Mô phỏng aggregateDailySales với org và 90 ngày (filter dùng cho step 6)
	fmt.Println("\n5. Mô phỏng aggregateDailySales (org 698c341c977ebc6295312ad8, 90 ngày):")
	ownerOrgID, _ := primitive.ObjectIDFromHex("698c341c977ebc6295312ad8")
	now := time.Now()
	fromTime := now.AddDate(0, 0, -90)
	toTime := now
	fromSec := fromTime.Unix()
	toSec := toTime.Unix()
	fromMs := fromSec * 1000
	toMs := toSec * 1000
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{"$or": []bson.M{
				{"insertedAt": bson.M{"$gte": fromSec, "$lte": toSec}},
				{"posCreatedAt": bson.M{"$gte": fromSec, "$lte": toSec}},
				{"insertedAt": bson.M{"$gte": fromMs, "$lte": toMs}},
				{"posCreatedAt": bson.M{"$gte": fromMs, "$lte": toMs}},
			}},
			{"posData.status": bson.M{"$nin": []int{6, 7}}},
			{"status": bson.M{"$nin": []int{6, 7}}},
		},
	}
	countMatch, _ := ordersColl.CountDocuments(ctx, filter)
	fmt.Printf("   Orders match filter (90d, status != 6,7): %d\n", countMatch)
	// Lấy 1 order trong range, xem insertedAt
	var sampleInRange struct {
		InsertedAt   interface{} `bson:"insertedAt"`
		PosCreatedAt interface{}  `bson:"posCreatedAt"`
	}
	_ = ordersColl.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"insertedAt": 1, "posCreatedAt": 1})).Decode(&sampleInRange)
	if sampleInRange.InsertedAt != nil {
		fmt.Printf("   Sample insertedAt: %v (type %T)\n", sampleInRange.InsertedAt, sampleInRange.InsertedAt)
	}

	// 6. Mô phỏng aggregate: iterate orders, extract variation_id, đếm cho product variations
	variationIDs := getVariationIDsForProduct(ctx, variationsColl, productID)
	if len(variationIDs) > 0 {
		fmt.Println("\n6. Mô phỏng aggregateDailySales - đếm qty theo variation:")
		opts := options.Find().SetProjection(bson.M{"orderItems": 1, "posData": 1})
		cursor, _ := ordersColl.Find(ctx, filter, opts)
		totalByVar := make(map[string]int64)
		for cursor.Next(ctx) {
			var doc struct {
				OrderItems interface{}          `bson:"orderItems"`
				PosData    map[string]interface{} `bson:"posData"`
			}
			if err := cursor.Decode(&doc); err != nil {
				continue
			}
			items := extractOrderItems(doc.OrderItems, doc.PosData)
			for _, it := range items {
				vid := getVariationIdFromItem(it)
				if vid == "" {
					continue
				}
				for _, targetVid := range variationIDs {
					if vid == targetVid {
						qty := getQty(it)
						totalByVar[vid] += qty
						break
					}
				}
			}
		}
		cursor.Close(ctx)
		for vid, total := range totalByVar {
			daily := float64(total) / 90
			fmt.Printf("   %s: total=%d, dailySales=%.4f\n", vid, total, daily)
		}
		if len(totalByVar) == 0 {
			fmt.Println("   Không có variation nào có sales trong 90 ngày!")
		}
	}

	// 7. Kiểm tra order có orderItems không null
	fmt.Println("\n7. Orders có orderItems != null:")
	count, _ := ordersColl.CountDocuments(ctx, bson.M{"orderItems": bson.M{"$ne": nil, "$exists": true}})
	fmt.Printf("   Số orders có orderItems: %d\n", count)

	totalOrders, _ := ordersColl.CountDocuments(ctx, bson.M{})
	fmt.Printf("   Tổng orders: %d\n", totalOrders)

	// 8. Lấy role ID cho org (để gọi API)
	fmt.Println("\n8. Role cho org 698c341c977ebc6295312ad8:")
	rolesColl := db.Collection("auth_roles")
	curRole, _ := rolesColl.Find(ctx, bson.M{"ownerOrganizationId": ownerOrgID}, options.Find().SetLimit(1).SetProjection(bson.M{"_id": 1, "name": 1}))
	for curRole.Next(ctx) {
		var r struct {
			ID   primitive.ObjectID `bson:"_id"`
			Name string             `bson:"name"`
		}
		if curRole.Decode(&r) == nil {
			fmt.Printf("   Role: %s (%s) - dùng X-Active-Role-ID: %s\n", r.Name, r.ID.Hex(), r.ID.Hex())
		}
	}
	curRole.Close(ctx)

	fmt.Println("\n=== Kết thúc chẩn đoán ===")
}

func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	if d, ok := v.(primitive.D); ok {
		m := make(map[string]interface{}, len(d))
		for _, e := range d {
			m[e.Key] = e.Value
		}
		return m
	}
	return nil
}

func getVariationIDsForProduct(ctx context.Context, coll *mongo.Collection, productID string) []string {
	var variations []struct {
		VariationId string `bson:"variationId"`
	}
	cur, err := coll.Find(ctx, bson.M{"productId": productID}, options.Find().SetProjection(bson.M{"variationId": 1}))
	if err != nil {
		return nil
	}
	_ = cur.All(ctx, &variations)
	cur.Close(ctx)
	ids := make([]string, 0, len(variations))
	for _, v := range variations {
		ids = append(ids, v.VariationId)
	}
	return ids
}

func extractOrderItems(orderItems interface{}, posData map[string]interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	appendItem := func(v interface{}) {
		if m := toMap(v); m != nil {
			out = append(out, m)
		}
	}
	if arr, ok := orderItems.([]interface{}); ok {
		for _, v := range arr {
			appendItem(v)
		}
	}
	if arr, ok := orderItems.(primitive.A); ok {
		for _, v := range arr {
			appendItem(v)
		}
	}
	if len(out) > 0 {
		return out
	}
	if posData != nil {
		if arr, ok := posData["items"].([]interface{}); ok {
			for _, v := range arr {
				appendItem(v)
			}
		}
	}
	return out
}

func getQty(it map[string]interface{}) int64 {
	if v, ok := it["quantity"]; ok && v != nil {
		switch x := v.(type) {
		case int64:
			return x
		case int:
			return int64(x)
		case float64:
			return int64(x)
		}
	}
	return 0
}

func getVariationIdFromItem(it map[string]interface{}) string {
	for _, k := range []string{"variation_id", "variationId"} {
		if v, ok := it[k]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
	}
	if infoRaw, ok := it["variation_info"]; ok && infoRaw != nil {
		info := toMap(infoRaw)
		if info != nil {
			for _, k := range []string{"variation_id", "id", "variationId"} {
				if v, ok := info[k]; ok && v != nil {
					if s, ok := v.(string); ok {
						return s
					}
					return fmt.Sprintf("%v", v)
				}
			}
		}
	}
	return ""
}
