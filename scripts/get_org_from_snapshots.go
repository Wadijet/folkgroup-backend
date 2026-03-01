// Script lấy ownerOrganizationId từ report_snapshots (có snapshot).
// Chạy: go run scripts/get_org_from_snapshots.go
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
		ReportKey  string             `bson:"reportKey"`
		PeriodKey  string             `bson:"periodKey"`
	}
	err = client.Database(dbName).Collection("report_snapshots").
		FindOne(ctx, bson.M{"reportKey": bson.M{"$regex": "^customer_"}},
			options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1, "reportKey": 1, "periodKey": 1})).Decode(&doc)
	if err != nil {
		log.Fatal("Không tìm thấy snapshot customer trong report_snapshots: ", err)
	}
	fmt.Printf("OrgID: %s\nReportKey: %s\nPeriodKey: %s\n", doc.OwnerOrgID.Hex(), doc.ReportKey, doc.PeriodKey)
}
