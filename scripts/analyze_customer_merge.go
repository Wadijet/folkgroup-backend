// Script phân tích dữ liệu khách hàng từ fb_customers và pc_pos_customers
// để đề xuất phương án merge cho hệ thống phân loại khách hàng.
//
// Chạy: go run scripts/analyze_customer_merge.go
// Cần: MONGODB_CONNECTION_URI, MONGODB_DBNAME_AUTH (từ .env hoặc env vars)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func loadScriptConfig() (uri, dbName string) {
	// Thử load .env từ nhiều vị trí
	tryPaths := []string{
		".env",
		"api/.env",
		"config/env/development.env",
		"api/config/env/development.env",
	}
	cwd, _ := os.Getwd()
	for _, p := range tryPaths {
		full := filepath.Join(cwd, p)
		if _, err := os.Stat(full); err == nil {
			_ = godotenv.Load(full)
			break
		}
		// Thử từ thư mục cha (khi chạy từ api/)
		parent := filepath.Dir(cwd)
		full = filepath.Join(parent, p)
		if _, err := os.Stat(full); err == nil {
			_ = godotenv.Load(full)
			break
		}
	}
	uri = os.Getenv("MONGODB_CONNECTION_URI")
	dbName = os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI") // fallback
	}
	return uri, dbName
}

func main() {
	fmt.Println("=== Phân Tích Merge Khách Hàng FB + POS ===\n")

	uri, dbName := loadScriptConfig()
	if uri == "" || dbName == "" {
		log.Fatal("Cần set MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH (trong .env hoặc env vars).\n" +
			"Ví dụ: tạo file .env với:\n  MONGODB_CONNECTION_URI=mongodb://localhost:27017\n  MONGODB_DBNAME_AUTH=your_db_name")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("Không thể kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Không thể ping MongoDB: %v", err)
	}

	db := client.Database(dbName)

	// 1. Thống kê cơ bản
	analyzeCounts(ctx, db)

	// 2. Phân tích format customerId
	analyzeCustomerIdFormat(ctx, db)

	// 3. Phân tích phone numbers
	analyzePhoneNumbers(ctx, db)

	// 4. Tìm overlap customerId giữa FB và POS
	analyzeCustomerIdOverlap(ctx, db)

	// 5. Phân tích orders - customerId vs billPhoneNumber
	analyzeOrdersCustomerLink(ctx, db)

	// 6. Phân tích conversations - customerId
	analyzeConversationsCustomer(ctx, db)

	// 7. Sample documents để xem cấu trúc
	printSampleDocuments(ctx, db)

	// === KHẢO SÁT SÂU: MERGE POTENTIAL ===
	fmt.Println("\n========== KHẢO SÁT SÂU: TIỀM NĂNG MERGE ==========\n")

	// 8. posData.fb_id — POS có link tới FB không?
	analyzePosFbId(ctx, db)

	// 9. Merge theo phone (chuẩn hóa)
	analyzePhoneMergePotential(ctx, db)

	// 10. conversation_link trong posData
	analyzeConversationLink(ctx, db)

	// 11. Orders: customerId POS vs billPhone — có thể link qua phone?
	analyzeOrderToCustomerLink(ctx, db)

	// 12. Tổng hợp: có thể merge bao nhiêu?
	analyzeMergeSummary(ctx, db)

	fmt.Println("\n✓ Hoàn thành phân tích")
}

func analyzeCounts(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 1. THỐNG KÊ SỐ LƯỢNG ---")

	fbCount, _ := db.Collection("fb_customers").CountDocuments(ctx, bson.M{})
	posCount, _ := db.Collection("pc_pos_customers").CountDocuments(ctx, bson.M{})
	convCount, _ := db.Collection("fb_conversations").CountDocuments(ctx, bson.M{})
	orderCount, _ := db.Collection("pc_pos_orders").CountDocuments(ctx, bson.M{})

	fmt.Printf("  fb_customers:        %d\n", fbCount)
	fmt.Printf("  pc_pos_customers:    %d\n", posCount)
	fmt.Printf("  fb_conversations:    %d\n", convCount)
	fmt.Printf("  pc_pos_orders:      %d\n", orderCount)
	fmt.Println()
}

