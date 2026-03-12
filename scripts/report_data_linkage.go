// Script tạo báo cáo rà soát việc link dữ liệu giữa các collection.
// Kiểm tra: orders↔conversations, orders↔customers, conversations↔messages, ads↔meta, ...
//
// Chạy: cd api && go run ../scripts/report_data_linkage.go
// Output: scripts/reports/BAO_CAO_RASOAT_LIEN_KET_DU_LIEU_YYYYMMDD.md
package main

import (
	"context"
	"fmt"
	"log"
	"meta_commerce/config"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(cfg.MongoDB_DBName_Auth)

	var sb strings.Builder
	now := time.Now().Format("2006-01-02 15:04")
	reportDate := time.Now().Format("20060102")

	// Header
	sb.WriteString("# BÁO CÁO RÀ SOÁT LIÊN KẾT DỮ LIỆU\n\n")
	sb.WriteString(fmt.Sprintf("**Ngày tạo:** %s  \n**Database:** %s\n\n", now, cfg.MongoDB_DBName_Auth))
	sb.WriteString("---\n\n")

	// 1. Tổng quan số lượng
	writeOverviewSection(&sb, db, ctx)

	// 2. Liên kết pc_pos_orders
	writeOrdersLinkageSection(&sb, db, ctx)

	// 3. Liên kết fb_conversations
	writeConversationsLinkageSection(&sb, db, ctx)

	// 4. Liên kết Meta Ads
	writeMetaAdsLinkageSection(&sb, db, ctx)

	// 5. Liên kết CRM & Customers
	writeCrmLinkageSection(&sb, db, ctx)

	// 6. Chi tiết từng loại liên kết
	writeDetailChecksSection(&sb, db, ctx)

	// Ghi file
	reportDir := filepath.Join("..", "scripts", "reports")
	if wd, _ := os.Getwd(); !strings.Contains(wd, "api") {
		reportDir = filepath.Join("scripts", "reports")
	}
	_ = os.MkdirAll(reportDir, 0755)
	outPath := filepath.Join(reportDir, fmt.Sprintf("BAO_CAO_RASOAT_LIEN_KET_DU_LIEU_%s.md", reportDate))
	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		log.Fatalf("Ghi file: %v", err)
	}
	fmt.Printf("✅ Đã tạo báo cáo: %s\n", outPath)
}

func writeOverviewSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 1. TỔNG QUAN SỐ LƯỢNG\n\n")
	cols := []struct {
		name string
		col  string
	}{
		{"pc_pos_orders", "pc_pos_orders"},
		{"pc_pos_customers", "pc_pos_customers"},
		{"fb_customers", "fb_customers"},
		{"fb_conversations", "fb_conversations"},
		{"fb_message_items", "fb_message_items"},
		{"fb_pages", "fb_pages"},
		{"fb_posts", "fb_posts"},
		{"meta_ads", "meta_ads"},
		{"crm_customers", "crm_customers"},
	}
	sb.WriteString("| Collection | Số bản ghi |\n|------------|------------|\n")
	for _, c := range cols {
		n, _ := db.Collection(c.col).CountDocuments(ctx, bson.M{})
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", c.name, n))
	}
	sb.WriteString("\n")
}

func writeOrdersLinkageSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 2. LIÊN KẾT PC_POS_ORDERS (posData)\n\n")
	orders := db.Collection("pc_pos_orders")
	total, _ := orders.CountDocuments(ctx, bson.M{})
	withConvId, _ := orders.CountDocuments(ctx, bson.M{"$or": []bson.M{
		{"posData.conversation_id": bson.M{"$exists": true, "$ne": ""}},
		{"posData.conversationId": bson.M{"$exists": true, "$ne": ""}},
		{"posData.conversation_link": bson.M{"$exists": true, "$ne": ""}},
	}})
	withCustomerId, _ := orders.CountDocuments(ctx, bson.M{"$or": []bson.M{
		{"customerId": bson.M{"$exists": true, "$ne": ""}},
		{"posData.customer.id": bson.M{"$exists": true, "$ne": ""}},
		{"posData.customer_id": bson.M{"$exists": true, "$ne": ""}},
	}})
	withAdId, _ := orders.CountDocuments(ctx, bson.M{"posData.ad_id": bson.M{"$exists": true, "$ne": ""}})
	withPostId, _ := orders.CountDocuments(ctx, bson.M{"posData.post_id": bson.M{"$exists": true, "$ne": ""}})
	withPageId, _ := orders.CountDocuments(ctx, bson.M{"posData.page_id": bson.M{"$exists": true, "$ne": ""}})

	sb.WriteString("| Trường | Số đơn có | Tỷ lệ |\n|--------|-----------|-------|\n")
	writePct := func(n int64, label string) {
		pct := "—"
		if total > 0 {
			pct = fmt.Sprintf("%.1f%%", float64(n)/float64(total)*100)
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %s |\n", label, n, pct))
	}
	writePct(withConvId, "conversation_id / conversationId / conversation_link")
	writePct(withCustomerId, "customerId / posData.customer.id / customer_id")
	writePct(withAdId, "posData.ad_id")
	writePct(withPostId, "posData.post_id")
	writePct(withPageId, "posData.page_id")
	sb.WriteString(fmt.Sprintf("\n*Tổng đơn hàng: %d*\n\n", total))
}

func writeConversationsLinkageSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 3. LIÊN KẾT FB_CONVERSATIONS\n\n")
	convs := db.Collection("fb_conversations")
	msgItems := db.Collection("fb_message_items")
	msgs := db.Collection("fb_messages")

	totalConv, _ := convs.CountDocuments(ctx, bson.M{})
	convWithCustomerId, _ := convs.CountDocuments(ctx, bson.M{"customerId": bson.M{"$exists": true, "$ne": ""}})
	totalMsgItems, _ := msgItems.CountDocuments(ctx, bson.M{})
	totalMsgs, _ := msgs.CountDocuments(ctx, bson.M{})

	// Conv có message_items
	pipe := []bson.M{
		{"$group": bson.M{"_id": "$conversationId", "cnt": bson.M{"$sum": 1}}},
		{"$count": "n"},
	}
	var convWithMsgResult struct{ N int `bson:"n"` }
	if cur, err := msgItems.Aggregate(ctx, pipe); err == nil && cur.Next(ctx) {
		_ = cur.Decode(&convWithMsgResult)
		cur.Close(ctx)
	}

	sb.WriteString("| Chỉ số | Giá trị |\n|--------|--------|\n")
	sb.WriteString(fmt.Sprintf("| Tổng conversations | %d |\n", totalConv))
	sb.WriteString(fmt.Sprintf("| Conv có customerId | %d |\n", convWithCustomerId))
	sb.WriteString(fmt.Sprintf("| Tổng fb_message_items | %d |\n", totalMsgItems))
	sb.WriteString(fmt.Sprintf("| Tổng fb_messages | %d |\n", totalMsgs))
	sb.WriteString(fmt.Sprintf("| Số conv có ≥1 message (từ message_items) | %d |\n", convWithMsgResult.N))
	sb.WriteString("\n")
}

func writeMetaAdsLinkageSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 4. LIÊN KẾT META ADS\n\n")
	ads := db.Collection("meta_ads")
	campaigns := db.Collection("meta_campaigns")
	adsets := db.Collection("meta_adsets")
	accounts := db.Collection("meta_ad_accounts")
	insights := db.Collection("meta_ad_insights")

	adCount, _ := ads.CountDocuments(ctx, bson.M{})
	campCount, _ := campaigns.CountDocuments(ctx, bson.M{})
	adSetCount, _ := adsets.CountDocuments(ctx, bson.M{})
	accCount, _ := accounts.CountDocuments(ctx, bson.M{})
	insightCount, _ := insights.CountDocuments(ctx, bson.M{})

	sb.WriteString("| Collection | Số bản ghi |\n|------------|------------|\n")
	sb.WriteString(fmt.Sprintf("| meta_ads | %d |\n", adCount))
	sb.WriteString(fmt.Sprintf("| meta_campaigns | %d |\n", campCount))
	sb.WriteString(fmt.Sprintf("| meta_adsets | %d |\n", adSetCount))
	sb.WriteString(fmt.Sprintf("| meta_ad_accounts | %d |\n", accCount))
	sb.WriteString(fmt.Sprintf("| meta_ad_insights | %d |\n", insightCount))
	sb.WriteString("\n")
}

func writeCrmLinkageSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 5. LIÊN KẾT CRM & CUSTOMERS (fb_customers, pc_pos_customers, crm_customers)\n\n")
	crm := db.Collection("crm_customers")
	posCust := db.Collection("pc_pos_customers")
	fbCust := db.Collection("fb_customers")
	convs := db.Collection("fb_conversations")

	crmTotal, _ := crm.CountDocuments(ctx, bson.M{})
	crmWithPos, _ := crm.CountDocuments(ctx, bson.M{"sourceIds.pos": bson.M{"$exists": true, "$ne": ""}})
	crmWithFb, _ := crm.CountDocuments(ctx, bson.M{"sourceIds.fb": bson.M{"$exists": true, "$ne": ""}})
	// Merge: crm có CẢ pos VÀ fb (trường hợp chung — đã merge)
	crmWithBoth, _ := crm.CountDocuments(ctx, bson.M{
		"$and": []bson.M{
			{"sourceIds.pos": bson.M{"$exists": true, "$ne": ""}},
			{"sourceIds.fb": bson.M{"$exists": true, "$ne": ""}},
		},
	})
	// Chỉ POS = có pos, không có fb
	crmPosOnly := crmWithPos - crmWithBoth
	if crmPosOnly < 0 {
		crmPosOnly = 0
	}
	// Chỉ FB = có fb, không có pos
	crmFbOnly := crmWithFb - crmWithBoth
	if crmFbOnly < 0 {
		crmFbOnly = 0
	}

	posCustTotal, _ := posCust.CountDocuments(ctx, bson.M{})
	fbCustTotal, _ := fbCust.CountDocuments(ctx, bson.M{})
	fbCustWithPageId, _ := fbCust.CountDocuments(ctx, bson.M{"pageId": bson.M{"$exists": true, "$ne": ""}})
	convWithCustId, _ := convs.CountDocuments(ctx, bson.M{"customerId": bson.M{"$exists": true, "$ne": ""}})

	sb.WriteString("| Chỉ số | Giá trị |\n|--------|--------|\n")
	sb.WriteString(fmt.Sprintf("| fb_customers | %d |\n", fbCustTotal))
	sb.WriteString(fmt.Sprintf("| fb_customers có pageId | %d |\n", fbCustWithPageId))
	sb.WriteString(fmt.Sprintf("| pc_pos_customers | %d |\n", posCustTotal))
	sb.WriteString(fmt.Sprintf("| crm_customers | %d |\n", crmTotal))
	sb.WriteString(fmt.Sprintf("| crm có sourceIds.pos | %d |\n", crmWithPos))
	sb.WriteString(fmt.Sprintf("| crm có sourceIds.fb | %d |\n", crmWithFb))
	sb.WriteString(fmt.Sprintf("| fb_conversations có customerId | %d |\n", convWithCustId))
	sb.WriteString("\n")

	// Phân tích merge: crm từ nguồn nào, có bao nhiêu trường hợp chung (FB + POS)
	sb.WriteString("### 5.1 CRM merge từ fb_customers + pc_pos_customers\n\n")
	sb.WriteString("crm_customers được merge từ fb_customers và/hoặc pc_pos_customers qua sourceIds.pos và sourceIds.fb.\n\n")
	sb.WriteString("| Phân loại | Số lượng | Mô tả |\n|------------|-----------|-------|\n")
	sb.WriteString(fmt.Sprintf("| **Chỉ POS** (sourceIds.pos có, fb không) | %d | Merge từ pc_pos_customers |\n", crmPosOnly))
	sb.WriteString(fmt.Sprintf("| **Chỉ FB** (sourceIds.fb có, pos không) | %d | Merge từ fb_customers |\n", crmFbOnly))
	sb.WriteString(fmt.Sprintf("| **Chung (FB + POS)** | %d | Đã merge cả hai nguồn — 1 khách = 1 crm |\n", crmWithBoth))
	sb.WriteString("| *Tổng crm có pos* | " + fmt.Sprintf("%d", crmWithPos) + " | = Chỉ POS + Chung |\n")
	sb.WriteString("| *Tổng crm có fb* | " + fmt.Sprintf("%d", crmWithFb) + " | = Chỉ FB + Chung |\n")
	sb.WriteString("\n")
}

