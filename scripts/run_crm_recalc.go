// Script đưa job recalculate CRM vào queue crm_bulk_jobs.
// Worker (chạy cùng server) sẽ pick job và gọi RecalculateAllCustomers — cập nhật crm_customers + ghi activity customer_updated với metricsSnapshot mới.
//
// Chạy: go run scripts/run_crm_recalc.go [ownerOrganizationId]
//       go run scripts/run_crm_recalc.go 69a655f0088600c32e62f955
//       go run scripts/run_crm_recalc.go 69a655f0088600c32e62f955 1000  (limit 1000 khách)
//
// Lưu ý: Cần server đang chạy (có CrmBulkWorker) để job được xử lý. Worker poll mỗi 2 phút.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	var orgID primitive.ObjectID
	if len(os.Args) >= 2 {
		var parseErr error
		orgID, parseErr = primitive.ObjectIDFromHex(os.Args[1])
		if parseErr != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", parseErr)
		}
	} else {
		var doc struct {
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if db.Collection("crm_customers").FindOne(ctx, bson.M{}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&doc) != nil {
			log.Fatal("Không tìm thấy org. Chạy với: go run scripts/run_crm_recalc.go <ownerOrganizationId>")
		}
		orgID = doc.OwnerOrganizationID
		fmt.Printf("Dùng ownerOrganizationId từ crm_customers: %s\n", orgID.Hex())
	}

	limit := 0
	if len(os.Args) >= 3 {
		limit, _ = strconv.Atoi(os.Args[2])
	}

	coll := db.Collection("crm_bulk_jobs")

	now := time.Now().Unix()
	doc := bson.M{
		"jobType":             "recalculate_all",
		"ownerOrganizationId": orgID,
		"params":              bson.M{"limit": limit},
		"createdAt":           now,
	}
	res, err := coll.InsertOne(ctx, doc)
	if err != nil {
		log.Fatalf("Insert job thất bại: %v", err)
	}

	jobID := res.InsertedID.(primitive.ObjectID)
	fmt.Printf("\n✓ Đã thêm job recalculate_all vào queue\n")
	fmt.Printf("  Job ID: %s\n", jobID.Hex())
	fmt.Printf("  Org: %s\n", orgID.Hex())
	fmt.Printf("  Limit: %d (0 = tất cả)\n", limit)
	fmt.Printf("\n  Worker chạy mỗi 2 phút. Đảm bảo server đang chạy để job được xử lý.\n")
	fmt.Printf("  Kiểm tra: db.crm_bulk_jobs.findOne({_id: ObjectId(\"%s\")})\n", jobID.Hex())
}
