// Script rà soát visitor primarySource=fb theo _LINKAGE_KEYS.md và BAO_CAO_LIEN_KET_DU_LIEU.
//
// Theo tài liệu:
// - crm_customers: sourceIds.fb -> fb_customers.customerId, sourceIds.pos -> pc_pos_customers.customerId
// - fb_conversations: customerId, panCakeData.customers[].id -> fb_customers.customerId (Pancake UUID)
// - pc_pos_customers.posData.fb_id -> fb_conversations.conversationId (pageId_psid)
//
// Kiểm tra: visitor có conv match qua (1) customerId/panCakeData.* hoặc (2) posData.fb_id=conversationId
//
// Chạy: go run scripts/audit_fb_visitors.go <ownerOrganizationId>
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

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.0f", f)
	}
	if i, ok := v.(int64); ok {
		return fmt.Sprintf("%d", i)
	}
	return fmt.Sprintf("%v", v)
}

// buildConvFilter giống buildConversationFilterForCustomerIds
func buildConvFilter(customerIds []string, ownerOrgID primitive.ObjectID) bson.M {
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
	if len(ids) == 0 {
		return bson.M{"ownerOrganizationId": ownerOrgID, "customerId": "__NO_MATCH__"}
	}
	convOr := []bson.M{
		{"customerId": bson.M{"$in": ids}},
		{"panCakeData.customer_id": bson.M{"$in": ids}},
		{"panCakeData.customer.id": bson.M{"$in": ids}},
		{"panCakeData.customers.id": bson.M{"$in": ids}},
		{"panCakeData.page_customer.id": bson.M{"$in": ids}},
		{"panCakeData.page_customer.customer_id": bson.M{"$in": ids}},
	}
	if len(numIds) > 0 {
		convOr = append(convOr,
			bson.M{"panCakeData.customer_id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.customers.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.page_customer.id": bson.M{"$in": numIds}},
			bson.M{"panCakeData.page_customer.customer_id": bson.M{"$in": numIds}},
		)
	}
	return bson.M{"ownerOrganizationId": ownerOrgID, "$or": convOr}
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
	if len(os.Args) < 2 {
		log.Fatal("Chạy: go run scripts/audit_fb_visitors.go <ownerOrganizationId>")
	}
	orgID, err := primitive.ObjectIDFromHex(os.Args[1])
	if err != nil {
		log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	convColl := db.Collection("fb_conversations")
	fbCustColl := db.Collection("fb_customers")
	posCustColl := db.Collection("pc_pos_customers")

	fmt.Println("=== Rà soát visitor primarySource=fb (theo _LINKAGE_KEYS.md) ===\n")

	// 1. Tại sao không recalc? — RecalculateMismatchCustomers chỉ xử lý engaged
	fmt.Println("--- 1. Tại sao 611 visitor không được recalc? ---")
	fmt.Println("  RecalculateMismatchCustomers chỉ xử lý: journeyStage=engaged VÀ visitor trong activity snapshot.")
	fmt.Println("  Visitor (journeyStage=visitor) nằm NGOÀI scope — không bao giờ được gọi RecalculateCustomerFromAllSources.\n")

	// 2. Lấy tất cả FB visitors
	filter := bson.M{
		"ownerOrganizationId": orgID,
		"journeyStage":        "visitor",
		"primarySource":      "fb",
	}
	total, _ := crmColl.CountDocuments(ctx, filter)
	fmt.Printf("--- 2. Tổng visitor primarySource=fb: %d ---\n\n", total)

	// 3. Mẫu customerId format trong fb_conversations
	fmt.Println("--- 3. Mẫu customerId trong fb_conversations (5 bản ghi) ---")
	cur, _ := convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID},
		options.Find().SetProjection(bson.M{"customerId": 1, "conversationId": 1, "panCakeData.customer_id": 1, "panCakeData.customer.id": 1}).SetLimit(5))
	var convSamples []bson.M
	cur.All(ctx, &convSamples)
	cur.Close(ctx)
	for i, c := range convSamples {
		custId := getStr(c, "customerId")
		panCustId := ""
		if pan, ok := c["panCakeData"].(map[string]interface{}); ok {
			panCustId = getStr(pan, "customer_id")
			if panCustId == "" {
				if cust, ok := pan["customer"].(map[string]interface{}); ok {
					panCustId = getStr(cust, "id")
				}
			}
		}
		fmt.Printf("  [%d] customerId=%q panCakeData.customer_id=%q (UUID=%v, numeric=%v)\n",
			i+1, custId, panCustId, isUUID(custId), isNumeric(custId))
	}

	// 4. Phân tích theo _LINKAGE_KEYS: visitor có match conv qua 2 path
	// Path A: customerId/panCakeData.customers[].id (crm sourceIds.fb = fb_customers.customerId = conv customer)
	// Path B: posData.fb_id = conversationId (pc_pos_customers.posData.fb_id -> fb_conversations)
	fmt.Println("\n--- 4. Kiểm tra theo _LINKAGE_KEYS.md ---")
	fmt.Println("  Path A: sourceIds.fb/unifiedId -> fb_conversations.customerId hoặc panCakeData.customers[].id")
	fmt.Println("  Path B: sourceIds.pos -> pc_pos_customers.posData.fb_id -> fb_conversations.conversationId")

	cur, _ = crmColl.Find(ctx, filter, options.Find().SetProjection(bson.M{"unifiedId": 1, "sourceIds": 1}))
	var visitors []bson.M
	cur.All(ctx, &visitors)
	cur.Close(ctx)

	// Path A: Lấy tất cả customer ids từ fb_conversations (theo check_linkage_fb_conversation_customer.go)
	convCustIds := make(map[string]bool)
	cur, _ = convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID},
		options.Find().SetProjection(bson.M{"customerId": 1, "panCakeData": 1}))
	var convDocs []bson.M
	cur.All(ctx, &convDocs)
	cur.Close(ctx)
	extractIds := func(m map[string]interface{}, keys ...string) {
		for _, k := range keys {
			if v := getStr(m, k); v != "" {
				convCustIds[v] = true
			}
		}
	}
	for _, d := range convDocs {
		extractIds(d, "customerId")
		if pan, ok := d["panCakeData"].(map[string]interface{}); ok {
			extractIds(pan, "customer_id")
			if cust, ok := pan["customer"].(map[string]interface{}); ok {
				extractIds(cust, "id")
			}
			if arr, ok := pan["customers"].([]interface{}); ok {
				for _, c := range arr {
					if cm, ok := c.(map[string]interface{}); ok {
						extractIds(cm, "id")
					}
				}
			}
			if pg, ok := pan["page_customer"].(map[string]interface{}); ok {
				extractIds(pg, "id", "customer_id")
			}
		}
	}

	// Path B: posData.fb_id -> conversationId. Lấy conversationIds từ pc_pos_customers (posId -> fb_id)
	posIdToConvId := make(map[string]string)
	posCur, _ := posCustColl.Find(ctx, bson.M{"ownerOrganizationId": orgID, "posData.fb_id": bson.M{"$exists": true, "$ne": ""}},
		options.Find().SetProjection(bson.M{"customerId": 1, "posData.fb_id": 1}))
	var posDocs []bson.M
	posCur.All(ctx, &posDocs)
	posCur.Close(ctx)
	for _, p := range posDocs {
		posId := getStr(p, "customerId")
		if posId == "" {
			continue
		}
		if pd, ok := p["posData"].(map[string]interface{}); ok {
			fbId := getStr(pd, "fb_id")
			if fbId != "" {
				posIdToConvId[posId] = fbId
			}
		}
	}
	convIdSet := make(map[string]bool)
	cur, _ = convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID}, options.Find().SetProjection(bson.M{"conversationId": 1}))
	var convIds []bson.M
	cur.All(ctx, &convIds)
	cur.Close(ctx)
	for _, c := range convIds {
		if id := getStr(c, "conversationId"); id != "" {
			convIdSet[id] = true
		}
	}

	matchCount := 0
	matchPathA := 0
	matchPathB := 0
	noMatchCount := 0
	sampleMatch := []string{}
	sampleNoMatch := []string{}
	for _, doc := range visitors {
		uid := getStr(doc, "unifiedId")
		sourceIds, _ := doc["sourceIds"].(map[string]interface{})
		fbId := getStr(sourceIds, "fb")
		posId := getStr(sourceIds, "pos")

		viaA := convCustIds[uid] || convCustIds[fbId] || convCustIds[posId]
		viaB := false
		if posId != "" {
			if convId, ok := posIdToConvId[posId]; ok && convIdSet[convId] {
				viaB = true
			}
		}
		hasMatch := viaA || viaB
		if hasMatch {
			matchCount++
			if viaA {
				matchPathA++
			}
			if viaB {
				matchPathB++
			}
			if len(sampleMatch) < 5 {
				sampleMatch = append(sampleMatch, uid)
			}
		} else {
			noMatchCount++
			if len(sampleNoMatch) < 5 {
				sampleNoMatch = append(sampleNoMatch, uid)
			}
		}
	}

	fmt.Printf("  Có conv match (recalc sẽ → engaged): %d\n", matchCount)
	fmt.Printf("    - Qua Path A (customerId/panCakeData): %d\n", matchPathA)
	fmt.Printf("    - Qua Path B (posData.fb_id=conversationId): %d\n", matchPathB)
	fmt.Printf("  Không match conv: %d\n", noMatchCount)
	fmt.Printf("  Mẫu có match: %v\n", sampleMatch)
	fmt.Printf("  Mẫu không match: %v\n", sampleNoMatch)

	// 5. Với visitor không match: kiểm tra fb_customers có tồn tại?
	fmt.Println("\n--- 5. Visitor không match: unifiedId có trong fb_customers không? ---")
	noMatchInFb := 0
	noMatchNotInFb := 0
	for _, doc := range visitors {
		uid := getStr(doc, "unifiedId")
		sourceIds, _ := doc["sourceIds"].(map[string]interface{})
		fbId := getStr(sourceIds, "fb")
		posId := getStr(sourceIds, "pos")

		hasMatch := convCustIds[uid] || convCustIds[fbId] || convCustIds[posId]
		if hasMatch {
			continue
		}

		// Không match conv — kiểm tra fb_customers
		var found bool
		for _, id := range []string{uid, fbId, posId} {
			if id == "" {
				continue
			}
			var fbDoc bson.M
			err := fbCustColl.FindOne(ctx, bson.M{"customerId": id, "ownerOrganizationId": orgID}).Decode(&fbDoc)
			if err == nil {
				found = true
				break
			}
		}
		if found {
			noMatchInFb++
		} else {
			noMatchNotInFb++
		}
	}
	fmt.Printf("  Không match conv nhưng có trong fb_customers: %d (linkage conv sai?)\n", noMatchInFb)
	fmt.Printf("  Không match conv và không có trong fb_customers: %d (orphan?)\n", noMatchNotInFb)

	// 6. Mẫu 1 visitor: tìm conv có chứa unifiedId ở bất kỳ đâu
	fmt.Println("\n--- 6. Mẫu visitor: conv có chứa unifiedId ở đâu? (text search) ---")
	sampleUID := "ecea1126-9287-4768-afa2-ae919092747d"
	anyFieldFilter := bson.M{
		"ownerOrganizationId": orgID,
		"$or": []bson.M{
			{"customerId": sampleUID},
			{"conversationId": sampleUID},
			{"panCakeData.customer_id": sampleUID},
			{"panCakeData.customer.id": sampleUID},
			{"panCakeData.page_customer.id": sampleUID},
			{"panCakeData.page_customer.customer_id": sampleUID},
		},
	}
	nAny, _ := convColl.CountDocuments(ctx, anyFieldFilter)
	fmt.Printf("  Conv có %s: %d\n", sampleUID, nAny)
	// Kiểm tra fb_messages
	msgColl := db.Collection("fb_messages")
	nMsg, _ := msgColl.CountDocuments(ctx, bson.M{"ownerOrganizationId": orgID, "customerId": sampleUID})
	fmt.Printf("  fb_messages có customerId=%s: %d\n", sampleUID, nMsg)

	// 7. Gợi ý
	fmt.Println("\n--- 7. Gợi ý ---")
	if matchCount > 0 {
		fmt.Printf("  • Thêm RecalculateFbVisitors: recalc %d visitor có conv match → sẽ chuyển thành engaged.\n", matchCount)
	}
	fmt.Println("  • RecalculateMismatchCustomers không xử lý visitor — cần hàm mới hoặc mở rộng filter.")
}

func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			// ok
		} else {
			return false
		}
	}
	return true
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
