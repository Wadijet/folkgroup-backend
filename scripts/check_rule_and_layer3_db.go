// Script kiểm tra rule execution logs và logic Layer 3 trong DB.
//
// Chạy: go run scripts/check_rule_and_layer3_db.go
//
// Kiểm tra:
// 1. rule_execution_logs — thống kê theo rule_id, sample RULE_CRM_CLASSIFICATION (input/output/explanation)
// 2. crm_customers engaged — currentMetrics.raw có lastConversationAt, totalMessages, conversationFromAds
// 3. crm_activity_history — metricsSnapshot có layer3.engaged
//
// Lưu ý: Layer 3 Engaged (temperature, depth, source) được derive bởi layer3.DeriveFromMap (Go code),
// KHÔNG qua Rule Engine. Chỉ RULE_CRM_CLASSIFICATION (journeyStage, valueTier...) mới ghi rule_execution_logs.
package main

import (
	"context"
	"encoding/json"
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	fmt.Println("=== KIỂM TRA RULE EXECUTION LOGS VÀ LAYER 3 TRONG DB ===\n")
	fmt.Printf("Database: %s\n\n", dbName)

	// --- 1. rule_execution_logs ---
	collLog := db.Collection("rule_execution_logs")
	totalLog, _ := collLog.CountDocuments(ctx, bson.M{})
	fmt.Println("1. rule_execution_logs")
	fmt.Printf("   Tổng documents: %d\n", totalLog)

	if totalLog > 0 {
		// Thống kê theo rule_id
		pipeline := []bson.M{
			{"$group": bson.M{"_id": "$rule_id", "count": bson.M{"$sum": 1}, "lastTs": bson.M{"$max": "$timestamp"}}},
			{"$sort": bson.M{"count": -1}},
			{"$limit": 15},
		}
		cursor, _ := collLog.Aggregate(ctx, pipeline)
		var groups []bson.M
		_ = cursor.All(ctx, &groups)
		cursor.Close(ctx)
		fmt.Println("   Phân bố theo rule_id:")
		for _, g := range groups {
			ruleID, _ := g["_id"].(string)
			count := g["count"]
			lastTs := g["lastTs"]
			fmt.Printf("     - %s: %v lần | lastTs=%v\n", ruleID, count, lastTs)
		}

		// Sample RULE_CRM_CLASSIFICATION — input, output, explanation
		var crmTraces []bson.M
		cur, _ := collLog.Find(ctx, bson.M{"rule_id": "RULE_CRM_CLASSIFICATION"},
			options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}}).SetLimit(3))
		_ = cur.All(ctx, &crmTraces)
		cur.Close(ctx)

		if len(crmTraces) > 0 {
			fmt.Println("\n   Sample RULE_CRM_CLASSIFICATION (3 gần nhất):")
			for i, t := range crmTraces {
				traceID, _ := t["trace_id"].(string)
				status, _ := t["execution_status"].(string)
				ts := t["timestamp"]
				fmt.Printf("\n   [%d] trace_id=%s | status=%s | timestamp=%v\n", i+1, traceID, status, ts)

				// InputSnapshot = input.Layers = {"raw": {...}} — RULE_CRM_CLASSIFICATION chỉ dùng orderCount, hasConversation (không có lastConversationAt, totalMessages)
				if inp, ok := t["input_snapshot"].(bson.M); ok && inp != nil {
					if raw, ok := inp["raw"].(bson.M); ok && raw != nil {
						orderCount := raw["orderCount"]
						hasConv := raw["hasConversation"]
						lastOrderAt := raw["lastOrderAt"]
						totalSpent := raw["totalSpent"]
						fmt.Printf("       input_snapshot.raw: orderCount=%v, hasConversation=%v, lastOrderAt=%v, totalSpent=%v\n",
							orderCount, hasConv, lastOrderAt, totalSpent)
					}
				}
				if out, ok := t["output_object"].(bson.M); ok && out != nil {
					journey, _ := out["journeyStage"].(string)
					valueTier, _ := out["valueTier"].(string)
					fmt.Printf("       output: journeyStage=%s, valueTier=%s\n", journey, valueTier)
				}
				if exp, ok := t["explanation"].(bson.M); ok && exp != nil {
					if logStr, ok := exp["log"].(string); ok && logStr != "" {
						preview := logStr
						if len(preview) > 200 {
							preview = preview[:200] + "..."
						}
						fmt.Printf("       explanation.log: %s\n", preview)
					} else {
						fmt.Printf("       explanation: có nhưng không có field log\n")
					}
				} else {
					fmt.Printf("       explanation: nil\n")
				}
			}
		} else {
			fmt.Println("\n   Chưa có trace RULE_CRM_CLASSIFICATION (Recalculate/RefreshMetrics chưa chạy qua Rule Engine)")
		}
	} else {
		fmt.Println("   Collection rỗng — chưa có lần chạy rule nào.")
		fmt.Println("   Gợi ý: Chạy Recalculate, RefreshMetrics, hoặc Ads worker để tạo trace.")
	}

	// --- 2. crm_customers engaged — currentMetrics.raw ---
	fmt.Println("\n2. crm_customers (journeyStage=engaged) — currentMetrics.raw")
	crmColl := db.Collection("crm_customers")
	engagedTotal, _ := crmColl.CountDocuments(ctx, bson.M{"journeyStage": "engaged"})
	engagedWithRaw, _ := crmColl.CountDocuments(ctx, bson.M{"journeyStage": "engaged", "currentMetrics.raw": bson.M{"$exists": true}})
	engagedWithLastConv, _ := crmColl.CountDocuments(ctx, bson.M{"journeyStage": "engaged", "currentMetrics.raw.lastConversationAt": bson.M{"$gt": 0}})
	engagedWithTotalMsg, _ := crmColl.CountDocuments(ctx, bson.M{"journeyStage": "engaged", "currentMetrics.raw.totalMessages": bson.M{"$gt": 0}})

	fmt.Printf("   Tổng engaged: %d\n", engagedTotal)
	fmt.Printf("   Có currentMetrics.raw: %d\n", engagedWithRaw)
	fmt.Printf("   Có lastConversationAt > 0: %d\n", engagedWithLastConv)
	fmt.Printf("   Có totalMessages > 0: %d\n", engagedWithTotalMsg)

	if engagedTotal > 0 && engagedWithRaw > 0 {
		var samples []bson.M
		cur, _ := crmColl.Find(ctx, bson.M{"journeyStage": "engaged"},
			options.Find().SetLimit(3).SetProjection(bson.M{
				"unifiedId": 1,
				"currentMetrics.raw.lastConversationAt": 1,
				"currentMetrics.raw.totalMessages":      1,
				"currentMetrics.raw.conversationFromAds": 1,
				"currentMetrics.layer3":                  1,
			}))
		_ = cur.All(ctx, &samples)
		cur.Close(ctx)
		fmt.Println("   Sample 3 khách engaged:")
		for i, s := range samples {
			uid, _ := s["unifiedId"].(string)
			cm, _ := s["currentMetrics"].(bson.M)
			raw := map[string]interface{}{}
			layer3 := map[string]interface{}{}
			if cm != nil {
				if r, ok := cm["raw"].(bson.M); ok {
					raw = r
				}
				if l3, ok := cm["layer3"].(bson.M); ok {
					layer3 = l3
				}
			}
			lastConv := raw["lastConversationAt"]
			totalMsg := raw["totalMessages"]
			fromAds := raw["conversationFromAds"]
			engagedL3 := layer3["engaged"]
			fmt.Printf("     [%d] unifiedId=%s | lastConv=%v | totalMsg=%v | fromAds=%v | layer3.engaged=%v\n",
				i+1, uid, lastConv, totalMsg, fromAds, engagedL3 != nil)
			if engagedL3 != nil {
				b, _ := json.MarshalIndent(engagedL3, "         ", "  ")
				fmt.Printf("         engaged: %s\n", string(b))
			}
		}
	}

	// --- 3. crm_activity_history — metricsSnapshot.layer3 ---
	fmt.Println("\n3. crm_activity_history — metricsSnapshot.layer3.engaged")
	actColl := db.Collection("crm_activity_history")
	actTotal, _ := actColl.CountDocuments(ctx, bson.M{})
	actWithMetrics, _ := actColl.CountDocuments(ctx, bson.M{"metadata.metricsSnapshot": bson.M{"$exists": true}})
	actWithLayer3, _ := actColl.CountDocuments(ctx, bson.M{"metadata.metricsSnapshot.layer3": bson.M{"$exists": true}})
	actWithEngaged, _ := actColl.CountDocuments(ctx, bson.M{"metadata.metricsSnapshot.layer3.engaged": bson.M{"$exists": true}})

	fmt.Printf("   Tổng activity: %d\n", actTotal)
	fmt.Printf("   Có metricsSnapshot: %d\n", actWithMetrics)
	fmt.Printf("   Có metricsSnapshot.layer3: %d\n", actWithLayer3)
	fmt.Printf("   Có metricsSnapshot.layer3.engaged: %d\n", actWithEngaged)

	if actWithEngaged > 0 {
		var sampleAct []bson.M
		cur, _ := actColl.Find(ctx, bson.M{"metadata.metricsSnapshot.layer3.engaged": bson.M{"$exists": true}},
			options.Find().SetLimit(2).SetProjection(bson.M{
				"objectId": 1, "activityType": 1, "activityAt": 1,
				"metadata.metricsSnapshot.layer3.engaged": 1,
				"metadata.metricsSnapshot.raw.lastConversationAt": 1,
				"metadata.metricsSnapshot.raw.totalMessages":      1,
			}))
		_ = cur.All(ctx, &sampleAct)
		cur.Close(ctx)
		fmt.Println("   Sample activity có layer3.engaged:")
		for i, a := range sampleAct {
			objID := a["objectId"]
			actType := a["activityType"]
			meta, _ := a["metadata"].(bson.M)
			ms, _ := meta["metricsSnapshot"].(bson.M)
			raw := map[string]interface{}{}
			engaged := interface{}(nil)
			if ms != nil {
				if r, ok := ms["raw"].(bson.M); ok {
					raw = r
				}
				if l3, ok := ms["layer3"].(bson.M); ok {
					engaged = l3["engaged"]
				}
			}
			fmt.Printf("     [%d] objectId=%v | type=%v | lastConv=%v | totalMsg=%v | engaged=%v\n",
				i+1, objID, actType, raw["lastConversationAt"], raw["totalMessages"], engaged != nil)
		}
	}

	fmt.Println("\n--- Tổng kết ---")
	fmt.Println("• rule_execution_logs: Trace mỗi lần RULE_CRM_CLASSIFICATION, RULE_ADS_*, CIO chạy.")
	fmt.Println("• RULE_CRM_CLASSIFICATION output: journeyStage, valueTier, lifecycleStage (Layer 1+2).")
	fmt.Println("• Layer 3 Engaged (temperature, depth, source) derive từ layer3.DeriveFromMap (Go) — KHÔNG có rule trace.")
	fmt.Println("• Nếu engaged có lastConversationAt=0, totalMessages=0 → Layer 3 sẽ ra cold, light (do thiếu dữ liệu raw).")
}
