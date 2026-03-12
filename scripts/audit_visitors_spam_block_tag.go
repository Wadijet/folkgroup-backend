// Script kiểm tra 691 visitor: có tag spam/block không (khách spam đã xóa chat/comment)?
//
// Chạy: go run scripts/audit_visitors_spam_block_tag.go [ownerOrganizationId]
//
// Kiểm tra:
// 1. crm_customers.conversationTags — tag "spam", "block"
// 2. pc_pos_customers.isBlock — khách POS bị block
// 3. fb_customers — panCakeData.is_block, panCakeData.tags
// 4. fb_conversations có tag spam/block (conv có thể đã xóa — tìm conv còn tồn tại match customer)
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

// hasSpamOrBlockTag kiểm tra tags có chứa "Block", "Spam", "Khách BLOCK" (object.text từ panCakeData.tags).
func hasSpamOrBlockTag(tags []string) bool {
	for _, t := range tags {
		lower := strings.ToLower(strings.TrimSpace(t))
		if lower == "block" || lower == "spam" ||
			strings.Contains(lower, "spam") || strings.Contains(lower, "block") || strings.Contains(lower, "chặn") {
			return true
		}
	}
	return false
}

// getTagsFromPanCakeDataTags đọc panCakeData.tags — mảng có thể chứa null và object.
// Cấu trúc thực tế: tags: [null, { text: "Đã mua", id: 24, color: "...", ... }, { text: "Block" }, ...]
// Mỗi object có field "text" (vd: "Đã mua", "NV11", "Block", "Spam"). Bỏ qua phần tử null.
func getTagsFromPanCakeDataTags(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, x := range arr {
		if s, ok := x.(string); ok && s != "" {
			out = append(out, s)
			continue
		}
		// Object: { text: "Block" } hoặc { name: "Block" }
		m2, ok := x.(map[string]interface{})
		if !ok {
			if bm, ok := x.(bson.M); ok {
				m2 = bm
			} else {
				continue
			}
		}
		if txt, ok := m2["text"].(string); ok && txt != "" {
			out = append(out, txt)
		} else if name, ok := m2["name"].(string); ok && name != "" {
			out = append(out, name)
		}
	}
	return out
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

	var orgID primitive.ObjectID
	if len(os.Args) >= 2 {
		var err error
		orgID, err = primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
	} else {
		log.Fatal("Chạy: go run scripts/audit_visitors_spam_block_tag.go <ownerOrganizationId>")
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
	posColl := db.Collection("pc_pos_customers")
	fbColl := db.Collection("fb_customers")
	convColl := db.Collection("fb_conversations")

	fmt.Println("=== Kiểm tra 691 visitor: có tag spam/block không? ===\n")
	fmt.Printf("Org: %s\n\n", orgID.Hex())

	// 1. crm_customers — conversationTags
	fmt.Println("--- 1. crm_customers.conversationTags (visitor) ---")
	cur, _ := crmColl.Find(ctx, bson.M{"ownerOrganizationId": orgID, "journeyStage": "visitor"},
		options.Find().SetProjection(bson.M{"unifiedId": 1, "conversationTags": 1, "primarySource": 1, "sourceIds": 1}))
	var visitors []bson.M
	cur.All(ctx, &visitors)
	cur.Close(ctx)

	withSpamBlockTag := 0
	var sampleWithTag []string
	for _, doc := range visitors {
		tags, _ := doc["conversationTags"].(bson.A)
		var tagStrs []string
		for _, t := range tags {
			if s, ok := t.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}
		if hasSpamOrBlockTag(tagStrs) {
			withSpamBlockTag++
			if len(sampleWithTag) < 10 {
				sampleWithTag = append(sampleWithTag, fmt.Sprintf("%v (tags=%v)", doc["unifiedId"], tagStrs))
			}
		}
	}
	fmt.Printf("  Visitor có conversationTags chứa spam/block: %d / %d\n", withSpamBlockTag, len(visitors))
	if len(sampleWithTag) > 0 {
		fmt.Printf("  Mẫu: %v\n", sampleWithTag)
	}

	// 2. pc_pos_customers — isBlock (visitor từ POS)
	fmt.Println("\n--- 2. pc_pos_customers.isBlock (visitor primarySource=pos) ---")
	posVisitorIds := make(map[string]bool)
	for _, doc := range visitors {
		if doc["primarySource"] == "pos" {
			sids, _ := doc["sourceIds"].(bson.M)
			if pos, ok := sids["pos"]; ok && pos != nil {
				posVisitorIds[fmt.Sprintf("%v", pos)] = true
			}
		}
	}
	posBlocked := 0
	posIds := keys(posVisitorIds)
	if len(posIds) > 0 {
		cur, _ = posColl.Find(ctx, bson.M{"ownerOrganizationId": orgID, "customerId": bson.M{"$in": posIds}},
			options.Find().SetProjection(bson.M{"customerId": 1, "isBlock": 1, "posData.is_block": 1}))
		for cur.Next(ctx) {
			var d struct {
				CustomerId string `bson:"customerId"`
				IsBlock    bool   `bson:"isBlock"`
				PosData    struct {
					IsBlock interface{} `bson:"is_block"`
				} `bson:"posData"`
			}
			if cur.Decode(&d) == nil && (d.IsBlock || toBool(d.PosData.IsBlock)) {
				posBlocked++
			}
		}
		cur.Close(ctx)
	}
	fmt.Printf("  Visitor POS bị block (isBlock=true): %d / %d\n", posBlocked, len(posVisitorIds))

	// 3. fb_customers — panCakeData (visitor từ FB)
	fmt.Println("\n--- 3. fb_customers (visitor primarySource=fb) — panCakeData.is_block, tags ---")
	fbVisitorIds := make(map[string]bool)
	for _, doc := range visitors {
		if doc["primarySource"] == "fb" {
			sids, _ := doc["sourceIds"].(bson.M)
			if fb, ok := sids["fb"]; ok && fb != nil {
				fbVisitorIds[fmt.Sprintf("%v", fb)] = true
			}
		}
	}
	fbBlocked := 0
	fbSpamTag := 0
	cur, _ = fbColl.Find(ctx, bson.M{"ownerOrganizationId": orgID, "customerId": bson.M{"$in": keys(fbVisitorIds)}},
		options.Find().SetProjection(bson.M{"customerId": 1, "panCakeData": 1}))
	for cur.Next(ctx) {
		var d struct {
			CustomerId   string                 `bson:"customerId"`
			PanCakeData  map[string]interface{} `bson:"panCakeData"`
		}
		if cur.Decode(&d) != nil {
			continue
		}
		if toBool(getFromMap(d.PanCakeData, "is_block")) {
			fbBlocked++
		}
		tags := getTagsFromPanCakeDataTags(d.PanCakeData, "tags")
		if hasSpamOrBlockTag(tags) {
			fbSpamTag++
		}
	}
	cur.Close(ctx)
	fmt.Printf("  Visitor FB bị block (panCakeData.is_block): %d\n", fbBlocked)
	fmt.Printf("  Visitor FB có tag spam/block (panCakeData.tags): %d\n", fbSpamTag)

	// 4. fb_conversations — tìm conv có tag spam/block (panCakeData.tags[].text: Block, Spam, Khách BLOCK)
	fmt.Println("\n--- 4. fb_conversations có tag spam/block (panCakeData.tags[].text = Block/Spam/Khách BLOCK) ---")
	convSpamBlockFilter := bson.M{
		"ownerOrganizationId": orgID,
		"$or": []bson.M{
			{"panCakeData.tags.text": bson.M{"$regex": "block", "$options": "i"}},
			{"panCakeData.tags.text": bson.M{"$regex": "spam", "$options": "i"}},
			{"panCakeData.tags.text": bson.M{"$regex": "chặn", "$options": "i"}},
		},
	}
	totalConv, _ := convColl.CountDocuments(ctx, bson.M{"ownerOrganizationId": orgID})
	totalConvSpamBlock, _ := convColl.CountDocuments(ctx, convSpamBlockFilter)
	fmt.Printf("  Tổng conv trong org: %d\n", totalConv)
	fmt.Printf("  Conv có tag Block/Spam/Chặn: %d\n", totalConvSpamBlock)

	cur, _ = convColl.Find(ctx, convSpamBlockFilter, options.Find().SetProjection(bson.M{"panCakeData.tags": 1, "customerId": 1, "panCakeData.customer_id": 1, "panCakeData.page_customer.customer_id": 1}))
	visitorIdsSet := make(map[string]bool)
	for _, v := range visitors {
		visitorIdsSet[fmt.Sprintf("%v", v["unifiedId"])] = true
		sids, _ := v["sourceIds"].(bson.M)
		if pos, ok := sids["pos"]; ok && pos != nil {
			visitorIdsSet[fmt.Sprintf("%v", pos)] = true
		}
		if fb, ok := sids["fb"]; ok && fb != nil {
			visitorIdsSet[fmt.Sprintf("%v", fb)] = true
		}
	}

	visitorsWithConvSpamBlock := make(map[string]bool)
	var sampleMatch []string
	for cur.Next(ctx) {
		var d struct {
			CustomerId  string                 `bson:"customerId"`
			PanCakeData map[string]interface{} `bson:"panCakeData"`
		}
		if cur.Decode(&d) != nil {
			continue
		}
		custIds := []string{d.CustomerId}
		if pc := d.PanCakeData; pc != nil {
			if cid, ok := pc["customer_id"].(string); ok && cid != "" {
				custIds = append(custIds, cid)
			}
			if pageCust, ok := pc["page_customer"].(map[string]interface{}); ok {
				if cid, ok := pageCust["customer_id"].(string); ok && cid != "" {
					custIds = append(custIds, cid)
				}
			}
		}
		for _, cid := range custIds {
			if visitorIdsSet[cid] {
				visitorsWithConvSpamBlock[cid] = true
				if len(sampleMatch) < 5 {
					sampleMatch = append(sampleMatch, fmt.Sprintf("customerId=%s", cid))
				}
				break
			}
		}
	}
	cur.Close(ctx)
	convSpamBlockMatchVisitor := len(visitorsWithConvSpamBlock)
	fmt.Printf("  Số visitor (unique) có conv tag Block/Spam: %d\n", convSpamBlockMatchVisitor)
	if len(sampleMatch) > 0 {
		fmt.Printf("  Mẫu visitor có conv Block/Spam: %v\n", sampleMatch)
	}


	fmt.Println("\n--- 5. Tổng kết ---")
	total := withSpamBlockTag + posBlocked + fbBlocked + fbSpamTag + convSpamBlockMatchVisitor
	if total > 0 {
		fmt.Printf("  Có %d visitor có dấu hiệu spam/block (tag, isBlock, hoặc conv có tag Block/Spam).\n", total)
		fmt.Printf("    - Conv có tag Block/Spam/Khách BLOCK match visitor: %d\n", convSpamBlockMatchVisitor)
		fmt.Println("  → Có thể đây là khách spam đã bị tag Block/Spam, hoặc bị block.")
	} else {
		fmt.Println("  Không tìm thấy visitor nào có tag spam/block hoặc isBlock.")
		fmt.Println("  → 691 visitor có thể là khách thật chưa có hội thoại, hoặc conv đã xóa hoàn toàn.")
	}

	fmt.Println("\n✓ Hoàn thành")
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func unique(ss []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range ss {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func toBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.ToLower(x) == "true" || x == "1"
	case float64:
		return x != 0
	case int:
		return x != 0
	case int64:
		return x != 0
	}
	return false
}

func getFromMap(m map[string]interface{}, key string) interface{} {
	if m == nil {
		return nil
	}
	return m[key]
}
