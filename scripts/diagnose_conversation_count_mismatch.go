// Script chẩn đoán: Số hội thoại (conversationCount) trong raw/currentMetrics không khớp với tab Hội thoại.
// Kiểm tra: crm_customers, fb_conversations, pc_pos_customers — tìm nguyên nhân aggregate trả 0.
//
// Chạy: go run scripts/diagnose_conversation_count_mismatch.go <ownerOrganizationId> [unifiedId hoặc "Tên khách"]
// VD: go run scripts/diagnose_conversation_count_mismatch.go 507f1f77bcf86cd799439011 "Thao Nguyen"
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// buildConversationFilterForCustomerIds — replicate logic từ service.crm.conversation_metrics
func buildConversationFilterForCustomerIds(customerIds []string, ownerOrgID primitive.ObjectID, conversationIds []string) bson.M {
	var ids []string
	var numIds []interface{}
	for _, id := range customerIds {
		if id != "" {
			ids = append(ids, id)
			if n, err := strconv.ParseInt(id, 10, 64); err == nil {
				numIds = append(numIds, n)
			}
		}
	}
	convCustomerOr := []bson.M{
		{"customerId": bson.M{"$in": ids}},
		{"links.customer.uid": bson.M{"$in": ids}},
		{"panCakeData.customer_id": bson.M{"$in": ids}},
		{"panCakeData.customer.id": bson.M{"$in": ids}},
		{"panCakeData.customers.id": bson.M{"$in": ids}},
		{"panCakeData.page_customer.id": bson.M{"$in": ids}},
		{"panCakeData.page_customer.customer_id": bson.M{"$in": ids}},
	}
	if len(numIds) > 0 {
		convCustomerOr = append(convCustomerOr,
			bson.M{"panCakeData.customer_id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customers.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.page_customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.page_customer.customer_id": bson.M{"$in": numIds}},
		)
	}
	for _, cid := range conversationIds {
		if cid != "" {
			convCustomerOr = append(convCustomerOr, bson.M{"conversationId": cid})
		}
	}
	if len(ids) == 0 && len(conversationIds) == 0 {
		return bson.M{"ownerOrganizationId": ownerOrgID, "customerId": "__NO_MATCH__"}
	}
	return bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$or":                 convCustomerOr,
	}
}

func getConversationIdsFromPosCustomers(ctx context.Context, posColl *mongo.Collection, posCustomerIds []string, ownerOrgID primitive.ObjectID) []string {
	if len(posCustomerIds) == 0 {
		return nil
	}
	cursor, err := posColl.Find(ctx, bson.M{
		"ownerOrganizationId": ownerOrgID,
		"customerId":          bson.M{"$in": posCustomerIds},
		"posData.fb_id":       bson.M{"$exists": true, "$ne": ""},
	}, nil)
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	var result []string
	seen := make(map[string]bool)
	for cursor.Next(ctx) {
		var doc struct {
			PosData map[string]interface{} `bson:"posData"`
		}
		if cursor.Decode(&doc) != nil || doc.PosData == nil {
			continue
		}
		fbId := ""
		if v, ok := doc.PosData["fb_id"].(string); ok && v != "" {
			fbId = v
		} else if n, ok := doc.PosData["fb_id"].(float64); ok {
			fbId = fmt.Sprintf("%.0f", n)
		}
		if fbId != "" && !seen[fbId] {
			seen[fbId] = true
			result = append(result, fbId)
		}
	}
	return result
}

func getConversationIdsFromFbMatch(ctx context.Context, convColl *mongo.Collection, customerIds []string, ownerOrgID primitive.ObjectID) []string {
	if len(customerIds) == 0 {
		return nil
	}
	filter := buildConversationFilterForCustomerIds(customerIds, ownerOrgID, nil)
	if filter["customerId"] == "__NO_MATCH__" {
		return nil
	}
	cursor, err := convColl.Find(ctx, filter, options.Find().SetProjection(bson.M{"conversationId": 1}).SetLimit(50))
	if err != nil {
		return nil
	}
	defer cursor.Close(ctx)
	var result []string
	seen := make(map[string]bool)
	for cursor.Next(ctx) {
		var doc struct {
			ConversationId string `bson:"conversationId"`
		}
		if cursor.Decode(&doc) != nil || doc.ConversationId == "" {
			continue
		}
		if !seen[doc.ConversationId] {
			seen[doc.ConversationId] = true
			result = append(result, doc.ConversationId)
		}
	}
	return result
}

