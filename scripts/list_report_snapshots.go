// Liệt kê tất cả customer snapshots trong DB.
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

	cursor, err := client.Database(dbName).Collection("report_snapshots").
		Find(ctx, bson.M{"reportKey": bson.M{"$regex": "^customer_"}},
			options.Find().SetProjection(bson.M{"reportKey": 1, "periodKey": 1, "ownerOrganizationId": 1}).SetSort(bson.M{"periodKey": -1}).SetLimit(20))
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	fmt.Println("=== Customer snapshots (20 mới nhất) ===")
	count := 0
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		count++
		fmt.Printf("%d. reportKey=%v periodKey=%v ownerOrgId=%v\n", count, doc["reportKey"], doc["periodKey"], doc["ownerOrganizationId"])
	}
	if count == 0 {
		fmt.Println("Không có snapshot nào")
	}
}