func analyzeCustomerIdFormat(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 2. FORMAT CUSTOMER ID ---")

	// FB
	fbColl := db.Collection("fb_customers")
	cursor, _ := fbColl.Find(ctx, bson.M{}, options.Find().SetLimit(20))
	var fbIds []string
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		if id, ok := d["customerId"].(string); ok && id != "" {
			fbIds = append(fbIds, id)
		}
	}
	cursor.Close(ctx)

	// POS
	posColl := db.Collection("pc_pos_customers")
	cursor, _ = posColl.Find(ctx, bson.M{}, options.Find().SetLimit(20))
	var posIds []string
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		if id, ok := d["customerId"].(string); ok && id != "" {
			posIds = append(posIds, id)
		}
	}
	cursor.Close(ctx)

	fmt.Printf("  FB customerId mẫu (max 5): ")
	for i, id := range fbIds {
		if i >= 5 {
			break
		}
		fmt.Printf("%q ", id)
	}
	fmt.Println()

	fmt.Printf("  POS customerId mẫu (max 5): ")
	for i, id := range posIds {
		if i >= 5 {
			break
		}
		fmt.Printf("%q ", id)
	}
	fmt.Println()

	// Phân loại format
	uuidRegex := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	for _, id := range fbIds {
		if uuidRegex.MatchString(id) {
			fmt.Printf("  FB: customerId có format UUID\n")
			break
		}
	}
	for _, id := range posIds {
		if uuidRegex.MatchString(id) {
			fmt.Printf("  POS: customerId có format UUID\n")
			break
		}
	}
	fmt.Println()
}

func normalizePhone(phone string) string {
	// Bỏ khoảng trắng, dấu gạch
	s := regexp.MustCompile(`[\s\-\.\(\)]`).ReplaceAllString(phone, "")
	// Chuẩn hóa đầu số VN: 84xxx, 0xxx -> 84xxx
	if strings.HasPrefix(s, "0") && len(s) >= 10 {
		s = "84" + s[1:]
	} else if strings.HasPrefix(s, "+84") {
		s = "84" + s[3:]
	} else if !strings.HasPrefix(s, "84") && len(s) == 9 {
		s = "84" + s
	}
	return s
}

func extractPhones(doc bson.M) []string {
	var phones []string
	if arr, ok := doc["phoneNumbers"].(bson.A); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok && s != "" {
				phones = append(phones, s)
			}
		}
	}
	if doc["posData"] != nil {
		if pd, ok := doc["posData"].(bson.M); ok {
			if arr, ok := pd["phone_numbers"].(bson.A); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok && s != "" {
						phones = append(phones, s)
					}
				}
			}
		}
	}
	if doc["panCakeData"] != nil {
		if pd, ok := doc["panCakeData"].(bson.M); ok {
			if arr, ok := pd["phone_numbers"].(bson.A); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok && s != "" {
						phones = append(phones, s)
					}
				}
			}
		}
	}
	return phones
}

func analyzePhoneNumbers(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 3. PHÂN TÍCH SỐ ĐIỆN THOẠI ---")

	fbColl := db.Collection("fb_customers")
	cursor, _ := fbColl.Find(ctx, bson.M{}, options.Find().SetLimit(100))
	fbWithPhone := 0
	var fbPhoneSamples []string
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		phones := extractPhones(d)
		if len(phones) > 0 {
			fbWithPhone++
			if len(fbPhoneSamples) < 5 {
				fbPhoneSamples = append(fbPhoneSamples, phones[0])
			}
		}
	}
	cursor.Close(ctx)

	posColl := db.Collection("pc_pos_customers")
	cursor, _ = posColl.Find(ctx, bson.M{}, options.Find().SetLimit(100))
	posWithPhone := 0
	var posPhoneSamples []string
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		phones := extractPhones(d)
		if len(phones) > 0 {
			posWithPhone++
			if len(posPhoneSamples) < 5 {
				posPhoneSamples = append(posPhoneSamples, phones[0])
			}
		}
	}
	cursor.Close(ctx)

	fmt.Printf("  FB có phone (trong 100 mẫu): %d\n", fbWithPhone)
	fmt.Printf("  FB phone mẫu: %v\n", fbPhoneSamples)
	fmt.Printf("  POS có phone (trong 100 mẫu): %d\n", posWithPhone)
	fmt.Printf("  POS phone mẫu: %v\n", posPhoneSamples)
	fmt.Println()
}

