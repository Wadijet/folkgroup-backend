// Script kiểm tra kỹ 611 visitor: tìm mọi liên kết có thể sang fb_conversations.
//
// Các path đã biết (theo _LINKAGE_KEYS):
// - customerId, panCakeData.customer_id, panCakeData.customers[].id, page_customer.id, conversationId
//
// Kiểm tra thêm:
// 1. Cấu trúc panCakeData thực tế — có field nào khác chứa customer id?
// 2. fb_messages — customerId, panCakeData
// 3. Phone matching — conv có phone trong panCakeData?
// 4. PSID — fb_customers.psid, conversationId = pageId_psid?
// 5. Tìm unifiedId xuất hiện BẤT KỲ ĐÂU trong document conv (recursive)
//
// Chạy: go run scripts/audit_611_visitors_deep.go <ownerOrganizationId>
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
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

func getStr(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case int:
		return fmt.Sprintf("%d", x)
	}
	return fmt.Sprintf("%v", v)
}

// extractAllStrings đệ quy lấy mọi string có thể là ID (UUID, numeric, pageId_psid) từ map/array
func extractAllStrings(m interface{}, out map[string]bool) {
	if m == nil {
		return
	}
	switch v := m.(type) {
	case string:
		s := strings.TrimSpace(v)
		if len(s) >= 10 && (isUUID(s) || isNumeric(s) || (strings.Contains(s, "_") && len(s) > 15)) {
			out[s] = true
		}
	case map[string]interface{}:
		for _, val := range v {
			extractAllStrings(val, out)
		}
	case []interface{}:
		for _, item := range v {
			extractAllStrings(item, out)
		}
	}
}

