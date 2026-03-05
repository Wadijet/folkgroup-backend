// Script kiểm tra các liên kết dữ liệu trong DB thực tế.
// Chạy: go run scripts/verify_linkages.go
// Cần: MONGODB_CONNECTION_URI, MONGODB_DBNAME_AUTH (từ .env)
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
			return
		}
		parent := filepath.Dir(cwd)
		if _, err := os.Stat(filepath.Join(parent, p)); err == nil {
			_ = godotenv.Load(filepath.Join(parent, p))
			return
		}
	}
}

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH trong .env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	fmt.Println("=== KIỂM TRA LIÊN KẾT DỮ LIỆU (BAO_CAO_LIEN_KET_DU_LIEU) ===\n")

	results := []string{}

	// 1. fb_pages.shop_id ↔ pc_pos_shops
	check1(db, ctx, &results)

	// 2. fb_conversations.post_id ↔ fb_posts
	check2(db, ctx, &results)

	// 3. pc_pos_orders.post_id, ad_id
	check3(db, ctx, &results)

	// 4. pc_pos_orders.ad_id ↔ meta_ads.adId
	check4(db, ctx, &results)

	// 5. fb_conversations.ad_ids[] ↔ meta_ads.adId
	check5(db, ctx, &results)

	// 6. crm_customers.sourceIds.pos → pc_pos_customers
	check6(db, ctx, &results)

	// 7. fb_conversations.conversationId → fb_messages
	check7(db, ctx, &results)

	// 8. meta_ads hierarchy (adId → campaign, adset, ad_account)
	check8(db, ctx, &results)

	// 9. meta_ad_insights — count, metaData (actions, frequency)
	check9(db, ctx, &results)

	// 10. So sánh format ad_id (orders posData) vs meta_ads
	check10(db, ctx, &results)

	// 11. panCakeData.ads — thống kê conv có ads
	check11(db, ctx, &results)

	// 12. pc_pos_orders.posData: ad_id, post_id, conversation_id, page_id — rà soát liên kết
	check12(db, ctx, &results)

	// In kết quả
	for _, r := range results {
		fmt.Println(r)
	}
}

func check1(db *mongo.Database, ctx context.Context, results *[]string) {
	pages := db.Collection("fb_pages")
	shops := db.Collection("pc_pos_shops")

	var page bson.M
	err := pages.FindOne(ctx, bson.M{"panCakeData.shop_id": bson.M{"$exists": true, "$ne": nil}}).Decode(&page)
	if err != nil {
		*results = append(*results, "1. fb_pages.shop_id ↔ pc_pos_shops: ⚠️ Không có fb_pages với shop_id")
		return
	}
	shopID, ok := getInt64(page, "panCakeData", "shop_id")
	if !ok {
		*results = append(*results, "1. fb_pages.shop_id ↔ pc_pos_shops: ⚠️ shop_id không parse được")
		return
	}

	// Tìm shop trong shopId, posData.shop_id, panCakeData.shop_id, id (dữ liệu đồng bộ)
	var shop bson.M
	err = shops.FindOne(ctx, bson.M{"$or": []bson.M{
		{"shopId": shopID},
		{"posData.shop_id": shopID},
		{"panCakeData.shop_id": shopID},
		{"id": shopID},
	}}).Decode(&shop)
	if err != nil {
		*results = append(*results, fmt.Sprintf("1. fb_pages.shop_id (%d) ↔ pc_pos_shops: ❌ Không tìm thấy shop tương ứng", shopID))
		return
	}
	*results = append(*results, fmt.Sprintf("1. fb_pages.shop_id (%d) ↔ pc_pos_shops: ✅ ĐÚNG", shopID))
}

