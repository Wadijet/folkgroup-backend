// Script tạo lại dữ liệu mẫu cho các bảng Meta Ads (meta_ad_accounts, meta_campaigns, meta_adsets, meta_ads, meta_ad_insights).
// Sinh dữ liệu trực tiếp trong code, không phụ thuộc file JSON bên ngoài.
// Chạy: go run scripts/seed_meta_ads_sample.go
// Chạy từ thư mục gốc project (có api/config/env/development.env).
package main

import (
	"context"
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

// Danh sách collections Meta Ads
var metaCollections = []string{
	"meta_ad_accounts",
	"meta_campaigns",
	"meta_adsets",
	"meta_ads",
	"meta_ad_insights",
}

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

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH trong .env")
	}

	// ownerOrganizationId từ META_DEFAULT_ORG_ID hoặc env
	orgIDHex := os.Getenv("META_DEFAULT_ORG_ID")
	if orgIDHex == "" {
		orgIDHex = "69a655f0088600c32e62f955"
	}
	ownerOrgID, err := primitive.ObjectIDFromHex(orgIDHex)
	if err != nil {
		log.Fatalf("META_DEFAULT_ORG_ID không hợp lệ: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)

	// Bước 1: Xóa dữ liệu cũ
	log.Println("Bước 1: Xóa dữ liệu cũ trong meta collections...")
	for _, col := range metaCollections {
		coll := db.Collection(col)
		if _, err := coll.DeleteMany(ctx, bson.M{}); err != nil {
			log.Printf("  [WARN] Xóa %s: %v", col, err)
		} else {
			log.Printf("  [CLEAR] %s", col)
		}
	}

	now := time.Now().UnixMilli()

	// Bước 2: Chèn meta_ad_accounts
	log.Println("\nBước 2: Chèn meta_ad_accounts...")
	adAccounts := []bson.M{
		{"adAccountId": "act_123456789012345", "name": "Folkgroup Ad Account - VN", "accountStatus": 1, "currency": "VND", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012346", "name": "Folkgroup Ad Account - Test", "accountStatus": 1, "currency": "VND", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012347", "name": "Brand A - Vietnam", "accountStatus": 1, "currency": "VND", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012348", "name": "Brand B - SEA", "accountStatus": 1, "currency": "USD", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012349", "name": "E-commerce Store 1", "accountStatus": 1, "currency": "VND", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012350", "name": "E-commerce Store 2", "accountStatus": 1, "currency": "VND", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012351", "name": "Agency Client Alpha", "accountStatus": 1, "currency": "USD", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012352", "name": "Agency Client Beta", "accountStatus": 1, "currency": "VND", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012353", "name": "App Install Campaigns", "accountStatus": 1, "currency": "VND", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adAccountId": "act_123456789012354", "name": "Lead Gen - B2B", "accountStatus": 1, "currency": "USD", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
	}
	adAccountID := "act_123456789012345" // Dùng cho campaign/adset/ad/insight
	coll := db.Collection("meta_ad_accounts")
	for _, doc := range adAccounts {
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			log.Printf("  [WARN] meta_ad_accounts insert: %v", err)
		}
	}
	log.Printf("  [OK] meta_ad_accounts: %d documents", len(adAccounts))

	// Bước 3: Chèn meta_campaigns
	log.Println("\nBước 3: Chèn meta_campaigns...")
	campaigns := []bson.M{
		{"campaignId": "1200000000000001", "adAccountId": adAccountID, "name": "Campaign 1 - Awareness", "objective": "OUTCOME_AWARENESS", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"campaignId": "1200000000000002", "adAccountId": adAccountID, "name": "Campaign 2 - Traffic", "objective": "OUTCOME_TRAFFIC", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"campaignId": "1200000000000003", "adAccountId": adAccountID, "name": "Campaign 3 - Conversions", "objective": "OUTCOME_ENGAGEMENT", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"campaignId": "1200000000000004", "adAccountId": adAccountID, "name": "Campaign 4 - Lead Gen", "objective": "OUTCOME_LEADS", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"campaignId": "1200000000000005", "adAccountId": adAccountID, "name": "Campaign 5 - Sales", "objective": "OUTCOME_SALES", "status": "PAUSED", "effectiveStatus": "PAUSED", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
	}
	campaignID := "1200000000000001"
	coll = db.Collection("meta_campaigns")
	for _, doc := range campaigns {
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			log.Printf("  [WARN] meta_campaigns insert: %v", err)
		}
	}
	log.Printf("  [OK] meta_campaigns: %d documents", len(campaigns))

	// Bước 4: Chèn meta_adsets
	log.Println("\nBước 4: Chèn meta_adsets...")
	adsets := []bson.M{
		{"adSetId": "1200000000000011", "campaignId": campaignID, "adAccountId": adAccountID, "name": "AdSet 1 - Vietnam 18-35", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "dailyBudget": "100000", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adSetId": "1200000000000012", "campaignId": campaignID, "adAccountId": adAccountID, "name": "AdSet 2 - Vietnam 35-55", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "dailyBudget": "150000", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adSetId": "1200000000000013", "campaignId": campaignID, "adAccountId": adAccountID, "name": "AdSet 3 - SEA Broad", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "dailyBudget": "200000", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adSetId": "1200000000000014", "campaignId": campaignID, "adAccountId": adAccountID, "name": "AdSet 4 - Retargeting", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adSetId": "1200000000000015", "campaignId": campaignID, "adAccountId": adAccountID, "name": "AdSet 5 - Lookalike", "status": "PAUSED", "effectiveStatus": "PAUSED", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
	}
	adSetID := "1200000000000011"
	coll = db.Collection("meta_adsets")
	for _, doc := range adsets {
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			log.Printf("  [WARN] meta_adsets insert: %v", err)
		}
	}
	log.Printf("  [OK] meta_adsets: %d documents", len(adsets))

	// Bước 5: Chèn meta_ads
	log.Println("\nBước 5: Chèn meta_ads...")
	ads := []bson.M{
		{"adId": "1200000000000021", "adSetId": adSetID, "campaignId": campaignID, "adAccountId": adAccountID, "name": "Ad 1 - Creative A", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "creativeId": "1200000000000031", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adId": "1200000000000022", "adSetId": adSetID, "campaignId": campaignID, "adAccountId": adAccountID, "name": "Ad 2 - Creative B", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "creativeId": "1200000000000032", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adId": "1200000000000023", "adSetId": adSetID, "campaignId": campaignID, "adAccountId": adAccountID, "name": "Ad 3 - Creative C", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adId": "1200000000000024", "adSetId": adSetID, "campaignId": campaignID, "adAccountId": adAccountID, "name": "Ad 4 - Creative D", "status": "ACTIVE", "effectiveStatus": "ACTIVE", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
		{"adId": "1200000000000025", "adSetId": adSetID, "campaignId": campaignID, "adAccountId": adAccountID, "name": "Ad 5 - Creative E", "status": "PAUSED", "effectiveStatus": "PAUSED", "ownerOrganizationId": ownerOrgID, "createdAt": now, "updatedAt": now, "lastSyncedAt": now},
	}
	adID := "1200000000000021"
	coll = db.Collection("meta_ads")
	for _, doc := range ads {
		if _, err := coll.InsertOne(ctx, doc); err != nil {
			log.Printf("  [WARN] meta_ads insert: %v", err)
		}
	}
	log.Printf("  [OK] meta_ads: %d documents", len(ads))

	// Bước 6: Chèn meta_ad_insights (insights theo ngày cho ad_account, campaign, adset, ad)
	log.Println("\nBước 6: Chèn meta_ad_insights...")
	coll = db.Collection("meta_ad_insights")
	insightCount := 0

	// 7 ngày gần đây cho mỗi object type
	objectTypes := []struct {
		objectID   string
		objectType string
	}{
		{adAccountID, "ad_account"},
		{campaignID, "campaign"},
		{adSetID, "adset"},
		{adID, "ad"},
	}

	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		impressions := 1000 + i*500
		clicks := 50 + i*10
		spend := 0.5 + float64(i)*0.2
		reach := 800 + i*400
		cpc := 0.01 + float64(i)*0.002
		cpm := 5.0 + float64(i)*0.5
		ctr := 0.05 + float64(i)*0.01

		for _, obj := range objectTypes {
			doc := bson.M{
				"objectId":            obj.objectID,
				"objectType":          obj.objectType,
				"adAccountId":         adAccountID,
				"dateStart":           date,
				"dateStop":            date,
				"impressions":         strconv.Itoa(impressions),
				"clicks":              strconv.Itoa(clicks),
				"spend":               strconv.FormatFloat(spend, 'f', 2, 64),
				"reach":               strconv.Itoa(reach),
				"cpc":                 strconv.FormatFloat(cpc, 'f', 4, 64),
				"cpm":                 strconv.FormatFloat(cpm, 'f', 2, 64),
				"ctr":                 strconv.FormatFloat(ctr, 'f', 2, 64),
				"ownerOrganizationId": ownerOrgID,
				"createdAt":           now,
				"updatedAt":            now,
			}
			if _, err := coll.InsertOne(ctx, doc); err != nil {
				log.Printf("  [WARN] meta_ad_insights insert: %v", err)
			} else {
				insightCount++
			}
		}
	}
	log.Printf("  [OK] meta_ad_insights: %d documents", insightCount)

	log.Printf("\n✅ Hoàn thành: Đã tạo lại dữ liệu mẫu cho %d collections Meta Ads.\n", len(metaCollections))
	log.Printf("   - meta_ad_accounts: %d\n", len(adAccounts))
	log.Printf("   - meta_campaigns: %d\n", len(campaigns))
	log.Printf("   - meta_adsets: %d\n", len(adsets))
	log.Printf("   - meta_ads: %d\n", len(ads))
	log.Printf("   - meta_ad_insights: %d\n", insightCount)
}
