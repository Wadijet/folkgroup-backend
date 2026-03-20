// Script kiểm tra logic link fb_conversations ↔ crm_customers trong DB.
//
// Chạy: go run scripts/check_conversation_customer_link.go
//
// Kiểm tra:
// 1. Format ID: fb_conversations (customerId, panCakeData.customer_id...) vs crm_customers (unifiedId, sourceIds)
// 2. Số engaged customers có match conversation (theo filter buildConversationFilterForCustomerIds)
// 3. Sample mismatch — engaged có lastConversationAt=0 nhưng có conv match không
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

// buildFilter giống buildConversationFilterForCustomerIds trong service.crm.conversation_metrics.go
func buildFilter(customerIds []string, ownerOrgID primitive.ObjectID, conversationIds []string) bson.M {
	var ids []string
	var numIds []interface{}
	for _, id := range customerIds {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
			if n, err := strconv.ParseInt(id, 10, 64); err == nil {
				numIds = append(numIds, n)
			}
		}
	}
	convCustomerOr := []bson.M{
		{"customerId": bson.M{"$in": ids}},
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

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	convColl := db.Collection("fb_conversations")
	crmColl := db.Collection("crm_customers")
	posColl := db.Collection("pc_pos_customers")

	fmt.Println("=== KIỂM TRA LOGIC LINK fb_conversations ↔ crm_customers ===\n")
	fmt.Printf("Database: %s\n\n", dbName)

	// --- 1. Format ID trong fb_conversations ---
	fmt.Println("1. Format ID trong fb_conversations (sample 5)")
	var convSamples []bson.M
	cur, _ := convColl.Find(ctx, bson.M{}, options.Find().SetLimit(5).SetProjection(bson.M{
		"conversationId": 1, "customerId": 1, "ownerOrganizationId": 1,
		"panCakeData.customer_id": 1, "panCakeData.customer.id": 1,
		"panCakeData.customers": 1, "panCakeData.page_customer.id": 1, "panCakeData.page_customer.customer_id": 1,
	}))
	_ = cur.All(ctx, &convSamples)
	cur.Close(ctx)

	for i, c := range convSamples {
		custId := c["customerId"]
		pd, _ := c["panCakeData"].(bson.M)
		panCustId, panCustObjId, panCustsId, pageCustId, pageCustCid := "", "", "", "", ""
		if pd != nil {
			panCustId, _ = pd["customer_id"].(string)
			if cust, ok := pd["customer"].(bson.M); ok {
				panCustObjId, _ = cust["id"].(string)
			}
			if arr, ok := pd["customers"].(bson.A); ok && len(arr) > 0 {
				if m, ok := arr[0].(bson.M); ok {
					panCustsId, _ = m["id"].(string)
				}
			}
			if pc, ok := pd["page_customer"].(bson.M); ok {
				pageCustId, _ = pc["id"].(string)
				pageCustCid, _ = pc["customer_id"].(string)
			}
		}
		fmt.Printf("  [%d] conversationId=%v | customerId=%v | panCakeData.customer_id=%v | customer.id=%v | customers[0].id=%v | page_customer.id=%v | page_customer.customer_id=%v\n",
			i+1, c["conversationId"], custId, panCustId, panCustObjId, panCustsId, pageCustId, pageCustCid)
	}

	// --- 2. Format ID trong crm_customers engaged ---
	fmt.Println("\n2. Format ID trong crm_customers engaged (sample 5)")
	var engagedSamples []bson.M
	cur, _ = crmColl.Find(ctx, bson.M{"journeyStage": "engaged"}, options.Find().SetLimit(5).SetProjection(bson.M{
		"unifiedId": 1, "sourceIds": 1, "ownerOrganizationId": 1,
		"currentMetrics.raw.lastConversationAt": 1, "currentMetrics.raw.totalMessages": 1,
	}))
	_ = cur.All(ctx, &engagedSamples)
	cur.Close(ctx)

	for i, c := range engagedSamples {
		uid, _ := c["unifiedId"].(string)
		pos, fb := "", ""
		if si, ok := c["sourceIds"].(bson.M); ok {
			pos, _ = si["pos"].(string)
			fb, _ = si["fb"].(string)
		}
		lastConv := getNestedInt64(c, "currentMetrics", "raw", "lastConversationAt")
		totalMsg := getNestedInt(c, "currentMetrics", "raw", "totalMessages")
		fmt.Printf("  [%d] unifiedId=%q | sourceIds.pos=%q | sourceIds.fb=%q | lastConv=%d | totalMsg=%d\n",
			i+1, uid, pos, fb, lastConv, totalMsg)
	}

	// --- 3a. Cross-check (chạy trước mục 3 vì mục 3 tốn thời gian): conv customerId có trong crm không ---
	fmt.Println("\n3a. Cross-check: fb_conversations.customerId có trong crm_customers (unifiedId/sourceIds) không?")
	var convCustomerIds []string
	var convSamples4 []bson.M
	cur4, err4 := convColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"customerId": 1, "panCakeData.customer_id": 1}).SetLimit(10000))
	if err4 == nil && cur4 != nil {
		_ = cur4.All(ctx, &convSamples4)
		cur4.Close(ctx)
		seen := make(map[string]bool)
		for _, d := range convSamples4 {
			for _, raw := range []interface{}{d["customerId"], getNested(d, "panCakeData", "customer_id")} {
				if s, ok := raw.(string); ok && s != "" && !seen[s] {
					seen[s] = true
					convCustomerIds = append(convCustomerIds, s)
				}
			}
		}
	}
	crmIdSet := make(map[string]bool)
	cur5, err5 := crmColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"unifiedId": 1, "sourceIds": 1}))
	if err5 == nil && cur5 != nil {
		for cur5.Next(ctx) {
			var d bson.M
			if cur5.Decode(&d) == nil {
				addIdsToSet(d, crmIdSet)
			}
		}
		cur5.Close(ctx)
	}
	crossMatched, crossUnmatched := 0, 0
	for _, id := range convCustomerIds {
		if crmIdSet[id] {
			crossMatched++
		} else {
			crossUnmatched++
		}
	}
	fmt.Printf("   Số customerId unique (từ 10k conv docs): %d\n", len(convCustomerIds))
	fmt.Printf("   Trong đó: match crm_customers=%d, không match=%d\n", crossMatched, crossUnmatched)

	// --- 3. Đếm match: engaged customer → có conv theo filter không ---
	fmt.Println("\n3. Kiểm tra link: engaged customer có match fb_conversations không (theo filter buildConversationFilterForCustomerIds)")

	engagedTotal, _ := crmColl.CountDocuments(ctx, bson.M{"journeyStage": "engaged"})
	convTotal, _ := convColl.CountDocuments(ctx, bson.M{})

	fmt.Printf("   Tổng engaged: %d | Tổng fb_conversations: %d\n", engagedTotal, convTotal)

	// Lấy danh sách org có engaged
	orgsRaw, _ := crmColl.Distinct(ctx, "ownerOrganizationId", bson.M{"journeyStage": "engaged"})
	var orgs []primitive.ObjectID
	for _, v := range orgsRaw {
		if oid, ok := v.(primitive.ObjectID); ok && !oid.IsZero() {
			orgs = append(orgs, oid)
		}
	}

	engagedWithMatch := int64(0)
	engagedWithoutMatch := int64(0)
	engagedNoConvButHasConvDB := int64(0)   // KHÔNG match conv nhưng hasConversation=true → có thể match qua fb_messages/expandIds
	engagedNoConvAndNoHasConvDB := int64(0) // KHÔNG match conv VÀ hasConversation=false → BUG: không nên là engaged
	sampleMismatch := []string{}
	sampleChecked := int64(0)

	for _, orgID := range orgs {
		if orgID.IsZero() {
			continue
		}
		cur, _ = crmColl.Find(ctx, bson.M{"journeyStage": "engaged", "ownerOrganizationId": orgID},
			options.Find().SetProjection(bson.M{"unifiedId": 1, "sourceIds": 1, "hasConversation": 1, "currentMetrics.raw.hasConversation": 1}).SetLimit(2000))
		var customers []bson.M
		_ = cur.All(ctx, &customers)
		cur.Close(ctx)

		for _, cust := range customers {
			ids := buildIdsFromCustomer(cust)
			convIds := []string{} // Engaged không có POS → convIds từ posData.fb_id = []
			sampleChecked++
			hasConvInDB := getNestedBool(cust, "hasConversation") || getNestedBool(cust, "currentMetrics", "raw", "hasConversation")
			if len(ids) == 0 && len(convIds) == 0 {
				engagedWithoutMatch++
				if len(sampleMismatch) < 3 {
					uid, _ := cust["unifiedId"].(string)
					sampleMismatch = append(sampleMismatch, fmt.Sprintf("unifiedId=%s ids=%v (rỗng) hasConvDB=%v", uid, ids, hasConvInDB))
				}
				continue
			}
			filter := buildFilter(ids, orgID, convIds)
			n, _ := convColl.CountDocuments(ctx, filter)
			if n > 0 {
				engagedWithMatch++
			} else {
				engagedWithoutMatch++
				if hasConvInDB {
					engagedNoConvButHasConvDB++
				} else {
					engagedNoConvAndNoHasConvDB++
				}
				if len(sampleMismatch) < 5 {
					uid, _ := cust["unifiedId"].(string)
					sampleMismatch = append(sampleMismatch, fmt.Sprintf("unifiedId=%s ids=%v → 0 conv | hasConversation DB=%v", uid, ids, hasConvInDB))
				}
			}
		}
	}

	fmt.Printf("   Engaged CÓ match conversation: %d\n", engagedWithMatch)
	fmt.Printf("   Engaged KHÔNG match: %d\n", engagedWithoutMatch)
	fmt.Printf("   Trong đó: KHÔNG match conv nhưng hasConversation=true: %d (có thể match qua fb_messages/expandIds)\n", engagedNoConvButHasConvDB)
	fmt.Printf("   ⚠️ BUG: KHÔNG match conv VÀ hasConversation=false: %d (không nên là engaged)\n", engagedNoConvAndNoHasConvDB)
	fmt.Printf("   Đã kiểm tra (sample): %d\n", sampleChecked)
	if sampleChecked > 0 {
		pct := float64(engagedWithMatch) / float64(sampleChecked) * 100
		fmt.Printf("   Tỷ lệ match (trong sample): %.1f%%\n", pct)
	}

	if len(sampleMismatch) > 0 {
		fmt.Println("\n   Sample engaged KHÔNG match:")
		for _, s := range sampleMismatch {
			fmt.Printf("     - %s\n", s)
		}
	}

	// --- 4. pc_pos_customers.posData.fb_id (conversationId) — engaged không có POS ---
	fmt.Println("\n4. pc_pos_customers.posData.fb_id (conversationId) — dùng cho link POS→conv")
	posWithFbId, _ := posColl.CountDocuments(ctx, bson.M{"posData.fb_id": bson.M{"$exists": true, "$ne": ""}})
	fmt.Printf("   pc_pos_customers có posData.fb_id: %d (engaged thường không có vì chưa mua)\n", posWithFbId)

	fmt.Println("\n--- Kết luận ---")
	if engagedNoConvAndNoHasConvDB > 0 {
		fmt.Printf("❌ LOGIC SAI: %d engaged có hasConversation=false và KHÔNG match conv. Theo rule RULE_CRM_CLASSIFICATION, engaged BẮT BUỘC phải có conversation.\n", engagedNoConvAndNoHasConvDB)
		fmt.Println("   → Cần Recalculate cho các customer này để chuyển về visitor.")
	} else if engagedWithMatch == 0 && engagedTotal > 0 {
		fmt.Println("⚠️ LOGIC LINK SAI: Không có engaged customer nào match được fb_conversations.")
		fmt.Println("   Nguyên nhân có thể: unifiedId/sourceIds.fb format khác customerId/panCakeData trong fb_conversations.")
		fmt.Println("   Cần so sánh format ID giữa 2 collection (xem mục 1, 2).")
	} else if engagedWithMatch > 0 && engagedWithoutMatch > 0 {
		fmt.Printf("⚠️ MỘT PHẦN: %d engaged match, %d không match (có thể match qua fb_messages hoặc expandCustomerIds).\n", engagedWithMatch, engagedWithoutMatch)
	} else if engagedWithMatch > 0 {
		fmt.Println("✅ Link logic đúng — engaged customers match được conversations.")
	}
}