func isUUID(s string) bool {
	matched, _ := regexp.MatchString(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`, s)
	return matched
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// normalizePhone chuẩn hóa SĐT để so sánh
func normalizePhone(p string) string {
	p = regexp.MustCompile(`\D`).ReplaceAllString(p, "")
	if len(p) >= 9 && (strings.HasPrefix(p, "84") || strings.HasPrefix(p, "0")) {
		if strings.HasPrefix(p, "0") {
			p = "84" + p[1:]
		} else if strings.HasPrefix(p, "84") {
			// ok
		}
	}
	return p
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
		log.Fatal("Chạy: go run scripts/audit_611_visitors_deep.go <ownerOrganizationId>")
	}
	orgID, err := primitive.ObjectIDFromHex(os.Args[1])
	if err != nil {
		log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	fbCustColl := db.Collection("fb_customers")
	convColl := db.Collection("fb_conversations")
	msgColl := db.Collection("fb_messages")

	fmt.Println("=== Kiểm tra kỹ 611 visitor — tìm mọi liên kết sang fb_conversations ===\n")

	filter := bson.M{
		"ownerOrganizationId": orgID,
		"journeyStage":        "visitor",
		"primarySource":      "fb",
	}
	cur, _ := crmColl.Find(ctx, filter, options.Find().SetProjection(bson.M{
		"unifiedId": 1, "sourceIds": 1, "profile.phoneNumbers": 1, "profile.name": 1,
	}))
	var visitors []bson.M
	cur.All(ctx, &visitors)
	cur.Close(ctx)

	// Enrich từ fb_customers
	visitorData := make(map[string]struct {
		UnifiedId string
		FbId      string
		PosId     string
		Phones    []string
		Psid      string
		PageId    string
	})
	for _, doc := range visitors {
		uid := getStr(doc["unifiedId"])
		sourceIds, _ := doc["sourceIds"].(map[string]interface{})
		fbId := getStr(sourceIds["fb"])
		posId := getStr(sourceIds["pos"])
		phones := []string{}
		if prof, ok := doc["profile"].(map[string]interface{}); ok {
			if arr, ok := prof["phoneNumbers"].([]interface{}); ok {
				for _, p := range arr {
					if s := getStr(p); s != "" {
						phones = append(phones, s)
					}
				}
			}
		}
		vd := struct {
			UnifiedId string
			FbId      string
			PosId     string
			Phones    []string
			Psid      string
			PageId    string
		}{uid, fbId, posId, phones, "", ""}
		var fbDoc bson.M
		if fbCustColl.FindOne(ctx, bson.M{"customerId": uid, "ownerOrganizationId": orgID}).Decode(&fbDoc) == nil {
			vd.Psid = getStr(fbDoc["psid"])
			vd.PageId = getStr(fbDoc["pageId"])
			if pd, ok := fbDoc["panCakeData"].(map[string]interface{}); ok {
				if pid := getStr(pd["psid"]); pid != "" {
					vd.Psid = pid
				}
				if pgid := getStr(pd["page_id"]); pgid != "" {
					vd.PageId = pgid
				}
				if arr, ok := pd["phone_numbers"].([]interface{}); ok {
					for _, p := range arr {
						if s := getStr(p); s != "" {
							vd.Phones = append(vd.Phones, s)
						}
					}
				}
			}
		}
		visitorData[uid] = vd
	}

	// 1. Cấu trúc panCakeData — mẫu 3 conv, liệt kê keys (2 cấp)
	fmt.Println("--- 1. Cấu trúc panCakeData trong fb_conversations (mẫu 3 bản ghi) ---")
	cur, _ = convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID}, options.Find().SetLimit(3))
	var convSamples []bson.M
	cur.All(ctx, &convSamples)
	cur.Close(ctx)
	for i, c := range convSamples {
		fmt.Printf("  [%d] conversationId=%s customerId=%s\n", i+1, getStr(c["conversationId"]), getStr(c["customerId"]))
		if pd, ok := c["panCakeData"].(map[string]interface{}); ok {
			for k, v := range pd {
				switch x := v.(type) {
				case map[string]interface{}:
					var subKeys []string
					for sk := range x {
						subKeys = append(subKeys, sk)
					}
					fmt.Printf("       panCakeData.%s: map[%v]\n", k, subKeys)
				case []interface{}:
					fmt.Printf("       panCakeData.%s: array[%d]\n", k, len(x))
				default:
					fmt.Printf("       panCakeData.%s: %T\n", k, v)
				}
			}
		}
	}

	// 2. Build map: mọi string-ID trong conv -> convId (để tìm visitor qua bất kỳ path nào)
	fmt.Println("\n--- 2. Index: mọi ID trong fb_conversations (recursive) -> conversationId ---")
	convIdByAnyId := make(map[string][]string)
	cur, _ = convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID},
		options.Find().SetProjection(bson.M{"conversationId": 1, "customerId": 1, "panCakeData": 1, "pageId": 1}))
	count := 0
	for cur.Next(ctx) {
		var doc bson.M
		if cur.Decode(&doc) != nil {
			continue
		}
		convId := getStr(doc["conversationId"])
		ids := make(map[string]bool)
		extractAllStrings(doc, ids)
		for id := range ids {
			if id != "" {
				convIdByAnyId[id] = append(convIdByAnyId[id], convId)
			}
		}
		count++
		if count%5000 == 0 {
			fmt.Printf("  Đã quét %d conv...\n", count)
		}
	}
	cur.Close(ctx)
	fmt.Printf("  Tổng conv đã quét: %d\n", count)
	fmt.Printf("  Số ID unique trong index: %d\n", len(convIdByAnyId))

	// 3. Với mỗi visitor: có ID nào (unifiedId, fbId, posId) nằm trong index không?
	fmt.Println("\n--- 3. Visitor match qua BẤT KỲ ID trong conv (recursive) ---")
	matchViaAnyId := 0
	var sampleMatch []string
	for uid, vd := range visitorData {
		idsToCheck := []string{uid, vd.FbId, vd.PosId}
		for _, id := range idsToCheck {
			if id == "" {
				continue
			}
			if convs, ok := convIdByAnyId[id]; ok && len(convs) > 0 {
				matchViaAnyId++
				if len(sampleMatch) < 5 {
					sampleMatch = append(sampleMatch, fmt.Sprintf("%s (id=%s -> %d conv)", uid, id, len(convs)))
				}
				break
			}
		}
	}
	fmt.Printf("  Visitor có match qua bất kỳ ID trong conv: %d\n", matchViaAnyId)
	if len(sampleMatch) > 0 {
		for _, s := range sampleMatch {
			fmt.Printf("    %s\n", s)
		}
	}

	// 4. fb_messages — customerId
	fmt.Println("\n--- 4. fb_messages: visitor có customerId trong messages không? ---")
	msgCustomerIds := make(map[string]bool)
	cur, _ = msgColl.Find(ctx, bson.M{"ownerOrganizationId": orgID}, options.Find().SetProjection(bson.M{"customerId": 1}))
	for cur.Next(ctx) {
		var doc bson.M
		if cur.Decode(&doc) != nil {
			continue
		}
		if cid := getStr(doc["customerId"]); cid != "" {
			msgCustomerIds[cid] = true
		}
	}
	cur.Close(ctx)
	matchViaMsg := 0
	for uid, vd := range visitorData {
		if msgCustomerIds[uid] || msgCustomerIds[vd.FbId] || msgCustomerIds[vd.PosId] {
			matchViaMsg++
		}
	}
	fmt.Printf("  Visitor có customerId trong fb_messages: %d\n", matchViaMsg)

	// 5. PSID + pageId -> conversationId (format pageId_psid)
	fmt.Println("\n--- 5. PSID: conversationId = pageId_psid? ---")
	convIdSet := make(map[string]bool)
	cur, _ = convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID}, options.Find().SetProjection(bson.M{"conversationId": 1}))
	for cur.Next(ctx) {
		var doc bson.M
		if cur.Decode(&doc) != nil {
			continue
		}
		convIdSet[getStr(doc["conversationId"])] = true
	}
	cur.Close(ctx)
	matchViaPsid := 0
	for _, vd := range visitorData {
		if vd.Psid != "" && vd.PageId != "" {
			candidate := vd.PageId + "_" + vd.Psid
			if convIdSet[candidate] {
				matchViaPsid++
			}
		}
	}
	fmt.Printf("  Visitor có psid+pageId khớp conversationId: %d\n", matchViaPsid)

	// 6. Phone matching — conv có phone trong panCakeData?
	fmt.Println("\n--- 6. Phone: conv có SĐT trùng visitor không? ---")
	phoneToConvs := make(map[string][]string)
	cur, _ = convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID},
		options.Find().SetProjection(bson.M{"conversationId": 1, "panCakeData": 1}))
	for cur.Next(ctx) {
		var doc bson.M
		if cur.Decode(&doc) != nil {
			continue
		}
		convId := getStr(doc["conversationId"])
		phones := extractPhonesFromDoc(doc)
		for _, p := range phones {
			np := normalizePhone(p)
			if len(np) >= 9 {
				phoneToConvs[np] = append(phoneToConvs[np], convId)
			}
		}
	}
	cur.Close(ctx)
	matchViaPhone := 0
	for _, vd := range visitorData {
		for _, p := range vd.Phones {
			np := normalizePhone(p)
			if len(np) >= 9 && len(phoneToConvs[np]) > 0 {
				matchViaPhone++
				break
			}
		}
	}
	fmt.Printf("  Visitor có phone khớp conv: %d\n", matchViaPhone)

	// 7. Phát hiện: fb_customers.panCakeData.thread_id = pageId_psid = conversationId!
	fmt.Println("\n--- 7. Kiểm tra: fb_customers.panCakeData.thread_id = conversationId? ---")
	matchViaThreadId := 0
	for uid := range visitorData {
		var fbDoc bson.M
		if fbCustColl.FindOne(ctx, bson.M{"customerId": uid, "ownerOrganizationId": orgID}).Decode(&fbDoc) != nil {
			continue
		}
		threadId := ""
		if pd, ok := fbDoc["panCakeData"].(map[string]interface{}); ok {
			threadId = getStr(pd["thread_id"])
		}
		if threadId != "" && convIdSet[threadId] {
			matchViaThreadId++
		}
	}
	fmt.Printf("  Visitor có panCakeData.thread_id khớp conversationId: %d\n", matchViaThreadId)

	// 7b. Kiểm tra panCakeData.customer_id (có thể khác panCakeData.id!)
	fmt.Println("\n--- 7b. Kiểm tra: fb_customers.panCakeData.customer_id (có thể khác id) ---")
	convCustIdSet := make(map[string]bool)
	cur, _ = convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID},
		options.Find().SetProjection(bson.M{"customerId": 1, "panCakeData.customer_id": 1, "panCakeData.customers.id": 1, "panCakeData.customer.id": 1}))
	for cur.Next(ctx) {
		var doc bson.M
		if cur.Decode(&doc) != nil {
			continue
		}
		convCustIdSet[getStr(doc["customerId"])] = true
		if pd, ok := doc["panCakeData"].(map[string]interface{}); ok {
			convCustIdSet[getStr(pd["customer_id"])] = true
			if cust, ok := pd["customer"].(map[string]interface{}); ok {
				convCustIdSet[getStr(cust["id"])] = true
			}
			if arr, ok := pd["customers"].([]interface{}); ok {
				for _, c := range arr {
					if cm, ok := c.(map[string]interface{}); ok {
						convCustIdSet[getStr(cm["id"])] = true
					}
				}
			}
		}
	}
	cur.Close(ctx)
	matchViaPanCakeCustomerId := 0
	for uid := range visitorData {
		var fbDoc bson.M
		if fbCustColl.FindOne(ctx, bson.M{"customerId": uid, "ownerOrganizationId": orgID}).Decode(&fbDoc) != nil {
			continue
		}
		panCakeCustId := ""
		if pd, ok := fbDoc["panCakeData"].(map[string]interface{}); ok {
			panCakeCustId = getStr(pd["customer_id"])
		}
		if panCakeCustId != "" && convCustIdSet[panCakeCustId] {
			matchViaPanCakeCustomerId++
		}
	}
	fmt.Printf("  Visitor có panCakeData.customer_id khớp conv: %d\n", matchViaPanCakeCustomerId)

	// 7c. Mẫu dump fb_customers
	fmt.Println("\n--- 7c. Mẫu fb_customers (ecea1126...) ---")
	sampleUID := "ecea1126-9287-4768-afa2-ae919092747d"
	var fbDoc bson.M
	if fbCustColl.FindOne(ctx, bson.M{"customerId": sampleUID, "ownerOrganizationId": orgID}).Decode(&fbDoc) == nil {
		js, _ := json.MarshalIndent(fbDoc, "  ", "  ")
		fmt.Printf("%s\n", string(js))
		threadId := ""
		if pd, ok := fbDoc["panCakeData"].(map[string]interface{}); ok {
			threadId = getStr(pd["thread_id"])
			fmt.Printf("\n  thread_id=%s -> convIdSet? %v\n", threadId, convIdSet[threadId])
			n, _ := convColl.CountDocuments(ctx, bson.M{"ownerOrganizationId": orgID, "conversationId": threadId})
			fmt.Printf("  Query conv conversationId=%s: %d bản ghi\n", threadId, n)
		}
	}

	// 8. Tổng kết
	fmt.Println("\n--- 8. Tổng kết ---")
	fmt.Printf("  Path đã biết (customerId/panCakeData.*): 0\n")
	fmt.Printf("  Path mới: bất kỳ ID trong conv: %d\n", matchViaAnyId)
	fmt.Printf("  Path mới: fb_messages.customerId: %d\n", matchViaMsg)
	fmt.Printf("  Path mới: psid+pageId=conversationId: %d\n", matchViaPsid)
	fmt.Printf("  Path mới: phone trong panCakeData: %d\n", matchViaPhone)
	fmt.Printf("  Path mới: panCakeData.thread_id=conversationId: %d\n", matchViaThreadId)
	fmt.Printf("  Path mới: panCakeData.customer_id (khác id): %d\n", matchViaPanCakeCustomerId)
}

func extractPhonesFromDoc(doc bson.M) []string {
	var out []string
	add := func(v interface{}) {
		switch x := v.(type) {
		case string:
			if regexp.MustCompile(`^[0-9+\s\-]{9,}$`).MatchString(strings.TrimSpace(x)) {
				out = append(out, x)
			}
		case []interface{}:
			for _, item := range x {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	pd, _ := doc["panCakeData"].(map[string]interface{})
	if pd == nil {
		return out
	}
	add(pd["phone_numbers"])
	if arr, ok := pd["customers"].([]interface{}); ok {
		for _, c := range arr {
			if cm, ok := c.(map[string]interface{}); ok {
				add(cm["phone_numbers"])
			}
		}
	}
	if cust, ok := pd["customer"].(map[string]interface{}); ok {
		add(cust["phone_numbers"])
	}
	if pg, ok := pd["page_customer"].(map[string]interface{}); ok {
		add(pg["phone_numbers"])
	}
	return out
}