func analyzeCustomerIdOverlap(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 4. OVERLAP CUSTOMER ID (FB ∩ POS) ---")

	fbColl := db.Collection("fb_customers")
	cursor, _ := fbColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"customerId": 1}))
	fbIds := make(map[string]bool)
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		if id, ok := d["customerId"].(string); ok && id != "" {
			fbIds[id] = true
		}
	}
	cursor.Close(ctx)

	posColl := db.Collection("pc_pos_customers")
	cursor, _ = posColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"customerId": 1}))
	overlapCount := 0
	var overlapSamples []string
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		if id, ok := d["customerId"].(string); ok && id != "" {
			if fbIds[id] {
				overlapCount++
				if len(overlapSamples) < 5 {
					overlapSamples = append(overlapSamples, id)
				}
			}
		}
	}
	cursor.Close(ctx)

	fmt.Printf("  Số customerId có trong CẢ FB và POS: %d\n", overlapCount)
	if len(overlapSamples) > 0 {
		fmt.Printf("  Mẫu overlap: %v\n", overlapSamples)
	}
	fmt.Println()
}

func analyzeOrdersCustomerLink(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 5. ORDERS - CUSTOMER ID vs BILL PHONE ---")

	orderColl := db.Collection("pc_pos_orders")
	pipe := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":            nil,
			"total":          bson.M{"$sum": 1},
			"withCustomerId": bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$and": []interface{}{bson.M{"$ne": []interface{}{"$customerId", ""}}, bson.M{"$ne": []interface{}{"$customerId", nil}}}}, 1, 0}}},
			"withBillPhone":  bson.M{"$sum": bson.M{"$cond": []interface{}{bson.M{"$and": []interface{}{bson.M{"$ne": []interface{}{"$billPhoneNumber", ""}}, bson.M{"$ne": []interface{}{"$billPhoneNumber", nil}}}}, 1, 0}}},
		}}},
	}
	cursor, err := orderColl.Aggregate(ctx, pipe)
	if err != nil {
		fmt.Printf("  Lỗi aggregate: %v\n", err)
		return
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		var r bson.M
		cursor.Decode(&r)
		total := getInt64(r, "total")
		withCust := getInt64(r, "withCustomerId")
		withPhone := getInt64(r, "withBillPhone")
		fmt.Printf("  Tổng orders: %d\n", total)
		fmt.Printf("  Có customerId: %d (%.1f%%)\n", withCust, float64(withCust)/float64(max(1, total))*100)
		fmt.Printf("  Có billPhoneNumber: %d (%.1f%%)\n", withPhone, float64(withPhone)/float64(max(1, total))*100)
	}

	// Thử match customerId từ posData
	orFilter := bson.M{"$or": []bson.M{
		{"customerId": bson.M{"$exists": true, "$ne": ""}},
		{"posData.customer.id": bson.M{"$exists": true, "$ne": ""}},
	}}
	pipe2 := mongo.Pipeline{
		{{Key: "$match", Value: orFilter}},
		{{Key: "$limit", Value: 5}},
		{{Key: "$project", Value: bson.M{"customerId": 1, "billPhoneNumber": 1, "posData.customer": 1}}},
	}
	cursor2, _ := orderColl.Aggregate(ctx, pipe2)
	fmt.Printf("  Mẫu order customerId/billPhone: ")
	count := 0
	for cursor2.Next(ctx) {
		var d bson.M
		cursor2.Decode(&d)
		cid := ""
		if v, ok := d["customerId"].(string); ok {
			cid = v
		}
		bill := ""
		if v, ok := d["billPhoneNumber"].(string); ok {
			bill = v
		}
		if count < 3 {
			fmt.Printf("[custId=%s billPhone=%s] ", trunc(cid, 12), trunc(bill, 12))
			count++
		}
	}
	cursor2.Close(ctx)
	fmt.Println()
	fmt.Println()
}

