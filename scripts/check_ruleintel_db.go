// Script kiểm tra module Rule Intelligence trong MongoDB.
//
// Chạy: go run scripts/check_ruleintel_db.go
//
// Kiểm tra các collection theo docs/02-architecture/core/rule-intelligence.md:
// - rule_definitions: Rule Definition (khi nào, ở đâu logic chạy)
// - rule_logic_definitions: Logic Script (script Goja)
// - rule_param_sets: Parameter Set (ngưỡng, config)
// - rule_output_definitions: Output Contract (schema output)
// - rule_execution_logs: Execution Trace (log mỗi lần chạy)
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

// Expected counts từ seed_rule_ads_system.go + seed_rule_crm_system.go (FolkForm v4.1)
const (
	expectedRuleDefinitions   = 53 // Ads (Kill, Decrease, Increase, Flag, Layer) + CRM
	expectedLogicScripts     = 36 // Ads logic scripts + CRM
	expectedParamSets        = 53
	expectedOutputContracts  = 6  // OUT_ACTION_CANDIDATE, OUT_FLAG_CANDIDATE, OUT_LAYER1/2/3, OUT_CRM_CLASSIFICATION
)

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

	fmt.Println("=== Rule Intelligence — Kiểm tra DB ===\n")
	fmt.Printf("Database: %s\n\n", dbName)

	// 1. rule_definitions
	collDef := db.Collection("rule_definitions")
	countDef, _ := collDef.CountDocuments(ctx, bson.M{})
	countActive, _ := collDef.CountDocuments(ctx, bson.M{"status": "active"})
	okDef := countDef >= expectedRuleDefinitions
	fmt.Printf("1. rule_definitions\n")
	fmt.Printf("   Tổng: %d (mong đợi >= %d) %s\n", countDef, expectedRuleDefinitions, statusIcon(okDef))
	fmt.Printf("   Active: %d\n", countActive)
	if countDef > 0 {
		cursor, _ := collDef.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "priority", Value: 1}}).SetLimit(20))
		var docs []bson.M
		cursor.All(ctx, &docs)
		cursor.Close(ctx)
		fmt.Printf("   Danh sách rule (priority asc):\n")
		for _, d := range docs {
			ruleID, _ := d["rule_id"].(string)
			ruleCode, _ := d["rule_code"].(string)
			domain, _ := d["domain"].(string)
			status, _ := d["status"].(string)
			priority := d["priority"]
			fmt.Printf("     - %s | %s | domain=%s | status=%s | priority=%v\n", ruleID, ruleCode, domain, status, priority)
		}
		if countDef > 20 {
			fmt.Printf("     ... và %d rule khác\n", countDef-20)
		}
	}
	fmt.Println()

	// 2. rule_logic_definitions
	collLogic := db.Collection("rule_logic_definitions")
	countLogic, _ := collLogic.CountDocuments(ctx, bson.M{})
	okLogic := countLogic >= expectedLogicScripts
	fmt.Printf("2. rule_logic_definitions\n")
	fmt.Printf("   Tổng: %d (mong đợi >= %d) %s\n", countLogic, expectedLogicScripts, statusIcon(okLogic))
	if countLogic > 0 {
		cursor, _ := collLogic.Find(ctx, bson.M{})
		var docs []bson.M
		cursor.All(ctx, &docs)
		cursor.Close(ctx)
		for _, d := range docs {
			logicID, _ := d["logic_id"].(string)
			version := d["logic_version"]
			runtime, _ := d["runtime"].(string)
			status, _ := d["status"].(string)
			scriptLen := 0
			if s, ok := d["script"].(string); ok {
				scriptLen = len(s)
			}
			fmt.Printf("     - %s v%v | runtime=%s | status=%s | script=%d chars\n", logicID, version, runtime, status, scriptLen)
		}
	}
	fmt.Println()

	// 3. rule_param_sets
	collParam := db.Collection("rule_param_sets")
	countParam, _ := collParam.CountDocuments(ctx, bson.M{})
	okParam := countParam >= expectedParamSets
	fmt.Printf("3. rule_param_sets\n")
	fmt.Printf("   Tổng: %d (mong đợi >= %d) %s\n", countParam, expectedParamSets, statusIcon(okParam))
	if countParam > 0 && countParam <= 25 {
		cursor, _ := collParam.Find(ctx, bson.M{})
		var docs []bson.M
		cursor.All(ctx, &docs)
		cursor.Close(ctx)
		for _, d := range docs {
			paramID, _ := d["param_set_id"].(string)
			version := d["param_version"]
			domain, _ := d["domain"].(string)
			fmt.Printf("     - %s v%v | domain=%s\n", paramID, version, domain)
		}
	}
	fmt.Println()

	// 4. rule_output_definitions
	collOut := db.Collection("rule_output_definitions")
	countOut, _ := collOut.CountDocuments(ctx, bson.M{})
	okOut := countOut >= expectedOutputContracts
	fmt.Printf("4. rule_output_definitions\n")
	fmt.Printf("   Tổng: %d (mong đợi >= %d) %s\n", countOut, expectedOutputContracts, statusIcon(okOut))
	if countOut > 0 {
		cursor, _ := collOut.Find(ctx, bson.M{})
		var docs []bson.M
		cursor.All(ctx, &docs)
		cursor.Close(ctx)
		for _, d := range docs {
			outputID, _ := d["output_id"].(string)
			outputType, _ := d["output_type"].(string)
			version := d["output_version"]
			fmt.Printf("     - %s v%v | type=%s\n", outputID, version, outputType)
		}
	}
	fmt.Println()

	// 5. rule_execution_logs (có thể rỗng nếu chưa chạy rule)
	collLog := db.Collection("rule_execution_logs")
	countLog, _ := collLog.CountDocuments(ctx, bson.M{})
	fmt.Printf("5. rule_execution_logs\n")
	fmt.Printf("   Tổng: %d (có thể 0 nếu chưa chạy rule)\n", countLog)
	if countLog > 0 {
		cursor, _ := collLog.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "executionTime", Value: -1}}).SetLimit(5))
		var docs []bson.M
		cursor.All(ctx, &docs)
		cursor.Close(ctx)
		fmt.Printf("   5 log gần nhất:\n")
		for _, d := range docs {
			ruleID, _ := d["rule_id"].(string)
			status, _ := d["execution_status"].(string)
			execTime := d["executionTime"]
			fmt.Printf("     - %s | status=%s | time=%v\n", ruleID, status, execTime)
		}
	}
	fmt.Println()

	// Tổng kết
	allOk := okDef && okLogic && okParam && okOut
	fmt.Println("--- Tổng kết ---")
	if allOk {
		fmt.Println("Module Rule Intelligence đã seed đầy đủ theo spec (rule_definitions, logic, param, output).")
		fmt.Println("Nếu rule_execution_logs rỗng: chưa có lần chạy rule nào (API Run hoặc Ads worker chưa gọi).")
	} else {
		fmt.Println("Thiếu dữ liệu! Chạy seed:")
		fmt.Println("  - Gọi migration.SeedRuleAdsSystem(ctx) khi init server")
		fmt.Println("  - Hoặc migration.SeedRuleAdsKillCandidate(ctx) cho SL-A đơn lẻ")
		fmt.Println("Xem: api/internal/api/ruleintel/migration/seed_rule_ads_system.go")
	}
}

func statusIcon(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}