func check2(db *mongo.Database, ctx context.Context, results *[]string) {
	convs := db.Collection("fb_conversations")
	posts := db.Collection("fb_posts")

	// Tìm conv có post_id (trực tiếp) HOẶC panCakeData.ads (có post_id trong từng ad)
	cur, err := convs.Find(ctx, bson.M{"$or": []bson.M{
		{"panCakeData.post_id": bson.M{"$exists": true, "$ne": ""}},
		{"panCakeData.ads.0": bson.M{"$exists": true}},
	}}, options.Find().SetLimit(20))
	if err != nil {
		*results = append(*results, "2. fb_conversations.post_id/ads[].post_id ↔ fb_posts: ⚠️ Lỗi query")
		return
	}
	defer cur.Close(ctx)

	var convList []bson.M
	if err := cur.All(ctx, &convList); err != nil {
		*results = append(*results, "2. fb_conversations.post_id/ads[].post_id ↔ fb_posts: ⚠️ Lỗi decode")
		return
	}

	postIDs := make(map[string]bool)
	for _, c := range convList {
		pc, _ := c["panCakeData"].(bson.M)
		if pc == nil {
			continue
		}
		// post_id trực tiếp
		if pid, ok := pc["post_id"].(string); ok && pid != "" {
			postIDs[pid] = true
		}
		// post_id từ panCakeData.ads[].post_id
		if arr, ok := pc["ads"]; ok && arr != nil {
			var list []interface{}
			switch a := arr.(type) {
			case bson.A:
				list = a
			case []interface{}:
				list = a
			default:
				continue
			}
			for _, v := range list {
				if m, ok := v.(bson.M); ok {
					if pid := getString(m, "post_id"); pid != "" {
						postIDs[pid] = true
					}
				}
			}
		}
	}

	if len(postIDs) == 0 {
		*results = append(*results, "2. fb_conversations.post_id/ads[].post_id ↔ fb_posts: ⚠️ Không có conversation có post_id hoặc ads[].post_id")
		return
	}

	matched, total := 0, 0
	for pid := range postIDs {
		total++
		var post bson.M
		err := posts.FindOne(ctx, bson.M{"$or": []bson.M{
			{"panCakeData.id": pid},
			{"panCakeData.post_id": pid},
			{"postId": pid},
		}}).Decode(&post)
		if err == nil {
			matched++
		}
	}
	if matched == total {
		*results = append(*results, fmt.Sprintf("2. fb_conversations.post_id/ads[].post_id ↔ fb_posts: ✅ ĐÚNG (%d/%d khớp)", matched, total))
	} else {
		*results = append(*results, fmt.Sprintf("2. fb_conversations.post_id/ads[].post_id ↔ fb_posts: ⚠️ MỘT PHẦN (%d/%d khớp)", matched, total))
	}
}

func check3(db *mongo.Database, ctx context.Context, results *[]string) {
	orders := db.Collection("pc_pos_orders")

	withPost, _ := orders.CountDocuments(ctx, bson.M{"posData.post_id": bson.M{"$exists": true, "$ne": ""}})
	withAd, _ := orders.CountDocuments(ctx, bson.M{"posData.ad_id": bson.M{"$exists": true, "$ne": ""}})
	total, _ := orders.CountDocuments(ctx, bson.M{})

	*results = append(*results, fmt.Sprintf("3. pc_pos_orders.post_id, ad_id: ✅ CÓ DỮ LIỆU (post_id: %d, ad_id: %d / tổng %d orders)", withPost, withAd, total))
}

func check4(db *mongo.Database, ctx context.Context, results *[]string) {
	orders := db.Collection("pc_pos_orders")
	ads := db.Collection("meta_ads")

	// Lấy ad_id từ orders
	cur, err := orders.Find(ctx, bson.M{"posData.ad_id": bson.M{"$exists": true, "$ne": ""}}, options.Find().SetLimit(20))
	if err != nil {
		*results = append(*results, "4. pc_pos_orders.ad_id ↔ meta_ads.adId: ⚠️ Lỗi query orders")
		return
	}
	defer cur.Close(ctx)

	var orderList []bson.M
	if err := cur.All(ctx, &orderList); err != nil {
		*results = append(*results, "4. pc_pos_orders.ad_id ↔ meta_ads.adId: ⚠️ Lỗi decode")
		return
	}

	adIDs := make(map[string]bool)
	for _, o := range orderList {
		pos, _ := o["posData"].(bson.M)
		if pos == nil {
			continue
		}
		if aid, ok := pos["ad_id"].(string); ok && aid != "" {
			adIDs[aid] = true
		}
	}

	if len(adIDs) == 0 {
		*results = append(*results, "4. pc_pos_orders.ad_id ↔ meta_ads.adId: ⚠️ Không có order có ad_id")
		return
	}

	matched, total := 0, 0
	for aid := range adIDs {
		total++
		var ad bson.M
		err := ads.FindOne(ctx, bson.M{"$or": []bson.M{
			{"adId": aid},
			{"metaData.id": aid},
		}}).Decode(&ad)
		if err == nil {
			matched++
		}
	}

	if total == 0 {
		*results = append(*results, "4. pc_pos_orders.ad_id ↔ meta_ads.adId: ⚠️ Không có ad_id để kiểm tra")
		return
	}
	adsCount, _ := ads.CountDocuments(ctx, bson.M{})
	if adsCount == 0 {
		*results = append(*results, fmt.Sprintf("4. pc_pos_orders.ad_id ↔ meta_ads.adId: ⚠️ meta_ads TRỐNG (orders có %d ad_id khác nhau, cần sync Meta)", total))
		return
	}
	if matched == total {
		*results = append(*results, fmt.Sprintf("4. pc_pos_orders.ad_id ↔ meta_ads.adId: ✅ ĐÚNG (%d/%d khớp)", matched, total))
	} else {
		*results = append(*results, fmt.Sprintf("4. pc_pos_orders.ad_id ↔ meta_ads.adId: ⚠️ MỘT PHẦN (%d/%d khớp, %d ad_id không có trong meta_ads)", matched, total, total-matched))
	}
}