func analyzeConversationsCustomer(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 6. CONVERSATIONS - CUSTOMER ID ---")

	convColl := db.Collection("fb_conversations")
	withCust, _ := convColl.CountDocuments(ctx, bson.M{"customerId": bson.M{"$exists": true, "$ne": ""}})
	total, _ := convColl.CountDocuments(ctx, bson.M{})
	fmt.Printf("  Tổng conversations: %d\n", total)
	fmt.Printf("  Có customerId: %d (%.1f%%)\n", withCust, float64(withCust)/float64(max(1, total))*100)
	fmt.Println()
}

func printSampleDocuments(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 7. DOCUMENT MẪU (để xem cấu trúc) ---")

	// FB customer - chỉ các field quan trọng
	var fbDoc bson.M
	db.Collection("fb_customers").FindOne(ctx, bson.M{}).Decode(&fbDoc)
	if fbDoc != nil {
		fmt.Println("  fb_customers (1 doc):")
		fmt.Printf("    customerId: %v\n", fbDoc["customerId"])
		fmt.Printf("    name: %v\n", fbDoc["name"])
		fmt.Printf("    phoneNumbers: %v\n", fbDoc["phoneNumbers"])
		fmt.Printf("    email: %v\n", fbDoc["email"])
		fmt.Printf("    pageId: %v\n", fbDoc["pageId"])
		fmt.Printf("    psid: %v\n", fbDoc["psid"])
	}

	var posDoc bson.M
	db.Collection("pc_pos_customers").FindOne(ctx, bson.M{}).Decode(&posDoc)
	if posDoc != nil {
		fmt.Println("  pc_pos_customers (1 doc):")
		fmt.Printf("    customerId: %v\n", posDoc["customerId"])
		fmt.Printf("    name: %v\n", posDoc["name"])
		fmt.Printf("    phoneNumbers: %v\n", posDoc["phoneNumbers"])
		fmt.Printf("    totalSpent: %v\n", posDoc["totalSpent"])
		fmt.Printf("    succeedOrderCount: %v\n", posDoc["succeedOrderCount"])
		fmt.Printf("    lastOrderAt: %v\n", posDoc["lastOrderAt"])
		if pd, ok := posDoc["posData"].(bson.M); ok {
			fmt.Printf("    posData.keys: %v\n", getKeys(pd))
		}
	}
	fmt.Println()
}

// analyzePosFbId kiểm tra posData.fb_id — POS customer có link tới FB customer không?
func analyzePosFbId(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 8. POS posData.fb_id (link tới FB?) ---")

	posColl := db.Collection("pc_pos_customers")
	cursor, _ := posColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"customerId": 1, "posData.fb_id": 1, "posData.conversation_link": 1}))
	posWithFbId := 0
	var fbIdSamples []string
	posFbIdToPosId := make(map[string]string) // fb_id -> pos customerId
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		posId, _ := d["customerId"].(string)
		if pd, ok := d["posData"].(bson.M); ok {
			fbId := getStringFromAny(pd["fb_id"])
			if fbId != "" {
				posWithFbId++
				if len(fbIdSamples) < 5 {
					fbIdSamples = append(fbIdSamples, fbId)
				}
				posFbIdToPosId[fbId] = posId
			}
		}
	}
	cursor.Close(ctx)

	posTotal, _ := posColl.CountDocuments(ctx, bson.M{})
	fmt.Printf("  POS có posData.fb_id: %d / %d (%.1f%%)\n", posWithFbId, posTotal, float64(posWithFbId)/float64(max(1, posTotal))*100)
	fmt.Printf("  Mẫu fb_id: %v\n", fbIdSamples)

	// fb_id có match với fb_customers.customerId hoặc psid không?
	// Format fb_id mẫu: "157725629736743_26258510603755268" → có thể là pageId_psid
	fbColl := db.Collection("fb_customers")
	cursor, _ = fbColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"customerId": 1, "psid": 1, "pageId": 1}))
	fbCustomerIds := make(map[string]bool)
	fbPsids := make(map[string]bool)
	fbPageIdPsid := make(map[string]bool) // "pageId_psid"
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		if id, ok := d["customerId"].(string); ok && id != "" {
			fbCustomerIds[id] = true
		}
		psid := getStringFromAny(d["psid"])
		pageId := getStringFromAny(d["pageId"])
		if psid != "" {
			fbPsids[psid] = true
			if pageId != "" {
				fbPageIdPsid[pageId+"_"+psid] = true
			}
		}
	}
	cursor.Close(ctx)

	mergeByFbId := 0
	mergeByPageIdPsid := 0
	for fbId := range posFbIdToPosId {
		if fbCustomerIds[fbId] || fbPsids[fbId] {
			mergeByFbId++
		}
		if fbPageIdPsid[fbId] {
			mergeByPageIdPsid++
		}
	}
	fmt.Printf("  → Match fb_customers.customerId hoặc psid: %d\n", mergeByFbId)
	fmt.Printf("  → Match format pageId_psid (posData.fb_id): %d\n", mergeByPageIdPsid)
	mergeFbIdBest := mergeByFbId
	if mergeByPageIdPsid > mergeFbIdBest {
		mergeFbIdBest = mergeByPageIdPsid
	}
	fmt.Printf("  → Có thể merge qua fb_id: %d khách POS\n", mergeFbIdBest)
	fmt.Println()
}

