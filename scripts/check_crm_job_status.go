// Script kiểm tra trạng thái job trong crm_bulk_jobs.
//
// Chạy: go run scripts/check_crm_job_status.go
//       go run scripts/check_crm_job_status.go <jobId>   (chi tiết 1 job)
//
// Recalculate 43k KH: mỗi KH ~100-500ms → ước tính 1-6 giờ. Job chỉ có processedAt khi XONG hết.
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

func formatTs(ts interface{}) string {
	if ts == nil {
		return "-"
	}
	var sec int64
	switch x := ts.(type) {
	case int64:
		sec = x
	case int32:
		sec = int64(x)
	case float64:
		sec = int64(x)
	default:
		return fmt.Sprintf("%v", ts)
	}
	if sec == 0 {
		return "-"
	}
	t := time.Unix(sec, 0)
	return t.Format("2006-01-02 15:04:05")
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
	coll := db.Collection("crm_bulk_jobs")

	if len(os.Args) >= 2 {
		// Chi tiết 1 job
		jobID, err := primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("Job ID không hợp lệ: %v", err)
		}
		var doc bson.M
		if coll.FindOne(ctx, bson.M{"_id": jobID}).Decode(&doc) != nil {
			log.Fatal("Không tìm thấy job")
		}
		fmt.Printf("=== Job %s ===\n\n", jobID.Hex())
		fmt.Printf("  jobType:    %v\n", doc["jobType"])
		fmt.Printf("  orgId:      %v\n", doc["ownerOrganizationId"])
		fmt.Printf("  params:     %v\n", doc["params"])
		fmt.Printf("  createdAt:  %s\n", formatTs(doc["createdAt"]))
		fmt.Printf("  processedAt: %s\n", formatTs(doc["processedAt"]))
		if errStr, ok := doc["processError"].(string); ok && errStr != "" {
			fmt.Printf("  processError: %s\n", errStr)
		}
		if result, ok := doc["result"].(bson.M); ok && result != nil {
			fmt.Printf("  result:\n")
			if v, ok := result["totalProcessed"]; ok {
				fmt.Printf("    totalProcessed: %v\n", v)
			}
			if v, ok := result["totalFailed"]; ok {
				fmt.Printf("    totalFailed: %v\n", v)
			}
			if v, ok := result["failedIds"]; ok {
				fmt.Printf("    failedIds: %v\n", v)
			}
		}
		status := "ĐANG CHỜ"
		if doc["processedAt"] != nil {
			status = "ĐÃ XONG"
			if errStr, ok := doc["processError"].(string); ok && errStr != "" {
				status = "ĐÃ XONG (LỖI)"
			}
		}
		fmt.Printf("\n  Trạng thái: %s\n", status)
		return
	}

	// Tổng quan: đang chờ + mới xong
	fmt.Println("=== crm_bulk_jobs — tổng quan ===\n")

	total, _ := coll.CountDocuments(ctx, bson.M{})
	fmt.Printf("Tổng số job: %d\n", total)

	pending, _ := coll.CountDocuments(ctx, bson.M{"processedAt": nil})
	fmt.Printf("Đang chờ (processedAt=null): %d\n", pending)

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(10)
	cursor, _ := coll.Find(ctx, bson.M{"processedAt": nil}, opts)
	var pendingList []bson.M
	cursor.All(ctx, &pendingList)
	cursor.Close(ctx)
	if len(pendingList) > 0 {
		fmt.Println("\n  Job đang chờ (mới nhất):")
		for _, d := range pendingList {
			fmt.Printf("    %s | %s | org=%v | created %s\n",
				d["_id"], d["jobType"], d["ownerOrganizationId"], formatTs(d["createdAt"]))
		}
	}

	recentDone, _ := coll.Find(ctx, bson.M{"processedAt": bson.M{"$ne": nil}}, options.Find().SetSort(bson.D{{Key: "processedAt", Value: -1}}).SetLimit(5))
	var doneList []bson.M
	recentDone.All(ctx, &doneList)
	recentDone.Close(ctx)
	if len(doneList) > 0 {
		fmt.Println("\nJob vừa xong (5 gần nhất):")
		for _, d := range doneList {
			errStr := ""
			if e, ok := d["processError"].(string); ok && e != "" {
				errStr = " [LỖI]"
			}
			r := ""
			if res, ok := d["result"].(bson.M); ok {
				if p, ok := res["totalProcessed"]; ok {
					r = fmt.Sprintf(" processed=%v", p)
				}
				if f, ok := res["totalFailed"]; ok {
					r += fmt.Sprintf(" failed=%v", f)
				}
			}
			fmt.Printf("  %s | %s | done %s%s%s\n",
				d["_id"], d["jobType"], formatTs(d["processedAt"]), errStr, r)
		}
	}

	fmt.Println("\n--- Lưu ý ---")
	fmt.Println("Recalculate 43k KH: mỗi KH ~100-500ms → ước tính 1-6 giờ.")
	fmt.Println("Job chỉ có processedAt khi XONG hết. Không có progress trung gian.")
	fmt.Println("Kiểm tra chi tiết: go run scripts/check_crm_job_status.go <jobId>")
}
