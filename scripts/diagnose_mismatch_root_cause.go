// Script điều tra nguyên nhân mismatch: engaged (crm) vs visitor (activity).
// Kiểm tra: khách mismatch có conversation không? Link qua field nào?
//
// Chạy: go run scripts/diagnose_mismatch_root_cause.go [ownerOrganizationId]
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

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if n, ok := v.(float64); ok {
		return fmt.Sprintf("%.0f", n)
	}
	return fmt.Sprintf("%v", v)
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

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	actColl := db.Collection("crm_activity_history")
	convColl := db.Collection("fb_conversations")
	posCustColl := db.Collection("pc_pos_customers")

	orgIDStr := "69a655f0088600c32e62f955"
	if len(os.Args) >= 2 {
		orgIDStr = os.Args[1]
	}
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)
	endMs := endDate.UnixMilli()

	fmt.Println("=== Điều tra nguyên nhân mismatch (engaged crm vs visitor activity) ===\n")

	// 1. Lấy last snapshot per customer từ activity
	pipe := []bson.M{
		{"$match": bson.M{
			"ownerOrganizationId":     orgID,
			"activityAt":              bson.M{"$lte": endMs},
			"metadata.metricsSnapshot": bson.M{"$exists": true},
		}},
		{"$sort": bson.M{"activityAt": -1}},
		{"$group": bson.M{
			"_id":             "$unifiedId",
			"metricsSnapshot": bson.M{"$first": "$metadata.metricsSnapshot"},
		}},
	}
	cursor, _ := actColl.Aggregate(ctx, pipe)
	activitySnapshot := make(map[string]string)
	for cursor.Next(ctx) {
		var doc struct {
			ID              string                 `bson:"_id"`
			MetricsSnapshot map[string]interface{} `bson:"metricsSnapshot"`
		}
		if cursor.Decode(&doc) == nil && doc.MetricsSnapshot != nil {
			for _, layer := range []string{"layer1", "layer2", "raw"} {
				if sub, ok := doc.MetricsSnapshot[layer].(map[string]interface{}); ok {
					if js, ok := sub["journeyStage"].(string); ok && js != "" {
						activitySnapshot[doc.ID] = js
						break
					}
				}
			}
		}
	}
	cursor.Close(ctx)

	// 2. Lấy engaged trong crm, tìm mismatch
	var engaged []struct {
		UnifiedId     string `bson:"unifiedId"`
		PrimarySource string `bson:"primarySource"`
		SourceIds     struct {
			Pos string `bson:"pos"`
			Fb  string `bson:"fb"`
		} `bson:"sourceIds"`
	}
	cur, _ := crmColl.Find(ctx, bson.M{
		"ownerOrganizationId": orgID,
		"journeyStage":        "engaged",
	}, options.Find().SetProjection(bson.M{"unifiedId": 1, "primarySource": 1, "sourceIds": 1}).SetLimit(500))
	cur.All(ctx, &engaged)
	cur.Close(ctx)

	var mismatches []struct {
		UnifiedId     string
		PrimarySource string
		SourceIds     struct{ Pos, Fb string }
	}
	for _, c := range engaged {
		if activitySnapshot[c.UnifiedId] == "visitor" {
			mismatches = append(mismatches, struct {
				UnifiedId     string
				PrimarySource string
				SourceIds     struct{ Pos, Fb string }
			}{c.UnifiedId, c.PrimarySource, struct{ Pos, Fb string }{c.SourceIds.Pos, c.SourceIds.Fb}})
		}
	}

	fmt.Printf("Mismatch mẫu (lấy tối đa 10): %d\n\n", len(mismatches))
	if len(mismatches) > 10 {
		mismatches = mismatches[:10]
	}

	for i, m := range mismatches {
		fmt.Printf("--- Khách %d: %s (primarySource=%s) ---\n", i+1, m.UnifiedId, m.PrimarySource)
		ids := []string{m.UnifiedId, m.SourceIds.Pos, m.SourceIds.Fb}
		for _, id := range ids {
			if id == "" {
				continue
			}
			// Filter giống buildConversationFilterForCustomerIds
			convFilter := bson.M{
				"ownerOrganizationId": orgID,
				"$or": []bson.M{
					{"customerId": id},
					{"panCakeData.customer_id": id},
					{"panCakeData.customer.id": id},
					{"panCakeData.customers.id": id},
					{"panCakeData.page_customer.id": id},
					{"panCakeData.page_customer.customer_id": id},
				},
			}
			n, _ := convColl.CountDocuments(ctx, convFilter)
			if n > 0 {
				fmt.Printf("  ✓ Có %d conv match id=%s (customerId/page_customer/customers)\n", n, id)
				// Lấy 1 mẫu
				var sample struct {
					CustomerId   string                 `bson:"customerId"`
					PanCakeData  map[string]interface{} `bson:"panCakeData"`
					ConversationId string `bson:"conversationId"`
				}
				convColl.FindOne(ctx, convFilter).Decode(&sample)
				fmt.Printf("    Mẫu: conversationId=%s customerId=%s\n", sample.ConversationId, sample.CustomerId)
				if sample.PanCakeData != nil {
					pc := sample.PanCakeData["page_customer"]
					if pm, ok := pc.(map[string]interface{}); ok {
						fmt.Printf("    page_customer.id=%s customer_id=%s\n", getStr(pm, "id"), getStr(pm, "customer_id"))
					}
				}
				break
			}
		}

		// Nếu primarySource=pos: kiểm tra pc_pos_customers.posData.fb_id → conversationId
		if m.PrimarySource == "pos" && m.SourceIds.Pos != "" {
			var posCust struct {
				CustomerId string                 `bson:"customerId"`
				PosData    map[string]interface{} `bson:"posData"`
			}
			err := posCustColl.FindOne(ctx, bson.M{
				"customerId":            m.SourceIds.Pos,
				"ownerOrganizationId":   orgID,
			}).Decode(&posCust)
			if err == nil && posCust.PosData != nil {
				fbId := getStr(posCust.PosData, "fb_id")
				if fbId != "" {
					fmt.Printf("  pc_pos_customer.posData.fb_id = %s (conversationId format)\n", fbId)
					n, _ := convColl.CountDocuments(ctx, bson.M{
						"ownerOrganizationId": orgID,
						"conversationId":      fbId,
					})
					if n > 0 {
						fmt.Printf("  ✓ Có %d conv match conversationId=%s (link qua posData.fb_id)\n", n, fbId)
						fmt.Printf("  → NGUYÊN NHÂN: aggregate KHÔNG query theo conversationId từ posData.fb_id!\n")
					} else {
						fmt.Printf("  ✗ Không tìm thấy conv với conversationId=%s\n", fbId)
					}
				} else {
					fmt.Printf("  pc_pos_customer không có posData.fb_id\n")
				}
			} else {
				fmt.Printf("  Không tìm thấy pc_pos_customer cho sourceIds.pos=%s\n", m.SourceIds.Pos)
			}
		}
		fmt.Println()
	}

	fmt.Println("✓ Hoàn thành")
}
