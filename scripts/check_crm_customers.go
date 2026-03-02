// Script kiểm tra crm_customers có customer với UUID từ 13 conversations.
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	actColl := db.Collection("crm_activity_history")

	orgID, _ := primitive.ObjectIDFromHex("698c341c977ebc6295312ad8")
	uuids := []string{
		"c612ff16-a513-4afe-9bb8-0bfc15be0267", "bc783602-7cc9-48a3-b1a7-b60cdf7f9080",
		"69a20a01-472c-42fc-9422-d7f032aaf4b2", "924fb7b9-89dc-44ad-b56a-23be5588bff6",
	}

	fmt.Println("=== CRM_CUSTOMERS (mẫu 4 UUID) ===")
	for _, uid := range uuids {
		n, _ := crmColl.CountDocuments(ctx, bson.M{
			"ownerOrganizationId": orgID,
			"$or": []bson.M{
				{"unifiedId": uid},
				{"sourceIds.fb": uid},
			},
		})
		fmt.Printf("  %s: %d\n", uid, n)
	}

	total, _ := crmColl.CountDocuments(ctx, bson.M{"ownerOrganizationId": orgID})
	convActs, _ := actColl.CountDocuments(ctx, bson.M{
		"ownerOrganizationId": orgID,
		"activityType":        "conversation_started",
	})
	fmt.Printf("\nTổng crm_customers (org): %d\n", total)
	fmt.Printf("Tổng crm_activity_history (conversation_started): %d\n", convActs)

	// Mẫu 3 crm_customers mới nhất (source fb)
	var samples []struct {
		UnifiedId string `bson:"unifiedId"`
		SourceIds struct {
			Fb string `bson:"fb"`
			Pos string `bson:"pos"`
		} `bson:"sourceIds"`
		PrimarySource string `bson:"primarySource"`
	}
	cursor, _ := crmColl.Find(ctx, bson.M{"ownerOrganizationId": orgID, "primarySource": "fb"}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(5))
	cursor.All(ctx, &samples)
	fmt.Println("\nMẫu crm_customers (primarySource=fb):")
	for i, s := range samples {
		fmt.Printf("  [%d] unifiedId=%s sourceIds.fb=%s\n", i+1, s.UnifiedId, s.SourceIds.Fb)
	}
}
