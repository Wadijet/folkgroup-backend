// Script rà soát liên kết fb_conversations ↔ fb_customers.
// Tìm TẤT CẢ đường link có thể (cùng nguồn FB/Pancake).
//
// Chạy: go run scripts/audit_fb_conv_fb_customer_link.go
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

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return fmt.Sprintf("%.0f", x)
	case int, int64:
		return fmt.Sprintf("%v", x)
	}
	return ""
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

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	convColl := db.Collection("fb_conversations")
	fbCustColl := db.Collection("fb_customers")

	fmt.Println("=== RÀ SOÁT LIÊN KẾT fb_conversations ↔ fb_customers ===\n")
	fmt.Printf("Database: %s\n\n", dbName)

	// 1. Load fb_customers: customerId, panCakeData.customer_id, psid, pageId
	type fbCustDoc struct {
		CustomerId string `bson:"customerId"`
		Psid       string `bson:"psid"`
		PageId     string `bson:"pageId"`
		PanCakeData bson.M `bson:"panCakeData"`
	}

	fbByCustomerId := make(map[string]bool)           // customerId (panCakeData.id)
	fbByPanCakeCustomerId := make(map[string]bool)     // panCakeData.customer_id
	fbByPsidPage := make(map[string]bool)             // pageId_psid (conversationId format)
	var fbList []fbCustDoc

	fbCur, _ := fbCustColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{
		"customerId": 1, "psid": 1, "pageId": 1, "panCakeData.customer_id": 1,
	}))
	for fbCur.Next(ctx) {
		var d fbCustDoc
		if fbCur.Decode(&d) == nil {
			fbList = append(fbList, d)
			if d.CustomerId != "" {
				fbByCustomerId[d.CustomerId] = true
			}
			if d.PanCakeData != nil {
				if cid := str(d.PanCakeData["customer_id"]); cid != "" {
					fbByPanCakeCustomerId[cid] = true
				}
			}
			if d.Psid != "" && d.PageId != "" {
				fbByPsidPage[d.PageId+"_"+d.Psid] = true
			}
		}
	}
	fbCur.Close(ctx)

	fmt.Printf("fb_customers: %d (customerId: %d, panCakeData.customer_id: %d, pageId_psid: %d)\n\n",
		len(fbList), len(fbByCustomerId), len(fbByPanCakeCustomerId), len(fbByPsidPage))

	// 2. Quét fb_conversations, đếm match theo từng đường
	type counts struct {
		byConvCustomerId      int // conv.customerId = fb.customerId
		byConvCustomerIdPanCake int // conv.customerId = fb.panCakeData.customer_id
		byCustomersId          int // conv.customers[].id = fb.customerId
		byPageCustomerId       int // conv.page_customer.id = fb.customerId
		byConversationId       int // conv.conversationId = fb.pageId_psid
		anyMatch               int
	}

	var c counts
	totalConv := 0
	var sampleMatch, sampleNoMatch []string

	convCur, _ := convColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{
		"conversationId": 1, "customerId": 1, "pageId": 1,
		"panCakeData.customer_id": 1, "panCakeData.customers": 1, "panCakeData.page_customer": 1, "panCakeData.from": 1,
	}))

	for convCur.Next(ctx) {
		totalConv++
		var doc bson.M
		if convCur.Decode(&doc) != nil {
			continue
		}

		convId := str(doc["conversationId"])
		custId := str(doc["customerId"])
		pageId := str(doc["pageId"])

		pd, _ := doc["panCakeData"].(bson.M)

		// Lấy customers[].id, page_customer.id
		custIds := []string{}
		if pd != nil {
			if arr, ok := pd["customers"].(bson.A); ok {
				for _, item := range arr {
					if m, ok := item.(bson.M); ok {
						if id := str(m["id"]); id != "" {
							custIds = append(custIds, id)
						}
					}
				}
			}
			if pc, ok := pd["page_customer"].(bson.M); ok {
				if id := str(pc["id"]); id != "" {
					custIds = append(custIds, id)
				}
			}
		}

		// Lấy psid từ from.id hoặc page_customer.psid
		psid := ""
		if pd != nil {
			if from, ok := pd["from"].(bson.M); ok {
				psid = str(from["id"])
			}
			if psid == "" {
				if pc, ok := pd["page_customer"].(bson.M); ok {
					psid = str(pc["psid"])
				}
			}
		}

		matched := false

		// Path 1: conv.customerId = fb.customerId
		if fbByCustomerId[custId] {
			c.byConvCustomerId++
			matched = true
		}
		// Path 2: conv.customerId = fb.panCakeData.customer_id
		if fbByPanCakeCustomerId[custId] {
			c.byConvCustomerIdPanCake++
			matched = true
		}
		// Path 3: conv.customers[].id = fb.customerId
		for _, id := range custIds {
			if fbByCustomerId[id] {
				c.byCustomersId++
				matched = true
				break
			}
		}
		// Path 4: conv.page_customer.id = fb.customerId (đã gộp trong Path 3)
		// Path 5: conv.conversationId = pageId_psid (fb có thread_id = pageId_psid)
		if pageId != "" && psid != "" && fbByPsidPage[pageId+"_"+psid] {
			c.byConversationId++
			matched = true
		}

		if matched {
			c.anyMatch++
			if len(sampleMatch) < 3 {
				sampleMatch = append(sampleMatch, fmt.Sprintf("convId=%s custId=%s customers=%v → match", convId, custId, custIds))
			}
		} else {
			if len(sampleNoMatch) < 5 {
				sampleNoMatch = append(sampleNoMatch, fmt.Sprintf("convId=%s custId=%s customers=%v psid=%s → KHÔNG match", convId, custId, custIds, psid))
			}
		}
	}
	convCur.Close(ctx)

	// 3. In kết quả
	fmt.Println("--- Kết quả match fb_conversations → fb_customers ---")
	fmt.Printf("Tổng fb_conversations: %d\n", totalConv)
	fmt.Printf("Conv có ≥1 đường link: %d (%.1f%%)\n", c.anyMatch, float64(c.anyMatch)/float64(totalConv)*100)
	fmt.Println("\nChi tiết theo đường link:")
	fmt.Printf("  conv.customerId = fb_customers.customerId: %d\n", c.byConvCustomerId)
	fmt.Printf("  conv.customerId = fb_customers.panCakeData.customer_id: %d\n", c.byConvCustomerIdPanCake)
	fmt.Printf("  conv.customers[].id = fb_customers.customerId: %d\n", c.byCustomersId)
	fmt.Printf("  conv.conversationId = fb.pageId_psid (thread_id): %d\n", c.byConversationId)

	if len(sampleMatch) > 0 {
		fmt.Println("\nMẫu conv CÓ match:")
		for _, s := range sampleMatch {
			fmt.Printf("  %s\n", s)
		}
	}
	if len(sampleNoMatch) > 0 {
		fmt.Println("\nMẫu conv KHÔNG match:")
		for _, s := range sampleNoMatch {
			fmt.Printf("  %s\n", s)
		}
	}

	// 4. Build set conversationId từ conv (để kiểm tra ngược)
	convIdsSet := make(map[string]bool)
	convCur2, _ := convColl.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"conversationId": 1}))
	for convCur2.Next(ctx) {
		var d struct {
			ConversationId string `bson:"conversationId"`
		}
		if convCur2.Decode(&d) == nil && d.ConversationId != "" {
			convIdsSet[d.ConversationId] = true
		}
	}
	convCur2.Close(ctx)

	// 5. Kiểm tra ngược: fb_customers có conv không?
	fmt.Println("\n--- Kiểm tra ngược: fb_customers có conversation không? ---")
	fbWithConv := 0
	fbWithoutConv := 0
	for _, d := range fbList {
		hasConv := false
		convKey := d.PageId + "_" + d.Psid
		if convKey != "_" && convIdsSet[convKey] {
			hasConv = true
		}
		if !hasConv && d.CustomerId != "" {
			// Đã có conv match qua customers[].id = d.CustomerId (trong lần quét trước)
			// Cần query: conv có customers[].id = d.CustomerId?
			n, _ := convColl.CountDocuments(ctx, bson.M{
				"$or": []bson.M{
					{"customerId": d.CustomerId},
					{"panCakeData.customer_id": d.CustomerId},
					{"panCakeData.customers.id": d.CustomerId},
					{"panCakeData.page_customer.id": d.CustomerId},
				},
			})
			if n > 0 {
				hasConv = true
			}
		}
		if hasConv {
			fbWithConv++
		} else {
			fbWithoutConv++
		}
	}
	fmt.Printf("fb_customers có ≥1 conv: %d\n", fbWithConv)
	fmt.Printf("fb_customers KHÔNG có conv: %d\n", fbWithoutConv)

	fmt.Println("\n--- Kết luận ---")
	if c.byCustomersId > 0 || c.byConversationId > 0 {
		fmt.Println("✓ Liên kết TỒN TẠI qua: conv.customers[].id = fb.customerId HOẶC conv.conversationId = fb.pageId_psid")
	}
	if c.byConvCustomerId == 0 && c.byConvCustomerIdPanCake == 0 {
		fmt.Println("⚠️ conv.customerId KHÔNG khớp fb.customerId hay fb.panCakeData.customer_id — dùng customers[].id hoặc conversationId")
	}
	fmt.Println("\n✓ Hoàn thành")
}