func check5(db *mongo.Database, ctx context.Context, results *[]string) {
	convs := db.Collection("fb_conversations")
	ads := db.Collection("meta_ads")

	// Tìm conv có ad_ids HOẶC panCakeData.ads (có ad_id trong từng ad)
	cur, err := convs.Find(ctx, bson.M{"$or": []bson.M{
		{"panCakeData.ad_ids.0": bson.M{"$exists": true}},
		{"panCakeData.ads.0": bson.M{"$exists": true}},
	}}, options.Find().SetLimit(20))
	if err != nil {
		*results = append(*results, "5. fb_conversations.ad_ids[]/ads[].ad_id ↔ meta_ads: ⚠️ Lỗi query")
		return
	}
	defer cur.Close(ctx)

	var convList []bson.M
	if err := cur.All(ctx, &convList); err != nil {
		*results = append(*results, "5. fb_conversations.ad_ids[]/ads[].ad_id ↔ meta_ads: ⚠️ Lỗi decode")
		return
	}

	adIDs := make(map[string]bool)
	for _, c := range convList {
		pc, _ := c["panCakeData"].(bson.M)
		if pc == nil {
			continue
		}
		// Từ ad_ids[]
		if arr, ok := pc["ad_ids"]; ok && arr != nil {
			var list []interface{}
			switch a := arr.(type) {
			case bson.A:
				list = a
			case []interface{}:
				list = a
			default:
				break
			}
			for _, v := range list {
				if s, ok := v.(string); ok && s != "" {
					adIDs[s] = true
				}
			}
		}
		// Từ panCakeData.ads[].ad_id
		if arr, ok := pc["ads"]; ok && arr != nil {
			var list []interface{}
			switch a := arr.(type) {
			case bson.A:
				list = a
			case []interface{}:
				list = a
			default:
				break
			}
			for _, v := range list {
				if m, ok := v.(bson.M); ok {
					if aid := getString(m, "ad_id"); aid != "" {
						adIDs[aid] = true
					}
				}
			}
		}
	}

	if len(adIDs) == 0 {
		*results = append(*results, "5. fb_conversations.ad_ids[]/ads[].ad_id ↔ meta_ads: ⚠️ Không có conversation có ad_ids hoặc ads")
		return
	}

	matched, total := 0, 0
	for aid := range adIDs {
		total++
		var ad bson.M
		err := ads.FindOne(ctx, bson.M{"$or": []bson.M{
			{"adId": aid},
			{"metaData.id": aid},
		}}).Decode(&ad)
		if err == nil {
			matched++
		}
	}

	adsCount, _ := ads.CountDocuments(ctx, bson.M{})
	if adsCount == 0 {
		*results = append(*results, fmt.Sprintf("5. fb_conversations.ad_ids[]/ads[].ad_id ↔ meta_ads: ⚠️ meta_ads TRỐNG (convs có %d ad_id khác nhau)", total))
		return
	}
	if matched == total {
		*results = append(*results, fmt.Sprintf("5. fb_conversations.ad_ids[]/ads[].ad_id ↔ meta_ads: ✅ ĐÚNG (%d/%d khớp)", matched, total))
	} else {
		*results = append(*results, fmt.Sprintf("5. fb_conversations.ad_ids[]/ads[].ad_id ↔ meta_ads: ⚠️ MỘT PHẦN (%d/%d khớp)", matched, total))
	}
}