// runAggregateConversationCount chạy pipeline aggregate ĐƠN GIẢN (không có convUpdatedAt/convExistedAt).
func runAggregateConversationCount(ctx context.Context, convColl *mongo.Collection, matchFilter bson.M) int {
	addFieldsStage := bson.M{
		"msgCount": bson.M{"$ifNull": bson.A{"$panCakeData.message_count", 0}},
		"convType": bson.M{"$ifNull": bson.A{"$panCakeData.type", "INBOX"}},
	}
	pipeStages := []bson.D{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$addFields", Value: addFieldsStage}},
		{{Key: "$group", Value: bson.M{
			"_id":               nil,
			"conversationCount": bson.M{"$sum": 1},
		}}},
	}
	cursor, err := convColl.Aggregate(ctx, pipeStages)
	if err != nil {
		return -1
	}
	defer cursor.Close(ctx)
	var result struct {
		ConversationCount int `bson:"conversationCount"`
	}
	if cursor.Next(ctx) {
		_ = cursor.Decode(&result)
	}
	return result.ConversationCount
}

// runFullServicePipelineWithDetails chạy pipeline ĐẦY ĐỦ giống service.crm.conversation_metrics.
// Trả về: (conversationCount, totalMessages). conversationCount=-2 nếu lỗi.
func runFullServicePipelineWithDetails(ctx context.Context, convColl *mongo.Collection, matchFilter bson.M, asOf int64) (int, int) {
	parseStringToLong := func(fieldPath string) bson.M {
		return bson.M{
			"$convert": bson.M{
				"input": bson.M{
					"$dateFromString": bson.M{
						"dateString":     bson.M{"$arrayElemAt": bson.A{bson.M{"$split": bson.A{fieldPath, "."}}, 0}},
						"onError":        nil,
						"onNull":         nil,
					},
				},
				"to":      "long",
				"onError": nil,
				"onNull":  nil,
			},
		}
	}
	convUpdatedAtParsed := bson.M{
		"$switch": bson.M{
			"branches": bson.A{
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, "string"}},
					"then": parseStringToLong("$panCakeData.updated_at")},
				bson.M{"case": bson.M{"$in": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, bson.A{"long", "int"}}},
					"then": "$panCakeData.updated_at"},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, "double"}},
					"then": bson.M{"$toLong": "$panCakeData.updated_at"}},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, "date"}},
					"then": bson.M{"$toLong": "$panCakeData.updated_at"}},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.updated_at"}, "timestamp"}},
					"then": bson.M{"$toLong": "$panCakeData.updated_at"}},
			},
			"default": nil,
		},
	}
	// $ifNull chỉ nhận 2 tham số — dùng lồng nhau cho fallback chain
	convUpdatedAt := bson.M{"$ifNull": bson.A{convUpdatedAtParsed, bson.M{"$ifNull": bson.A{"$panCakeUpdatedAt", "$updatedAt"}}}}
	convInsertedAtMs := bson.M{
		"$switch": bson.M{
			"branches": bson.A{
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "string"}},
					"then": parseStringToLong("$panCakeData.inserted_at")},
				bson.M{"case": bson.M{"$in": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, bson.A{"long", "int"}}},
					"then": "$panCakeData.inserted_at"},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "double"}},
					"then": bson.M{"$toLong": "$panCakeData.inserted_at"}},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "date"}},
					"then": bson.M{"$toLong": "$panCakeData.inserted_at"}},
				bson.M{"case": bson.M{"$eq": bson.A{bson.M{"$type": "$panCakeData.inserted_at"}, "timestamp"}},
					"then": bson.M{"$toLong": "$panCakeData.inserted_at"}},
			},
			"default": nil,
		},
	}
	addFieldsStage := bson.M{
		"convUpdatedAt":   convUpdatedAt,
		"convInsertedAtMs": convInsertedAtMs,
		"convExistedAt":   "$convInsertedAtMs",
		"msgCount":        bson.M{"$ifNull": bson.A{"$panCakeData.message_count", 0}},
		"convType":        bson.M{"$ifNull": bson.A{"$panCakeData.type", "INBOX"}},
		"hasAdIds":        bson.M{"$gt": bson.A{bson.M{"$size": bson.M{"$ifNull": bson.A{"$panCakeData.ad_ids", bson.A{}}}}, 0}},
	}
	pipeStages := []bson.D{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$addFields", Value: addFieldsStage}},
	}
	if asOf > 0 {
		pipeStages = append(pipeStages, bson.D{{Key: "$match", Value: bson.M{
			"$or": []bson.M{
				{"convExistedAt": bson.M{"$lte": asOf}},
				{"convExistedAt": nil},
			},
		}}})
	}
	pipeStages = append(pipeStages, bson.D{{Key: "$group", Value: bson.M{
		"_id":                     nil,
		"conversationCount":       bson.M{"$sum": 1},
		"conversationCountInbox":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$convType", "INBOX"}}, 1, 0}}},
		"conversationCountComment": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$convType", "COMMENT"}}, 1, 0}}},
		"lastConversationAt":      bson.M{"$max": "$convUpdatedAt"},
		"firstConversationAt":     bson.M{"$min": "$convUpdatedAt"},
		"totalMessages":          bson.M{"$sum": "$msgCount"},
		"anyFromAds":              bson.M{"$max": bson.M{"$cond": bson.A{"$hasAdIds", 1, 0}}},
	}}})
	cursor, err := convColl.Aggregate(ctx, pipeStages)
	if err != nil {
		log.Printf("  [DEBUG] Pipeline đầy đủ lỗi: %v", err)
		return -2, 0 // -2 = lỗi pipeline đầy đủ
	}
	defer cursor.Close(ctx)
	var result struct {
		ConversationCount int `bson:"conversationCount"`
		TotalMessages     int `bson:"totalMessages"`
	}
	if cursor.Next(ctx) {
		_ = cursor.Decode(&result)
	}
	return result.ConversationCount, result.TotalMessages
}

