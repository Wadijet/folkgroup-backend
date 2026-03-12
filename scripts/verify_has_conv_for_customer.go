// Script kiểm tra hasConv có đúng không — mô phỏng chính xác logic Recalculate.
// So sánh: filter buildConversationFilterForCustomerIds (có numIds) vs DB thực tế.
//
// Chạy: go run scripts/verify_has_conv_for_customer.go <ownerOrganizationId> <unifiedId>
//
// Ví dụ: go run scripts/verify_has_conv_for_customer.go 69a655f0088600c32e62f955 77e5b6f3-58ad-4e4c-aab1-8c62c2608916
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

	if len(os.Args) < 3 {
		log.Fatal("Chạy: go run scripts/verify_has_conv_for_customer.go <ownerOrganizationId> <unifiedId>")
	}
	orgIDStr := os.Args[1]
	unifiedId := os.Args[2]
	orgID, err := primitive.ObjectIDFromHex(orgIDStr)
	if err != nil {
		log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	convColl := db.Collection("fb_conversations")
	msgColl := db.Collection("fb_messages")
	posCustColl := db.Collection("pc_pos_customers")

	fmt.Printf("=== Kiểm tra hasConv cho %s ===\n\n", unifiedId)

	// 1. Lấy customer
	var cust struct {
		UnifiedId     string `bson:"unifiedId"`
		PrimarySource string `bson:"primarySource"`
		SourceIds     struct {
			Pos string `bson:"pos"`
			Fb  string `bson:"fb"`
		} `bson:"sourceIds"`
		JourneyStage string `bson:"journeyStage"`
	}
	err = crmColl.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": orgID}).Decode(&cust)
	if err != nil {
		log.Fatalf("Không tìm thấy customer: %v", err)
	}
	fmt.Printf("Customer: primarySource=%s, journeyStage=%s\n", cust.PrimarySource, cust.JourneyStage)
	fmt.Printf("  sourceIds.pos=%s, sourceIds.fb=%s\n\n", cust.SourceIds.Pos, cust.SourceIds.Fb)

	ids := []string{unifiedId, cust.SourceIds.Pos, cust.SourceIds.Fb}
	// Loại rỗng, thêm numIds (giống buildConversationFilterForCustomerIds)
	var cleanIds []string
	var numIds []interface{}
	for _, id := range ids {
		if id != "" {
			cleanIds = append(cleanIds, id)
			if n, err := strconv.ParseInt(id, 10, 64); err == nil {
				numIds = append(numIds, n)
			}
		}
	}
	if len(cleanIds) == 0 {
		cleanIds = ids
	}

	// 2. Kiểm tra DB: có conv match không? (giống buildConversationFilterForCustomerIds)
	fmt.Println("--- 1. DB thực tế (filter giống buildConversationFilterForCustomerIds) ---")
	convOr := []bson.M{
		{"customerId": bson.M{"$in": cleanIds}},
		{"panCakeData.customer_id": bson.M{"$in": cleanIds}},
		{"panCakeData.customer.id": bson.M{"$in": cleanIds}},
		{"panCakeData.customers.id": bson.M{"$in": cleanIds}},
		{"panCakeData.page_customer.id": bson.M{"$in": cleanIds}},
		{"panCakeData.page_customer.customer_id": bson.M{"$in": cleanIds}},
	}
	if len(numIds) > 0 {
		convOr = append(convOr,
			bson.M{"panCakeData.customer_id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customers.id": bson.M{"$in": numIds}},
		)
	}
	convFilter := bson.M{"ownerOrganizationId": orgID, "$or": convOr}
	nConv, _ := convColl.CountDocuments(ctx, convFilter)
	fmt.Printf("  Số conv match (customerIds + numIds): %d\n", nConv)

	// 3. conversationIds từ posData.fb_id + getConversationIdsFromFbMatch
	fmt.Println("\n--- 2. conversationIds (posData.fb_id + fb_conversations match) ---")
	posIds := []string{cust.SourceIds.Pos}
	if cust.SourceIds.Pos == "" {
		posIds = []string{}
	}
	var convIdsFromPos []string
	if len(posIds) > 0 {
		cur, _ := posCustColl.Find(ctx, bson.M{
			"ownerOrganizationId": orgID,
			"customerId":         bson.M{"$in": posIds},
			"posData.fb_id":      bson.M{"$exists": true, "$ne": ""},
		}, options.Find().SetProjection(bson.M{"posData.fb_id": 1}))
		for cur.Next(ctx) {
			var doc struct {
				PosData map[string]interface{} `bson:"posData"`
			}
			if cur.Decode(&doc) == nil && doc.PosData != nil {
				if v, ok := doc.PosData["fb_id"].(string); ok && v != "" {
					convIdsFromPos = append(convIdsFromPos, v)
				} else if n, ok := doc.PosData["fb_id"].(float64); ok {
					convIdsFromPos = append(convIdsFromPos, fmt.Sprintf("%.0f", n))
				}
			}
		}
		cur.Close(ctx)
	}
	fmt.Printf("  conversationIds từ pos: %v\n", convIdsFromPos)
	// getConversationIdsFromFbMatch: query convs theo customerIds, lấy conversationId
	var convIdsFromFb []string
	curFb, _ := convColl.Find(ctx, convFilter, options.Find().SetProjection(bson.M{"conversationId": 1}).SetLimit(50))
	for curFb.Next(ctx) {
		var doc struct {
			ConversationId string `bson:"conversationId"`
		}
		if curFb.Decode(&doc) == nil && doc.ConversationId != "" {
			convIdsFromFb = append(convIdsFromFb, doc.ConversationId)
		}
	}
	curFb.Close(ctx)
	fmt.Printf("  conversationIds từ fb_conversations match: %v\n", convIdsFromFb)

	// 4. Filter đầy đủ (customerIds + numIds + conversationIds) — giống aggregate
	fmt.Println("\n--- 3. Filter đầy đủ (giống aggregate + checkHasConversation) ---")
	convOrFull := make([]bson.M, len(convOr), len(convOr)+len(convIdsFromPos)+len(convIdsFromFb)+10)
	copy(convOrFull, convOr)
	for _, cid := range convIdsFromPos {
		if cid != "" {
			convOrFull = append(convOrFull, bson.M{"conversationId": cid})
		}
	}
	for _, cid := range convIdsFromFb {
		if cid != "" {
			convOrFull = append(convOrFull, bson.M{"conversationId": cid})
		}
	}
	convFilterFull := bson.M{"ownerOrganizationId": orgID, "$or": convOrFull}
	nConvFull, _ := convColl.CountDocuments(ctx, convFilterFull)
	fmt.Printf("  Số conv match (filter đầy đủ): %d\n", nConvFull)

	// 5. fb_messages (checkHasConversation cũng check fb_messages)
	fmt.Println("\n--- 4. fb_messages (checkHasConversation) ---")
	msgOr := []bson.M{{"customerId": bson.M{"$in": cleanIds}}}
	if len(numIds) > 0 {
		msgOr = append(msgOr, bson.M{"customerId": bson.M{"$in": numIds}})
	}
	nMsg, _ := msgColl.CountDocuments(ctx, bson.M{"ownerOrganizationId": orgID, "$or": msgOr})
	fmt.Printf("  Số message match customerIds: %d\n", nMsg)

	// 6. Kết luận
	fmt.Println("\n--- 5. Kết luận hasConv ---")
	hasConvExpected := nConvFull > 0 || nMsg > 0
	if hasConvExpected {
		fmt.Println("  ✓ hasConv=true là ĐÚNG — DB có conv hoặc message match")
		if nConvFull > 0 {
			fmt.Printf("    (fb_conversations: %d)\n", nConvFull)
		}
		if nMsg > 0 {
			fmt.Printf("    (fb_messages: %d)\n", nMsg)
		}
	} else {
		fmt.Println("  ✓ hasConv=false là ĐÚNG — DB không có conv/message match")
		fmt.Println("    → Khách thực sự không có hội thoại")
	}
	if nConv > 0 && nConvFull == 0 {
		fmt.Println("\n  ⚠ LƯU Ý: Có conv match customerIds nhưng filter đầy đủ = 0")
		fmt.Println("    → Có thể thiếu conversationIds (posData.fb_id, getConversationIdsFromFbMatch)")
	}

	// 7. Last activity snapshot (kiểm tra logRecalculateActivity có ghi đúng không)
	fmt.Println("\n--- 6. Last activity (metricsSnapshot) ---")
	actColl := db.Collection("crm_activity_history")
	var lastAct struct {
		ActivityAt  int64                  `bson:"activityAt"`
		ActivityType string                `bson:"activityType"`
		Source      string                 `bson:"source"`
		SourceRef   map[string]interface{} `bson:"sourceRef"`
		Metadata    map[string]interface{} `bson:"metadata"`
	}
	actColl.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": orgID, "metadata.metricsSnapshot": bson.M{"$exists": true}},
		options.FindOne().SetSort(bson.D{{Key: "activityAt", Value: -1}})).Decode(&lastAct)
	if lastAct.Metadata != nil {
		snap, _ := lastAct.Metadata["metricsSnapshot"].(map[string]interface{})
		stage := ""
		for _, layer := range []string{"layer1", "layer2", "raw"} {
			if sub, ok := snap[layer].(map[string]interface{}); ok {
				if v, ok := sub["journeyStage"].(string); ok && v != "" {
					stage = v
					break
				}
			}
		}
		fmt.Printf("  activityAt=%d activityType=%s source=%s journeyStage=%s\n", lastAct.ActivityAt, lastAct.ActivityType, lastAct.Source, stage)
		if lastAct.SourceRef != nil {
			fmt.Printf("  sourceRef=%v\n", lastAct.SourceRef)
		}
	} else {
		fmt.Println("  Không có activity với metricsSnapshot")
	}

	// 8. Chi tiết 1 conv mẫu (nếu có)
	if nConv > 0 {
		var sample struct {
			ConversationId string                 `bson:"conversationId"`
			CustomerId     string                 `bson:"customerId"`
			PanCakeData    map[string]interface{} `bson:"panCakeData"`
		}
		convColl.FindOne(ctx, convFilter).Decode(&sample)
		fmt.Printf("\n  Mẫu conv: conversationId=%s customerId=%s\n", sample.ConversationId, sample.CustomerId)
		if sample.PanCakeData != nil {
			pc := sample.PanCakeData["page_customer"]
			if pm, ok := pc.(map[string]interface{}); ok {
				fmt.Printf("  page_customer.id=%s customer_id=%s\n", getStr(pm, "id"), getStr(pm, "customer_id"))
			}
		}
	}

	fmt.Println("\n✓ Hoàn thành")
}