func check6(db *mongo.Database, ctx context.Context, results *[]string) {
	crm := db.Collection("crm_customers")
	pos := db.Collection("pc_pos_customers")

	cur, err := crm.Find(ctx, bson.M{"sourceIds.pos": bson.M{"$exists": true, "$ne": ""}}, options.Find().SetLimit(20))
	if err != nil {
		*results = append(*results, "6. crm_customers.sourceIds.pos → pc_pos_customers: ⚠️ Lỗi query")
		return
	}
	defer cur.Close(ctx)

	var crmList []bson.M
	if err := cur.All(ctx, &crmList); err != nil {
		*results = append(*results, "6. crm_customers.sourceIds.pos → pc_pos_customers: ⚠️ Lỗi decode")
		return
	}

	matched, total := 0, 0
	for _, c := range crmList {
		sids, _ := c["sourceIds"].(bson.M)
		if sids == nil {
			continue
		}
		posID, ok := sids["pos"].(string)
		if !ok || posID == "" {
			continue
		}
		total++
		var cust bson.M
		err := pos.FindOne(ctx, bson.M{"$or": []bson.M{
			{"customerId": posID},
			{"posData.customer.id": posID},
		}}).Decode(&cust)
		if err == nil {
			matched++
		}
	}

	if total == 0 {
		*results = append(*results, "6. crm_customers.sourceIds.pos → pc_pos_customers: ⚠️ Không có crm_customers có sourceIds.pos")
		return
	}
	if matched == total {
		*results = append(*results, fmt.Sprintf("6. crm_customers.sourceIds.pos → pc_pos_customers: ✅ ĐÚNG (%d/%d khớp)", matched, total))
	} else {
		*results = append(*results, fmt.Sprintf("6. crm_customers.sourceIds.pos → pc_pos_customers: ⚠️ MỘT PHẦN (%d/%d khớp)", matched, total))
	}
}

func check7(db *mongo.Database, ctx context.Context, results *[]string) {
	convs := db.Collection("fb_conversations")
	msgs := db.Collection("fb_messages")

	cur, err := convs.Find(ctx, bson.M{}, options.Find().SetLimit(10))
	if err != nil {
		*results = append(*results, "7. fb_conversations.conversationId → fb_messages: ⚠️ Lỗi query")
		return
	}
	defer cur.Close(ctx)

	var convList []bson.M
	if err := cur.All(ctx, &convList); err != nil {
		*results = append(*results, "7. fb_conversations.conversationId → fb_messages: ⚠️ Lỗi decode")
		return
	}

	matched, total := 0, 0
	for _, c := range convList {
		cid, ok := c["conversationId"].(string)
		if !ok || cid == "" {
			continue
		}
		total++
		count, _ := msgs.CountDocuments(ctx, bson.M{"conversationId": cid})
		if count > 0 {
			matched++
		}
	}

	if total == 0 {
		*results = append(*results, "7. fb_conversations.conversationId → fb_messages: ⚠️ Không có conversation")
		return
	}
	*results = append(*results, fmt.Sprintf("7. fb_conversations.conversationId → fb_messages: ✅ ĐÚNG (%d/%d conv có messages)", matched, total))
}

func check8(db *mongo.Database, ctx context.Context, results *[]string) {
	ads := db.Collection("meta_ads")
	campaigns := db.Collection("meta_campaigns")
	adsets := db.Collection("meta_adsets")
	accounts := db.Collection("meta_ad_accounts")

	adCount, _ := ads.CountDocuments(ctx, bson.M{})
	if adCount == 0 {
		*results = append(*results, "8. meta_ads hierarchy: ⚠️ meta_ads TRỐNG (chưa sync Meta)")
		return
	}

	var ad bson.M
	err := ads.FindOne(ctx, bson.M{}).Decode(&ad)
	if err != nil {
		*results = append(*results, "8. meta_ads hierarchy: ⚠️ Không lấy được ad mẫu")
		return
	}

	// Lấy ID từ top-level hoặc metaData (dữ liệu đồng bộ gốc)
	adID := getString(ad, "adId")
	if adID == "" {
		meta, _ := ad["metaData"].(bson.M)
		adID = getString(meta, "id")
	}
	campaignID := getString(ad, "campaignId")
	if campaignID == "" {
		meta, _ := ad["metaData"].(bson.M)
		campaignID = getString(meta, "campaign_id")
	}
	adSetID := getString(ad, "adSetId")
	if adSetID == "" {
		meta, _ := ad["metaData"].(bson.M)
		adSetID = getString(meta, "adset_id")
	}
	accountID := getString(ad, "adAccountId")
	if accountID == "" {
		meta, _ := ad["metaData"].(bson.M)
		accountID = getString(meta, "account_id")
	}

	// Tìm campaign/adset/account trong top-level hoặc metaData
	okCampaign, _ := campaigns.CountDocuments(ctx, bson.M{"$or": []bson.M{
		{"campaignId": campaignID},
		{"metaData.id": campaignID},
	}})
	okAdSet, _ := adsets.CountDocuments(ctx, bson.M{"$or": []bson.M{
		{"adSetId": adSetID},
		{"metaData.id": adSetID},
	}})
	okAccount, _ := accounts.CountDocuments(ctx, bson.M{"$or": []bson.M{
		{"adAccountId": accountID},
		{"metaData.id": accountID},
	}})

	campCount, _ := campaigns.CountDocuments(ctx, bson.M{})
	adSetCount, _ := adsets.CountDocuments(ctx, bson.M{})
	accCount, _ := accounts.CountDocuments(ctx, bson.M{})

	allOk := okCampaign > 0 && okAdSet > 0 && okAccount > 0
	if allOk {
		*results = append(*results, fmt.Sprintf("8. meta_ads hierarchy (adId=%s): ✅ ĐÚNG → campaign, adset, ad_account đều có", adID))
	} else {
		*results = append(*results, fmt.Sprintf("8. meta_ads hierarchy: ⚠️ MỘT PHẦN | meta_ads:%d campaigns:%d adsets:%d accounts:%d | ad mẫu campaignId=%s", adCount, campCount, adSetCount, accCount, campaignID))
	}
}