func getStringFromAny(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case int:
		return fmt.Sprintf("%d", x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// analyzePhoneMergePotential đếm có thể merge bao nhiêu qua phone (chuẩn hóa)
func analyzePhoneMergePotential(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 9. MERGE THEO PHONE (chuẩn hóa) ---")

	// Build map: normalizedPhone -> []{source, customerId}
	fbColl := db.Collection("fb_customers")
	cursor, _ := fbColl.Find(ctx, bson.M{})
	phoneToFb := make(map[string][]string) // normalized -> fb customerIds
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		cid, _ := d["customerId"].(string)
		phones := extractPhones(d)
		for _, p := range phones {
			np := normalizePhone(p)
			if len(np) >= 9 {
				phoneToFb[np] = append(phoneToFb[np], cid)
			}
		}
	}
	cursor.Close(ctx)

	posColl := db.Collection("pc_pos_customers")
	cursor, _ = posColl.Find(ctx, bson.M{})
	phoneToPos := make(map[string][]string)
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		cid, _ := d["customerId"].(string)
		phones := extractPhones(d)
		for _, p := range phones {
			np := normalizePhone(p)
			if len(np) >= 9 {
				phoneToPos[np] = append(phoneToPos[np], cid)
			}
		}
	}
	cursor.Close(ctx)

	// Số phone có trong CẢ FB và POS
	mergePhones := 0
	var samplePhones []string
	for np := range phoneToFb {
		if len(phoneToPos[np]) > 0 {
			mergePhones++
			if len(samplePhones) < 5 {
				samplePhones = append(samplePhones, np)
			}
		}
	}

	fmt.Printf("  FB unique phones (chuẩn hóa): %d\n", len(phoneToFb))
	fmt.Printf("  POS unique phones (chuẩn hóa): %d\n", len(phoneToPos))
	fmt.Printf("  Số phone có trong CẢ FB và POS: %d\n", mergePhones)
	fmt.Printf("  Mẫu phone trùng: %v\n", samplePhones)
	fmt.Printf("  → Có thể merge qua phone: ~%d cặp (1 phone = 1 khách merged)\n", mergePhones)
	fmt.Println()
}

