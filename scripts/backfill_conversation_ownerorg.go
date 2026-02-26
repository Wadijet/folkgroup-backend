// Script cập nhật ownerOrganizationId cho fb_conversations thiếu (từ fb_pages).
// Sau khi chạy, gọi POST /api/v1/customers/backfill-activity để tạo activity conversation_started.
// Chạy: go run scripts/backfill_conversation_ownerorg.go
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

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	convColl := db.Collection("fb_conversations")
	pageColl := db.Collection("fb_pages")

	// Lấy map pageId -> ownerOrganizationId từ fb_pages
	pagesCursor, err := pageColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"pageId": 1, "ownerOrganizationId": 1}))
	if err != nil {
		log.Fatalf("Find fb_pages lỗi: %v", err)
	}
	pageToOrg := make(map[string]primitive.ObjectID)
	for pagesCursor.Next(ctx) {
		var p struct {
			PageId   string             `bson:"pageId"`
			OwnerOrg primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if err := pagesCursor.Decode(&p); err != nil {
			continue
		}
		if p.PageId != "" && !p.OwnerOrg.IsZero() {
			pageToOrg[p.PageId] = p.OwnerOrg
		}
	}
	pagesCursor.Close(ctx)
	log.Printf("Đã load %d page -> ownerOrg từ fb_pages", len(pageToOrg))

	// Tìm conversations thiếu ownerOrganizationId (hoặc zero/empty) và có pageId
	filter := bson.M{
		"$or": []bson.M{
			{"ownerOrganizationId": bson.M{"$exists": false}},
			{"ownerOrganizationId": primitive.NilObjectID},
			{"ownerOrganizationId": ""},
		},
		"pageId": bson.M{"$exists": true, "$ne": ""},
	}
	cursor, err := convColl.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 1, "conversationId": 1, "pageId": 1}))
	if err != nil {
		log.Fatalf("Find fb_conversations lỗi: %v", err)
	}

	updated := 0
	skipped := 0
	for cursor.Next(ctx) {
		var doc struct {
			ID             primitive.ObjectID `bson:"_id"`
			ConversationId string             `bson:"conversationId"`
			PageId         string             `bson:"pageId"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		ownerOrg, ok := pageToOrg[doc.PageId]
		if !ok {
			skipped++
			continue
		}
		_, err := convColl.UpdateOne(ctx, bson.M{"_id": doc.ID}, bson.M{"$set": bson.M{"ownerOrganizationId": ownerOrg}})
		if err != nil {
			log.Printf("Update conversation %s lỗi: %v", doc.ConversationId, err)
			continue
		}
		updated++
	}
	cursor.Close(ctx)

	log.Printf("Hoàn tất: cập nhật %d, bỏ qua %d (không tìm thấy page)", updated, skipped)
	fmt.Println("Bước tiếp theo: gọi POST /api/v1/customers/backfill-activity với ownerOrganizationId để tạo activity conversation_started.")
}