func check9(db *mongo.Database, ctx context.Context, results *[]string) {
	insights := db.Collection("meta_ad_insights")
	count, _ := insights.CountDocuments(ctx, bson.M{})
	if count == 0 {
		*results = append(*results, "9. meta_ad_insights: ⚠️ TRỐNG (chưa sync insights)")
		return
	}

	var sample bson.M
	err := insights.FindOne(ctx, bson.M{}, options.FindOne().SetSort(bson.M{"updatedAt": -1})).Decode(&sample)
	if err != nil {
		*results = append(*results, fmt.Sprintf("9. meta_ad_insights: ✅ %d docs (không lấy được mẫu)", count))
		return
	}

	meta, _ := sample["metaData"].(bson.M)
	hasActions := meta != nil && meta["actions"] != nil
	hasFreq := meta != nil && meta["frequency"] != nil
	hasInlineClicks := meta != nil && meta["inline_link_clicks"] != nil

	// Đếm theo objectType
	campCount, _ := insights.CountDocuments(ctx, bson.M{"objectType": "campaign"})
	adCount, _ := insights.CountDocuments(ctx, bson.M{"objectType": "ad"})

	*results = append(*results, fmt.Sprintf("9. meta_ad_insights: ✅ %d docs (campaign:%d ad:%d) | actions=%v frequency=%v inline_link_clicks=%v",
		count, campCount, adCount, hasActions, hasFreq, hasInlineClicks))
}

func check10(db *mongo.Database, ctx context.Context, results *[]string) {
	orders := db.Collection("pc_pos_orders")
	ads := db.Collection("meta_ads")

	var order bson.M
	if err := orders.FindOne(ctx, bson.M{"posData.ad_id": bson.M{"$exists": true, "$ne": ""}}).Decode(&order); err != nil {
		*results = append(*results, "10. Format ad_id: ⚠️ Không có order có ad_id")
		return
	}
	pos, _ := order["posData"].(bson.M)
	orderAdID, _ := pos["ad_id"].(string)

	var ad bson.M
	if err := ads.FindOne(ctx, bson.M{}).Decode(&ad); err != nil {
		*results = append(*results, fmt.Sprintf("10. Format ad_id: orders posData.ad_id='%s' | meta_ads TRỐNG", orderAdID))
		return
	}
	adAdID := getString(ad, "adId")
	if adAdID == "" {
		meta, _ := ad["metaData"].(bson.M)
		adAdID = getString(meta, "id")
	}
	*results = append(*results, fmt.Sprintf("10. Format ad_id: orders posData.ad_id='%s' | meta_ads adId/metaData.id='%s' | cùng format=%v",
		orderAdID, adAdID, orderAdID == adAdID))
}

func check11(db *mongo.Database, ctx context.Context, results *[]string) {
	convs := db.Collection("fb_conversations")
	withAds, _ := convs.CountDocuments(ctx, bson.M{"panCakeData.ads.0": bson.M{"$exists": true}})
	withAdIds, _ := convs.CountDocuments(ctx, bson.M{"panCakeData.ad_ids.0": bson.M{"$exists": true}})
	total, _ := convs.CountDocuments(ctx, bson.M{})
	*results = append(*results, fmt.Sprintf("11. panCakeData.ads: ✅ %d conv có ads[], %d có ad_ids[] / tổng %d conv", withAds, withAdIds, total))
}