func writeDetailChecksSection(sb *strings.Builder, db *mongo.Database, ctx context.Context) {
	sb.WriteString("## 6. CHI TIẾT RÀ SOÁT TỪNG LIÊN KẾT\n\n")

	// 6.1 Orders posData → các collection đích
	orders := db.Collection("pc_pos_orders")
	pages := db.Collection("fb_pages")
	posts := db.Collection("fb_posts")
	convs := db.Collection("fb_conversations")
	ads := db.Collection("meta_ads")
	posCust := db.Collection("pc_pos_customers")
	fbCust := db.Collection("fb_customers")

	cur, _ := orders.Find(ctx, bson.M{"$or": []bson.M{
		{"posData.ad_id": bson.M{"$exists": true, "$ne": ""}},
		{"posData.post_id": bson.M{"$exists": true, "$ne": ""}},
		{"posData.conversation_id": bson.M{"$exists": true, "$ne": ""}},
		{"posData.conversationId": bson.M{"$exists": true, "$ne": ""}},
		{"posData.conversation_link": bson.M{"$exists": true, "$ne": ""}},
		{"posData.page_id": bson.M{"$exists": true, "$ne": ""}},
	}}, options.Find().SetLimit(200))
	defer cur.Close(ctx)

	var orderList []bson.M
	_ = cur.All(ctx, &orderList)

	adMatch, adTotal := 0, 0
	postMatch, postTotal := 0, 0
	convMatch, convTotal := 0, 0
	pageMatch, pageTotal := 0, 0
	posCustMatch, posCustTotal := 0, 0
	fbCustMatch, fbCustTotal := 0, 0

	for _, o := range orderList {
		pos, _ := o["posData"].(bson.M)
		// ad_id → meta_ads
		if aid := getStr(pos, "ad_id"); aid != "" {
			adTotal++
			var ad bson.M
			if ads.FindOne(ctx, bson.M{"$or": []bson.M{{"adId": aid}, {"metaData.id": aid}}}).Decode(&ad) == nil {
				adMatch++
			}
		}
		// post_id → fb_posts
		if pid := getStr(pos, "post_id"); pid != "" {
			postTotal++
			var post bson.M
			if posts.FindOne(ctx, bson.M{"$or": []bson.M{
				{"panCakeData.id": pid}, {"panCakeData.post_id": pid}, {"postId": pid},
			}}).Decode(&post) == nil {
				postMatch++
			}
		}
		// conversation_id → fb_conversations
		cid := getStr(pos, "conversation_id")
		if cid == "" {
			cid = getStr(pos, "conversationId")
		}
		if cid == "" {
			cid = getStr(pos, "conversation_link")
		}
		if cid != "" {
			convTotal++
			var conv bson.M
			if convs.FindOne(ctx, bson.M{"$or": []bson.M{
				{"conversationId": cid},
				{"panCakeData.id": cid},
			}}).Decode(&conv) == nil {
				convMatch++
			}
		}
		// page_id → fb_pages
		if pageId := getStr(pos, "page_id"); pageId != "" {
			pageTotal++
			var page bson.M
			if pages.FindOne(ctx, bson.M{"$or": []bson.M{{"pageId": pageId}, {"panCakeData.id": pageId}}}).Decode(&page) == nil {
				pageMatch++
			}
		}
		// customerId → pc_pos_customers
		custId := getStr(o, "customerId")
		if custId == "" && pos != nil {
			custId = getStr(pos, "customer_id")
			if custId == "" {
				if c, ok := pos["customer"].(bson.M); ok {
					custId = getStr(c, "id")
				}
			}
		}
		if custId != "" {
			posCustTotal++
			var cust bson.M
			if posCust.FindOne(ctx, bson.M{"$or": []bson.M{
				{"customerId": custId}, {"posData.customer.id": custId}, {"id": custId},
			}}).Decode(&cust) == nil {
				posCustMatch++
			}
			fbCustTotal++
			var fb bson.M
			if fbCust.FindOne(ctx, bson.M{"customerId": custId}).Decode(&fb) == nil {
				fbCustMatch++
			}
		}
	}

	sb.WriteString("### 6.1 pc_pos_orders.posData → collection đích (mẫu 200 đơn)\n\n")
	sb.WriteString("| Liên kết | Có trong order | Khớp đích | Tỷ lệ khớp |\n|----------|----------------|------------|-------------|\n")
	writeRow := func(label string, total, match int) {
		pct := "—"
		if total > 0 {
			pct = fmt.Sprintf("%.1f%%", float64(match)/float64(total)*100)
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %s |\n", label, total, match, pct))
	}
	writeRow("posData.ad_id → meta_ads", adTotal, adMatch)
	writeRow("posData.post_id → fb_posts", postTotal, postMatch)
	writeRow("posData.conversation_id → fb_conversations", convTotal, convMatch)
	writeRow("posData.page_id → fb_pages", pageTotal, pageMatch)
	writeRow("customerId / posData.customer → pc_pos_customers", posCustTotal, posCustMatch)
	writeRow("customerId / posData.customer → fb_customers", fbCustTotal, fbCustMatch)
	sb.WriteString("\n")

	// Lấy mẫu conversations cho 6.2 và 6.3
	convCur, _ := convs.Find(ctx, bson.M{"conversationId": bson.M{"$exists": true, "$ne": ""}}, options.Find().SetLimit(100))
	defer convCur.Close(ctx)
	var convList []bson.M
	_ = convCur.All(ctx, &convList)

	// 6.2 fb_conversations.customerId → fb_customers
	sb.WriteString("### 6.2 fb_conversations.customerId → fb_customers\n\n")
	convCustMatch, convCustTotal := 0, 0
	for _, c := range convList {
		cid, ok := c["customerId"].(string)
		if !ok || cid == "" {
			continue
		}
		convCustTotal++
		var fb bson.M
		if fbCust.FindOne(ctx, bson.M{"customerId": cid}).Decode(&fb) == nil {
			convCustMatch++
		}
	}
	sb.WriteString(fmt.Sprintf("Mẫu %d conversations có customerId: %d khớp fb_customers (%.1f%%)\n\n",
		convCustTotal, convCustMatch, float64(convCustMatch)/float64(max(convCustTotal, 1))*100))

	// 6.3 fb_conversations → fb_message_items
	sb.WriteString("### 6.3 fb_conversations.conversationId → fb_message_items\n\n")
	msgItems := db.Collection("fb_message_items")
	convWithMsg, convCheckTotal := 0, 0
	for _, c := range convList {
		cid, ok := c["conversationId"].(string)
		if !ok || cid == "" {
			continue
		}
		convCheckTotal++
		n, _ := msgItems.CountDocuments(ctx, bson.M{"conversationId": cid})
		if n > 0 {
			convWithMsg++
		}
	}
	sb.WriteString(fmt.Sprintf("Mẫu %d conversations: %d có ≥1 message trong fb_message_items (%.1f%%)\n\n",
		convCheckTotal, convWithMsg, float64(convWithMsg)/float64(max(convCheckTotal, 1))*100))

	// 6.4 fb_pages.shop_id → pc_pos_shops
	sb.WriteString("### 6.4 fb_pages.shop_id → pc_pos_shops\n\n")
	pagesColl := db.Collection("fb_pages")
	shops := db.Collection("pc_pos_shops")
	var page bson.M
	if err := pagesColl.FindOne(ctx, bson.M{"panCakeData.shop_id": bson.M{"$exists": true, "$ne": nil}}).Decode(&page); err != nil {
		sb.WriteString("⚠️ Không có fb_pages với shop_id\n\n")
	} else {
		shopID, ok := getInt64(page, "panCakeData", "shop_id")
		if !ok {
			sb.WriteString("⚠️ shop_id không parse được\n\n")
		} else {
			var shop bson.M
			if shops.FindOne(ctx, bson.M{"$or": []bson.M{
				{"shopId": shopID}, {"posData.shop_id": shopID}, {"panCakeData.shop_id": shopID}, {"id": shopID},
			}}).Decode(&shop) == nil {
				sb.WriteString(fmt.Sprintf("✅ fb_pages.shop_id (%d) khớp pc_pos_shops\n\n", shopID))
			} else {
				sb.WriteString(fmt.Sprintf("❌ fb_pages.shop_id (%d) không tìm thấy trong pc_pos_shops\n\n", shopID))
			}
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

func getInt64(m bson.M, keys ...string) (int64, bool) {
	var cur interface{} = m
	for _, k := range keys {
		mm, ok := cur.(bson.M)
		if !ok {
			return 0, false
		}
		cur = mm[k]
	}
	switch v := cur.(type) {
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	}
	return 0, false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
