// Script gộp lịch sử đơn hàng trùng — giữ 1 bản ghi/đơn (ưu tiên order_completed > order_cancelled > order_created).
// Chạy: go run scripts/dedupe_order_activities.go
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
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	coll := db.Collection("crm_activity_history")

	// Lấy tất cả order activities (domain = order)
	cursor, err := coll.Find(ctx, bson.M{"domain": "order"}, options.Find().SetProjection(bson.M{
		"_id": 1, "unifiedId": 1, "ownerOrganizationId": 1, "activityType": 1,
		"sourceRef": 1, "activityAt": 1, "metadata": 1, "displayLabel": 1, "displayIcon": 1, "displaySubtext": 1,
	}))
	if err != nil {
		log.Fatalf("Find lỗi: %v", err)
	}

	type act struct {
		ID        primitive.ObjectID `bson:"_id"`
		UnifiedId string             `bson:"unifiedId"`
		OwnerOrg  primitive.ObjectID `bson:"ownerOrganizationId"`
		Type      string             `bson:"activityType"`
		SourceRef bson.M             `bson:"sourceRef"`
		ActivityAt int64            `bson:"activityAt"`
		Metadata  bson.M             `bson:"metadata"`
		DisplayLabel string          `bson:"displayLabel"`
		DisplayIcon  string          `bson:"displayIcon"`
		DisplaySubtext string        `bson:"displaySubtext"`
	}

	var all []act
	if err := cursor.All(ctx, &all); err != nil {
		log.Fatalf("Decode lỗi: %v", err)
	}
	cursor.Close(ctx)

	// Chuẩn hóa orderId sang int64 (tránh int32 vs int64 khác key)
	toInt64 := func(v interface{}) int64 {
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
		default:
			return 0
		}
	}

	// Nhóm theo (unifiedId, ownerOrgId, orderId)
	type key struct {
		u, o string
		oid  int64
	}
	groups := make(map[key][]act)
	for _, a := range all {
		oid := toInt64(a.SourceRef["orderId"])
		if oid == 0 {
			continue
		}
		k := key{a.UnifiedId, a.OwnerOrg.Hex(), oid}
		groups[k] = append(groups[k], a)
	}

	// Ưu tiên: order_completed > order_cancelled > order_created
	priority := map[string]int{"order_completed": 3, "order_cancelled": 2, "order_created": 1}

	deleted := 0
	for k, arr := range groups {
		if len(arr) <= 1 {
			continue
		}
		// Chọn bản ghi giữ lại (ưu tiên type cao nhất, rồi activityAt mới nhất)
		var keep act
		keepPrio := 0
		for _, a := range arr {
			p := priority[a.Type]
			if p > keepPrio || (p == keepPrio && a.ActivityAt > keep.ActivityAt) {
				keep = a
				keepPrio = p
			}
		}
		// Xóa các bản còn lại
		for _, a := range arr {
			if a.ID == keep.ID {
				continue
			}
			_, err := coll.DeleteOne(ctx, bson.M{"_id": a.ID})
			if err != nil {
				log.Printf("Xóa %v lỗi: %v", a.ID, err)
			} else {
				deleted++
				if deleted <= 20 {
					fmt.Printf("Đã xóa: unifiedId=%s orderId=%d type=%s (giữ %s)\n", k.u, k.oid, a.Type, keep.Type)
				}
			}
		}
	}

	fmt.Printf("\n✓ Đã xóa %d bản ghi trùng\n", deleted)
}