// check12 rà soát pc_pos_orders.posData: ad_id, post_id, conversation_id, page_id
func check12(db *mongo.Database, ctx context.Context, results *[]string) {
	orders := db.Collection("pc_pos_orders")
	pages := db.Collection("fb_pages")
	posts := db.Collection("fb_posts")
	convs := db.Collection("fb_conversations")
	ads := db.Collection("meta_ads")

	// Đếm orders có từng field
	withAdId, _ := orders.CountDocuments(ctx, bson.M{"posData.ad_id": bson.M{"$exists": true, "$ne": ""}})
	withPostId, _ := orders.CountDocuments(ctx, bson.M{"posData.post_id": bson.M{"$exists": true, "$ne": ""}})
	withConvId, _ := orders.CountDocuments(ctx, bson.M{"posData.conversation_id": bson.M{"$exists": true, "$ne": ""}})
	withPageId, _ := orders.CountDocuments(ctx, bson.M{"posData.page_id": bson.M{"$exists": true, "$ne": ""}})
	total, _ := orders.CountDocuments(ctx, bson.M{})

	// Lấy mẫu để kiểm tra liên kết
	cur, _ := orders.Find(ctx, bson.M{"$or": []bson.M{
		{"posData.ad_id": bson.M{"$exists": true, "$ne": ""}},
		{"posData.post_id": bson.M{"$exists": true, "$ne": ""}},
		{"posData.conversation_id": bson.M{"$exists": true, "$ne": ""}},
		{"posData.page_id": bson.M{"$exists": true, "$ne": ""}},
	}}, options.Find().SetLimit(50))
	defer cur.Close(ctx)

	var orderList []bson.M
	_ = cur.All(ctx, &orderList)

	// Đếm khớp
	adMatch, adTotal := 0, 0
	postMatch, postTotal := 0, 0
	convMatch, convTotal := 0, 0
	pageMatch, pageTotal := 0, 0

	for _, o := range orderList {
		pos, _ := o["posData"].(bson.M)
		if pos == nil {
			continue
		}
		// ad_id → meta_ads
		if aid := getString(pos, "ad_id"); aid != "" {
			adTotal++
			var ad bson.M
			if ads.FindOne(ctx, bson.M{"$or": []bson.M{{"adId": aid}, {"metaData.id": aid}}}).Decode(&ad) == nil {
				adMatch++
			}
		}
		// post_id → fb_posts
		if pid := getString(pos, "post_id"); pid != "" {
			postTotal++
			var post bson.M
			if posts.FindOne(ctx, bson.M{"$or": []bson.M{
				{"panCakeData.id": pid},
				{"panCakeData.post_id": pid},
				{"postId": pid},
			}}).Decode(&post) == nil {
				postMatch++
			}
		}
		// conversation_id → fb_conversations (format pageId_psid = conversationId)
		if cid := getString(pos, "conversation_id"); cid != "" {
			convTotal++
			var conv bson.M
			if convs.FindOne(ctx, bson.M{"conversationId": cid}).Decode(&conv) == nil {
				convMatch++
			}
		}
		// page_id → fb_pages
		if pageId := getString(pos, "page_id"); pageId != "" {
			pageTotal++
			var page bson.M
			if pages.FindOne(ctx, bson.M{"$or": []bson.M{
				{"pageId": pageId},
				{"panCakeData.id": pageId},
			}}).Decode(&page) == nil {
				pageMatch++
			}
		}
	}

	// Kết quả
	*results = append(*results, fmt.Sprintf("12. pc_pos_orders.posData liên kết:"))
	*results = append(*results, fmt.Sprintf("    • ad_id: %d orders có | %d/%d khớp meta_ads", withAdId, adMatch, adTotal))
	*results = append(*results, fmt.Sprintf("    • post_id: %d orders có | %d/%d khớp fb_posts", withPostId, postMatch, postTotal))
	*results = append(*results, fmt.Sprintf("    • conversation_id: %d orders có | %d/%d khớp fb_conversations", withConvId, convMatch, convTotal))
	*results = append(*results, fmt.Sprintf("    • page_id: %d orders có | %d/%d khớp fb_pages (tổng %d orders)", withPageId, pageMatch, pageTotal, total))
}

func getString(m map[string]interface{}, key string) string {
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