// analyzeConversationLink xem posData.conversation_link chứa gì
func analyzeConversationLink(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 10. posData.conversation_link ---")

	posColl := db.Collection("pc_pos_customers")
	cursor, _ := posColl.Find(ctx, bson.M{"posData.conversation_link": bson.M{"$exists": true, "$ne": ""}}, options.Find().SetLimit(10).SetProjection(bson.M{"customerId": 1, "posData.conversation_link": 1, "posData.fb_id": 1}))
	withConvLink, _ := posColl.CountDocuments(ctx, bson.M{"posData.conversation_link": bson.M{"$exists": true, "$ne": ""}})
	total, _ := posColl.CountDocuments(ctx, bson.M{})

	fmt.Printf("  POS có conversation_link: %d / %d (%.1f%%)\n", withConvLink, total, float64(withConvLink)/float64(max(1, total))*100)
	fmt.Printf("  Mẫu conversation_link: ")
	count := 0
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		if pd, ok := d["posData"].(bson.M); ok {
			cl := getStringFromAny(pd["conversation_link"])
			if count < 3 && cl != "" {
				fmt.Printf("%q ", trunc(cl, 40))
				count++
			}
		}
	}
	cursor.Close(ctx)
	fmt.Println()

	// conversation_link có match fb_conversations.conversationId không?
	convColl := db.Collection("fb_conversations")
	convCursor, _ := convColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"conversationId": 1}))
	convIds := make(map[string]bool)
	for convCursor.Next(ctx) {
		var d bson.M
		convCursor.Decode(&d)
		if id, ok := d["conversationId"].(string); ok && id != "" {
			convIds[id] = true
		}
	}
	convCursor.Close(ctx)

	posCursor2, _ := posColl.Find(ctx, bson.M{"posData.conversation_link": bson.M{"$exists": true, "$ne": ""}}, options.Find().SetProjection(bson.M{"posData.conversation_link": 1}))
	convLinkMatch := 0
	for posCursor2.Next(ctx) {
		var d bson.M
		posCursor2.Decode(&d)
		if pd, ok := d["posData"].(bson.M); ok {
			cl := getStringFromAny(pd["conversation_link"])
			if convIds[cl] {
				convLinkMatch++
			}
		}
	}
	posCursor2.Close(ctx)
	fmt.Printf("  → conversation_link match fb_conversations.conversationId: %d\n", convLinkMatch)
	fmt.Println()
}

// analyzeOrderToCustomerLink orders.customerId (POS) — có thể link qua billPhone tới FB?
func analyzeOrderToCustomerLink(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 11. ORDERS → CUSTOMER LINK (qua billPhone) ---")

	// Map: normalized billPhone -> pos customerId (từ orders)
	orderColl := db.Collection("pc_pos_orders")
	cursor, _ := orderColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"customerId": 1, "billPhoneNumber": 1, "posData.customer.id": 1, "posData.bill_phone_number": 1}))
	orderPhoneToPosId := make(map[string]string) // normalized phone -> pos customerId (lấy từ order gần nhất)
	for cursor.Next(ctx) {
		var d bson.M
		cursor.Decode(&d)
		cid := getStringFromAny(d["customerId"])
		if cid == "" {
			if pd, ok := d["posData"].(bson.M); ok {
				if cust, ok := pd["customer"].(bson.M); ok {
					cid = getStringFromAny(cust["id"])
				}
			}
		}
		phone := getStringFromAny(d["billPhoneNumber"])
		if phone == "" {
			if pd, ok := d["posData"].(bson.M); ok {
				phone = getStringFromAny(pd["bill_phone_number"])
			}
		}
		if cid != "" && phone != "" {
			np := normalizePhone(phone)
			if len(np) >= 9 {
				orderPhoneToPosId[np] = cid
			}
		}
	}
	cursor.Close(ctx)

	// FB phones
	fbColl := db.Collection("fb_customers")
	fbCursor, _ := fbColl.Find(ctx, bson.M{})
	orderPhoneMatchFb := 0
	for fbCursor.Next(ctx) {
		var d bson.M
		fbCursor.Decode(&d)
		phones := extractPhones(d)
		for _, p := range phones {
			np := normalizePhone(p)
			if orderPhoneToPosId[np] != "" {
				orderPhoneMatchFb++
				break
			}
		}
	}
	fbCursor.Close(ctx)

	fmt.Printf("  Orders có billPhone: %d unique phones link tới POS customer\n", len(orderPhoneToPosId))
	fmt.Printf("  FB customers có phone trùng với order.billPhone: %d\n", orderPhoneMatchFb)
	fmt.Printf("  → Có thể link order (POS) → FB qua billPhone: %d khách FB\n", orderPhoneMatchFb)
	fmt.Println()
}