func buildIdsFromCustomer(c bson.M) []string {
	var ids []string
	add := func(s string) {
		if s != "" {
			ids = append(ids, s)
		}
	}
	if uid, ok := c["unifiedId"].(string); ok {
		add(uid)
	}
	if si, ok := c["sourceIds"].(bson.M); ok {
		if p, ok := si["pos"].(string); ok {
			add(p)
		}
		if f, ok := si["fb"].(string); ok {
			add(f)
		}
	}
	return ids
}

func addIdsToSet(c bson.M, set map[string]bool) {
	if uid, ok := c["unifiedId"].(string); ok && uid != "" {
		set[uid] = true
	}
	if si, ok := c["sourceIds"].(bson.M); ok {
		if p, ok := si["pos"].(string); ok && p != "" {
			set[p] = true
		}
		if f, ok := si["fb"].(string); ok && f != "" {
			set[f] = true
		}
	}
}

func getNestedInt64(m bson.M, keys ...string) int64 {
	v := getNested(m, keys...)
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func getNestedInt(m bson.M, keys ...string) int {
	return int(getNestedInt64(m, keys...))
}

func getNested(m bson.M, keys ...string) interface{} {
	for _, k := range keys {
		if m == nil {
			return nil
		}
		v, ok := m[k]
		if !ok {
			return nil
		}
		if next, ok := v.(bson.M); ok {
			m = next
		} else {
			return v
		}
	}
	return m
}

func getNestedBool(m bson.M, keys ...string) bool {
	v := getNested(m, keys...)
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		return x == "1" || strings.EqualFold(x, "true")
	}
	return false
}
