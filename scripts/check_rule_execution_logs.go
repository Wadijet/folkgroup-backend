// Script kiểm tra rule_execution_logs — có nội dung log (explanation.log) không.
//
// Chạy: go run scripts/check_rule_execution_logs.go
//
// Engine ghi trace.Explanation = report từ script (có field log).
// Script này kiểm tra các document trong rule_execution_logs có explanation.log không.
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
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if dbName == "" {
		dbName = os.Getenv("MONGODB_DBNAME")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH (hoặc MONGODB_DBNAME)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	coll := db.Collection("rule_execution_logs")

	count, _ := coll.CountDocuments(ctx, bson.M{})
	fmt.Println("=== Rule Execution Logs — Kiểm tra nội dung log (explanation.log) ===\n")
	fmt.Printf("Database: %s | Collection: rule_execution_logs\n", dbName)
	fmt.Printf("Tổng documents: %d\n\n", count)

	if count == 0 {
		fmt.Println("Collection rỗng — chưa có lần chạy rule nào.")
		fmt.Println("Gợi ý: Gọi API Run rule hoặc Ads worker để tạo execution log.")
		return
	}

	cursor, err := coll.Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).SetLimit(20))
	if err != nil {
		log.Fatalf("Find: %v", err)
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		log.Fatalf("All: %v", err)
	}

	hasLog := 0
	noLog := 0
	for _, d := range docs {
		traceID, _ := d["trace_id"].(string)
		ruleID, _ := d["rule_id"].(string)
		status, _ := d["execution_status"].(string)
		explanation, _ := d["explanation"].(bson.M)
		timestamp := d["timestamp"]

		var logContent string
		var hasExplanationLog bool
		if explanation != nil {
			if l, ok := explanation["log"].(string); ok {
				logContent = l
				hasExplanationLog = true
				hasLog++
			} else {
				hasExplanationLog = false
				noLog++
			}
		} else {
			hasExplanationLog = false
			noLog++
		}

		icon := "✓"
		if !hasExplanationLog {
			icon = "✗"
		}
		fmt.Printf("%s trace=%s | rule=%s | status=%s | timestamp=%v\n", icon, traceID, ruleID, status, timestamp)
		if hasExplanationLog {
			preview := logContent
			if len(preview) > 120 {
				preview = preview[:120] + "..."
			}
			fmt.Printf("    explanation.log: %q\n", preview)
		} else {
			if explanation == nil {
				fmt.Printf("    explanation: nil (thiếu)\n")
			} else {
				fmt.Printf("    explanation: có nhưng KHÔNG có field log — keys: %v\n", keys(explanation))
			}
		}
		fmt.Println()
	}

	fmt.Println("--- Tổng kết ---")
	fmt.Printf("  Có explanation.log: %d\n", hasLog)
	fmt.Printf("  Thiếu explanation.log: %d\n", noLog)
	if noLog > 0 {
		fmt.Println("\n⚠️ Một số trace thiếu nội dung log — có thể do:")
		fmt.Println("  - Rule chạy lỗi (execution_status=error) → explanation có thể nil")
		fmt.Println("  - Logic script cũ không trả về report.log")
	}
}

func keys(m map[string]interface{}) []string {
	var k []string
	for kk := range m {
		k = append(k, kk)
	}
	return k
}