// analyzeMergeSummary tổng hợp tiềm năng merge
func analyzeMergeSummary(ctx context.Context, db *mongo.Database) {
	fmt.Println("--- 12. TỔNG HỢP: TIỀM NĂNG MERGE ---")

	// Đếm merge qua fb_id (format pageId_psid)
	posColl := db.Collection("pc_pos_customers")
	posCursor, _ := posColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"customerId": 1, "posData.fb_id": 1}))
	fbColl := db.Collection("fb_customers")
	fbCursor, _ := fbColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"customerId": 1, "psid": 1, "pageId": 1}))
	fbIds := make(map[string]bool)
	fbPsids := make(map[string]bool)
	fbPageIdPsid := make(map[string]bool)
	for fbCursor.Next(ctx) {
		var d bson.M
		fbCursor.Decode(&d)
		if id, ok := d["customerId"].(string); ok && id != "" {
			fbIds[id] = true
		}
		psid := getStringFromAny(d["psid"])
		pageId := getStringFromAny(d["pageId"])
		if psid != "" {
			fbPsids[psid] = true
			if pageId != "" {
				fbPageIdPsid[pageId+"_"+psid] = true
			}
		}
	}
	fbCursor.Close(ctx)

	mergeByFbId := 0
	for posCursor.Next(ctx) {
		var d bson.M
		posCursor.Decode(&d)
		if pd, ok := d["posData"].(bson.M); ok {
			fbId := getStringFromAny(pd["fb_id"])
			if fbIds[fbId] || fbPsids[fbId] || fbPageIdPsid[fbId] {
				mergeByFbId++
			}
		}
	}
	posCursor.Close(ctx)

	// Đếm merge qua phone
	posColl2 := db.Collection("pc_pos_customers")
	posCur2, _ := posColl2.Find(ctx, bson.M{})
	phoneToPos := make(map[string]string)
	for posCur2.Next(ctx) {
		var d bson.M
		posCur2.Decode(&d)
		cid, _ := d["customerId"].(string)
		phones := extractPhones(d)
		for _, p := range phones {
			np := normalizePhone(p)
			if len(np) >= 9 {
				phoneToPos[np] = cid
			}
		}
	}
	posCur2.Close(ctx)

	fbCur2, _ := fbColl.Find(ctx, bson.M{})
	mergeByPhone := 0
	mergedPosIds := make(map[string]bool)
	for fbCur2.Next(ctx) {
		var d bson.M
		fbCur2.Decode(&d)
		phones := extractPhones(d)
		for _, p := range phones {
			np := normalizePhone(p)
			if posId := phoneToPos[np]; posId != "" {
				mergeByPhone++
				mergedPosIds[posId] = true
				break
			}
		}
	}
	fbCur2.Close(ctx)

	posTotal, _ := posColl.CountDocuments(ctx, bson.M{})
	fbTotal, _ := fbColl.CountDocuments(ctx, bson.M{})

	fmt.Println("  KẾT LUẬN:")
	fmt.Printf("  • Merge qua posData.fb_id: %d khách POS có thể link tới FB\n", mergeByFbId)
	fmt.Printf("  • Merge qua phone: %d khách FB có phone trùng POS\n", mergeByPhone)
	fmt.Printf("  • Tổng POS: %d, Tổng FB: %d\n", posTotal, fbTotal)
	fmt.Println()
	fmt.Println("  ĐỀ XUẤT CHIẾN LƯỢC MERGE:")
	if mergeByFbId > 0 {
		fmt.Printf("  1. Ưu tiên posData.fb_id (chính xác): merge %d khách\n", mergeByFbId)
	}
	if mergeByPhone > 0 {
		fmt.Printf("  2. Fallback phone (chuẩn hóa): merge thêm ~%d khách (trừ trùng fb_id)\n", mergeByPhone)
	}
	if mergeByFbId == 0 && mergeByPhone == 0 {
		fmt.Println("  Chưa có overlap qua fb_id hoặc phone. Cần kiểm tra thêm posData structure.")
	}
	fmt.Println()
}

func getInt64(m bson.M, key string) int64 {
	if v, ok := m[key].(int32); ok {
		return int64(v)
	}
	if v, ok := m[key].(int64); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func getKeys(m bson.M) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