func getMessageCountFromConv(doc map[string]interface{}) int {
	pd, _ := doc["panCakeData"].(map[string]interface{})
	if pd == nil {
		return 0
	}
	if v, ok := pd["message_count"]; ok && v != nil {
		switch x := v.(type) {
		case int:
			return x
		case int32:
			return int(x)
		case int64:
			return int(x)
		case float64:
			return int(x)
		}
	}
	return 0
}

func extractIdsFromConv(doc map[string]interface{}) []string {
	var ids []string
	seen := make(map[string]bool)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			ids = append(ids, s)
		}
	}
	if cid, ok := doc["customerId"].(string); ok {
		add(cid)
	}
	pd, _ := doc["panCakeData"].(map[string]interface{})
	if pd == nil {
		return ids
	}
	if arr, ok := pd["customers"].([]interface{}); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				if v, ok := m["id"]; ok && v != nil {
					add(fmt.Sprintf("%v", v))
				}
			}
		}
	}
	if pc, ok := pd["page_customer"].(map[string]interface{}); ok {
		if v, ok := pc["id"]; ok && v != nil {
			add(fmt.Sprintf("%v", v))
		}
	}
	if cust, ok := pd["customer"].(map[string]interface{}); ok {
		if v, ok := cust["id"]; ok && v != nil {
			add(fmt.Sprintf("%v", v))
		}
	}
	if v, ok := pd["customer_id"]; ok && v != nil {
		add(fmt.Sprintf("%v", v))
	}
	return ids
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

	if len(os.Args) < 2 {
		log.Fatal("Cách dùng: go run scripts/diagnose_conversation_count_mismatch.go <ownerOrganizationId> [unifiedId hoặc tên khách]")
	}
	orgID, err := primitive.ObjectIDFromHex(os.Args[1])
	if err != nil {
		log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
	}

	searchArg := ""
	if len(os.Args) > 2 {
		searchArg = strings.TrimSpace(strings.Join(os.Args[2:], " "))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	convColl := db.Collection("fb_conversations")
	posColl := db.Collection("pc_pos_customers")
	actColl := db.Collection("crm_activity_history")

	// 1. Tìm customer
	var filter bson.M
	if searchArg != "" {
		// Tìm theo unifiedId hoặc tên
		if len(searchArg) == 24 && isHex(searchArg) {
			filter = bson.M{"ownerOrganizationId": orgID, "unifiedId": searchArg}
		} else {
			filter = bson.M{
				"ownerOrganizationId": orgID,
				"$or": []bson.M{
					{"profile.name": bson.M{"$regex": searchArg, "$options": "i"}},
					{"unifiedId": searchArg},
					{"uid": searchArg},
				},
			}
		}
	} else {
		// Mặc định: lấy 10 customer đầu tiên (có thể thêm filter nếu cần)
		filter = bson.M{"ownerOrganizationId": orgID}
	}

	opts := options.Find().SetLimit(10).SetSort(bson.D{{Key: "updatedAt", Value: -1}})
	cursor, err := crmColl.Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("Query crm_customers lỗi: %v", err)
	}
	defer cursor.Close(ctx)

	var customers []map[string]interface{}
	for cursor.Next(ctx) {
		var c map[string]interface{}
		if cursor.Decode(&c) != nil {
			continue
		}
		customers = append(customers, c)
	}

	if len(customers) == 0 {
		// Gợi ý: liệt kê org IDs có trong crm_customers
		distinctOrgs, _ := crmColl.Distinct(ctx, "ownerOrganizationId", bson.M{})
		fmt.Println("Không tìm thấy customer nào với filter hiện tại.")
		fmt.Println("Gợi ý: Kiểm tra ownerOrganizationId. Các org có customer:")
		for _, v := range distinctOrgs {
			if oid, ok := v.(primitive.ObjectID); ok {
				fmt.Printf("  - %s\n", oid.Hex())
			}
		}
		log.Fatal("Chạy lại với: go run scripts/diagnose_conversation_count_mismatch.go <ownerOrganizationId> [tên khách hoặc unifiedId]")
	}

	for _, c := range customers {
		unifiedId, _ := c["unifiedId"].(string)
		uid, _ := c["uid"].(string)
		sourceIds, _ := c["sourceIds"].(map[string]interface{})
		currentMetrics, _ := c["currentMetrics"].(map[string]interface{})
		profile, _ := c["profile"].(map[string]interface{})
		name, _ := profile["name"].(string)

		// Lấy conversationCount, totalMessages từ currentMetrics.raw
		convCount := 0
		totalMsgsRaw := 0
		if cm := currentMetrics; cm != nil {
			if raw, ok := cm["raw"].(map[string]interface{}); ok {
				if v, ok := raw["conversationCount"]; ok && v != nil {
					switch x := v.(type) {
					case int:
						convCount = x
					case int32:
						convCount = int(x)
					case int64:
						convCount = int(x)
					case float64:
						convCount = int(x)
					}
				}
				if v, ok := raw["totalMessages"]; ok && v != nil {
					switch x := v.(type) {
					case int:
						totalMsgsRaw = x
					case int32:
						totalMsgsRaw = int(x)
					case int64:
						totalMsgsRaw = int(x)
					case float64:
						totalMsgsRaw = int(x)
					}
				}
			}
		}

		// Build customerIds (như buildCustomerIdsForRecalculate)
		ids := buildCustomerIdsFromDoc(c, sourceIds)
		posId := getStr(sourceIds, "pos")
		fbId := getStr(sourceIds, "fb")

		// Lấy conversationIds
		convIdsFromPos := []string{}
		if posId != "" {
			convIdsFromPos = getConversationIdsFromPosCustomers(ctx, posColl, []string{posId}, orgID)
		}
		convIdsFromFb := getConversationIdsFromFbMatch(ctx, convColl, ids, orgID)

		conversationIds := convIdsFromPos
		for _, cid := range convIdsFromFb {
			if cid != "" {
				has := false
				for _, x := range conversationIds {
					if x == cid {
						has = true
						break
					}
				}
				if !has {
					conversationIds = append(conversationIds, cid)
				}
			}
		}

		// Đếm fb_conversations match (Find)
		matchFilter := buildConversationFilterForCustomerIds(ids, orgID, conversationIds)
		convCountActual, _ := convColl.CountDocuments(ctx, matchFilter)

		// Chạy aggregate pipeline đơn giản và pipeline đầy đủ (như service)
		convCountAgg := runAggregateConversationCount(ctx, convColl, matchFilter)
		convCountFull, totalMsgsAgg := runFullServicePipelineWithDetails(ctx, convColl, matchFilter, 0)

		// Đếm activity conversation_started
		actFilter := bson.M{
			"ownerOrganizationId": orgID,
			"unifiedId":           unifiedId,
			"activityType":       "conversation_started",
		}
		actCount, _ := actColl.CountDocuments(ctx, actFilter)

		fmt.Println("\n" + strings.Repeat("=", 70))
		fmt.Printf("CUSTOMER: %s (unifiedId=%s, uid=%s)\n", name, unifiedId, uid)
		fmt.Println(strings.Repeat("=", 70))
		fmt.Printf("  sourceIds: pos=%s, fb=%s\n", posId, fbId)
		fmt.Printf("  currentMetrics.raw.conversationCount: %d\n", convCount)
		fmt.Printf("  currentMetrics.raw.totalMessages: %d\n", totalMsgsRaw)
		fmt.Printf("  customerIds (để match conv): %v\n", ids)
		fmt.Printf("  conversationIds từ POS (posData.fb_id): %v\n", convIdsFromPos)
		fmt.Printf("  conversationIds từ FB match: %v\n", convIdsFromFb)
		fmt.Printf("  Tổng conversationIds dùng trong filter: %v\n", conversationIds)
		fmt.Printf("  ---\n")
		fmt.Printf("  fb_conversations match filter (Find): %d (số thực tế trong DB)\n", convCountActual)
		fmt.Printf("  aggregate pipeline đơn giản: %d\n", convCountAgg)
		fmt.Printf("  aggregate pipeline ĐẦY ĐỦ (như service): count=%d, totalMessages=%d\n", convCountFull, totalMsgsAgg)
		if convCountFull == -2 {
			fmt.Printf("  ⚠️ Pipeline đầy đủ LỖI — kiểm tra $addFields (parseStringToLong, convUpdatedAt, convExistedAt)\n")
		}
		if totalMsgsRaw != totalMsgsAgg && convCountFull >= 0 {
			fmt.Printf("  ⚠️ totalMessages LỆCH: raw=%d, aggregate=%d — Recalculate chưa ghi hoặc raw cũ\n", totalMsgsRaw, totalMsgsAgg)
		}
		fmt.Printf("  crm_activity_history (conversation_started): %d\n", actCount)
		fmt.Printf("  ---\n")

		if convCount != int(convCountActual) {
			fmt.Printf("  ⚠️  LỆCH: raw.conversationCount=%d nhưng fb_conversations có %d hội thoại\n", convCount, convCountActual)
			if convCountFull == 0 && int(convCountActual) > 0 {
				fmt.Printf("  → NGUYÊN NHÂN: Pipeline đầy đủ trong service trả 0 (Find trả %d) — lỗi trong $addFields (parseStringToLong, convExistedAt)\n", convCountActual)
			} else if convCountFull > 0 && convCount == 0 {
				fmt.Printf("  → Nguyên nhân: Recalculate chưa ghi conversationCount vào raw (aggregate OK=%d)\n", convCountFull)
			} else if convCountAgg > 0 && convCountFull == 0 {
				fmt.Printf("  → Nguyên nhân: Pipeline đơn giản OK=%d nhưng pipeline đầy đủ trả 0 — lỗi trong addFields (convUpdatedAt/convInsertedAtMs/convExistedAt)\n", convCountAgg)
			}
		} else {
			fmt.Printf("  ✓ Khớp: conversationCount=%d\n", convCount)
		}

		// Chi tiết các conv match (nếu có) — kèm message_count để kiểm tra totalMessages
		if convCountActual > 0 && convCountActual <= 10 {
			fmt.Printf("\n  Chi tiết %d hội thoại (panCakeData.message_count):\n", convCountActual)
			cur, _ := convColl.Find(ctx, matchFilter, options.Find().SetLimit(10).SetProjection(bson.M{
				"conversationId": 1, "customerId": 1, "panCakeData": 1, "links": 1,
			}))
			idx := 0
			totalMsgSum := 0
			for cur.Next(ctx) {
				var conv map[string]interface{}
				if cur.Decode(&conv) != nil {
					continue
				}
				convId, _ := conv["conversationId"].(string)
				custId, _ := conv["customerId"].(string)
				extIds := extractIdsFromConv(conv)
				msgCount := getMessageCountFromConv(conv)
				totalMsgSum += msgCount
				fmt.Printf("    [%d] conversationId=%s, customerId=%s, message_count=%d, panCakeData IDs=%v\n", idx+1, convId, custId, msgCount, extIds)
				idx++
			}
			cur.Close(ctx)
			fmt.Printf("  → Tổng message_count từ fb_conversations: %d\n", totalMsgSum)
		}
	}
}

func buildCustomerIdsFromDoc(c map[string]interface{}, sourceIds map[string]interface{}) []string {
	seen := make(map[string]bool)
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	add(getStr(c, "uid"))
	add(getStr(c, "unifiedId"))
	add(getStr(sourceIds, "pos"))
	add(getStr(sourceIds, "fb"))
	add(getStr(sourceIds, "zalo"))
	if arr, ok := sourceIds["allInboxIds"].(bson.A); ok {
		for _, v := range arr {
			add(fmt.Sprintf("%v", v))
		}
	}
	if m, ok := sourceIds["fbByPage"].(map[string]interface{}); ok {
		for _, v := range m {
			add(fmt.Sprintf("%v", v))
		}
	}
	if m, ok := sourceIds["zaloByPage"].(map[string]interface{}); ok {
		for _, v := range m {
			add(fmt.Sprintf("%v", v))
		}
	}
	return ids
}

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

func isHex(s string) bool {
	if len(s) != 24 {
		return false
	}
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}
