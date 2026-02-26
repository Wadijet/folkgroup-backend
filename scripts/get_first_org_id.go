// Script lấy ownerOrganizationId đầu tiên từ fb_conversations.
// Chạy: go run scripts/get_first_org_id.go
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	var doc struct {
		OwnerOrgID primitive.ObjectID `bson:"ownerOrganizationId"`
	}
	err = client.Database(dbName).Collection("fb_conversations").
		FindOne(ctx, bson.M{"ownerOrganizationId": bson.M{"$exists": true, "$ne": primitive.NilObjectID}},
			options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&doc)
	if err != nil {
		// Thử auth_organizations
		var org struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		err = client.Database(dbName).Collection("auth_organizations").
			FindOne(ctx, bson.M{}, options.FindOne().SetProjection(bson.M{"_id": 1})).Decode(&org)
		if err != nil {
			log.Fatal("Không tìm thấy ownerOrganizationId từ fb_conversations hoặc auth_organizations")
		}
		fmt.Print(org.ID.Hex())
		return
	}
	fmt.Print(doc.OwnerOrgID.Hex())
}
