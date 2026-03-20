// Script kiểm tra logic scripts trong DB có report.log hay không.
//
// Chạy: go run scripts/check_ruleintel_report_log.go
//
// Executor yêu cầu script phải trả về report có field log.
// Script này quét rule_logic_definitions và báo logic nào thiếu report.log.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

func hasReportLog(script string) bool {
	if script == "" {
		return false
	}
	// Có report.log (gán từng bước) hoặc report:{...log:...} (inline)
	return strings.Contains(script, "report.log") ||
		strings.Contains(script, "report:{") && strings.Contains(script, "log:")
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
	coll := db.Collection("rule_logic_definitions")

	cursor, err := coll.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "logic_id", Value: 1}}))
	if err != nil {
		log.Fatalf("Find: %v", err)
	}
	defer cursor.Close(ctx)

	var docs []bson.M
	if err := cursor.All(ctx, &docs); err != nil {
		log.Fatalf("All: %v", err)
	}

	fmt.Println("=== Rule Intelligence — Kiểm tra report.log trong logic scripts ===\n")
	fmt.Printf("Database: %s | Collection: rule_logic_definitions\n", dbName)
	fmt.Printf("Tổng: %d logic scripts\n\n", len(docs))

	missing := []string{}
	for _, d := range docs {
		logicID, _ := d["logic_id"].(string)
		version := d["logic_version"]
		script, _ := d["script"].(string)
		ok := hasReportLog(script)
		icon := "✓"
		if !ok {
			icon = "✗"
			missing = append(missing, fmt.Sprintf("%s v%v", logicID, version))
		}
		scriptLen := len(script)
		fmt.Printf("  %s %s v%v | script=%d chars | report.log: %s\n", icon, logicID, version, scriptLen, map[bool]string{true: "có", false: "THIẾU"}[ok])
	}

	fmt.Println()
	if len(missing) > 0 {
		fmt.Println("--- Logic thiếu report.log (sẽ lỗi khi chạy) ---")
		for _, m := range missing {
			fmt.Printf("  - %s\n", m)
		}
		fmt.Println("\nCần chạy lại seed để cập nhật: migration.SeedRuleAdsSystem(ctx), SeedRuleCrmSystem(ctx)")
	} else {
		fmt.Println("Tất cả logic scripts đều có report.log.")
	}
}
