// Script kiểm tra activity về lịch sử chat (conversation) trong crm_activity_history.
// Chạy: go run scripts/check_activity_conversation_data.go
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

func main() {
	loadEnv()
	uri := os.Getenv("MONGODB_CONNECTION_URI")
	dbName := os.Getenv("MONGODB_DBNAME_AUTH")
	if uri == "" {
		uri = os.Getenv("MONGODB_ConnectionURI")
	}
	if uri == "" || dbName == "" {
		log.Fatal("Cần MONGODB_CONNECTION_URI và MONGODB_DBNAME_AUTH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	coll := client.Database(dbName).Collection("crm_activity_history")

	// Đếm tổng activity
	total, _ := coll.CountDocuments(ctx, bson.M{})
	fmt.Printf("📊 Tổng activity trong crm_activity_history: %d\n", total)

	// Đếm activity theo domain
	fmt.Println("\n📈 Thống kê theo domain:")
	pipe := []bson.M{
		{"$group": bson.M{"_id": "$domain", "count": bson.M{"$sum": 1}}},
		{"$sort": bson.M{"count": -1}},
	}
	cursor, err := coll.Aggregate(ctx, pipe)
	if err != nil {
		log.Fatalf("Aggregate lỗi: %v", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		domain := doc.ID
		if domain == "" {
			domain = "(empty)"
		}
		fmt.Printf("  - %s: %d\n", domain, doc.Count)
	}

	// Đếm activity conversation
	convCount, _ := coll.CountDocuments(ctx, bson.M{"domain": "conversation"})
	fmt.Printf("\n💬 Activity domain=conversation: %d\n", convCount)

	if convCount > 0 {
		fmt.Println("\n📝 Mẫu 5 activity conversation gần nhất (kèm ownerOrgId):")
		opts := options.Find().SetLimit(5).SetSort(bson.D{{Key: "activityAt", Value: -1}})
		cursor2, err := coll.Find(ctx, bson.M{"domain": "conversation"}, opts)
		if err != nil {
			log.Printf("Find lỗi: %v", err)
		} else {
			defer cursor2.Close(ctx)
			for cursor2.Next(ctx) {
				var doc struct {
					ID                   primitive.ObjectID `bson:"_id"`
					UnifiedId            string             `bson:"unifiedId"`
					OwnerOrganizationID  primitive.ObjectID `bson:"ownerOrganizationId"`
					ActivityType         string             `bson:"activityType"`
					Source               string             `bson:"source"`
					ActivityAt           int64              `bson:"activityAt"`
					DisplayLabel         string             `bson:"displayLabel"`
				}
				if err := cursor2.Decode(&doc); err != nil {
					continue
				}
				ts := time.UnixMilli(doc.ActivityAt).Format("2006-01-02 15:04")
				fmt.Printf("  - %s | unifiedId=%s | orgId=%s | type=%s | %s\n",
					ts, doc.UnifiedId, doc.OwnerOrganizationID.Hex(), doc.ActivityType, doc.DisplayLabel)
			}
		}
	} else {
		fmt.Println("\n⚠️ Không có activity conversation!")
		fmt.Println("   Nguyên nhân có thể:")
		fmt.Println("   1. Chưa chạy backfill: POST /api/v1/customers/backfill-activity với body {\"ownerOrganizationId\": \"<org_id>\"}")
		fmt.Println("   2. fb_conversations chưa có customerId hoặc customerId không resolve được unifiedId")
		fmt.Println("   3. Chạy trước: go run scripts/backfill_fb_customers_from_conversations.go <org_id>")
	}

	// Kiểm tra fb_conversations có customerId không
	fmt.Println("\n📋 Kiểm tra fb_conversations:")
	fbColl := client.Database(dbName).Collection("fb_conversations")
	withCust, _ := fbColl.CountDocuments(ctx, bson.M{"customerId": bson.M{"$exists": true, "$ne": ""}})
	totalConv, _ := fbColl.CountDocuments(ctx, bson.M{})
	fmt.Printf("   Tổng conversations: %d\n", totalConv)
	fmt.Printf("   Có customerId: %d\n", withCust)

	// Tìm 1 customer có conversation trong org của admin (role 698c341c977ebc6295312bb5)
	// auth_roles nằm trong dbName
	roleID, _ := primitive.ObjectIDFromHex("698c341c977ebc6295312bb5")
	var roleDoc struct {
		OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
	}
	if err := client.Database(dbName).Collection("auth_roles").FindOne(ctx, bson.M{"_id": roleID}).Decode(&roleDoc); err == nil {
		orgID := roleDoc.OwnerOrganizationID
		fmt.Printf("\n🔑 Org của admin role: %s\n", orgID.Hex())
		convInOrg, _ := coll.CountDocuments(ctx, bson.M{"domain": "conversation", "ownerOrganizationId": orgID})
		fmt.Printf("   Conversation activities trong org này: %d\n", convInOrg)
		if convInOrg > 0 {
			var sample struct {
				UnifiedId string `bson:"unifiedId"`
			}
			_ = coll.FindOne(ctx, bson.M{"domain": "conversation", "ownerOrganizationId": orgID},
				options.FindOne().SetSort(bson.D{{Key: "activityAt", Value: -1}})).Decode(&sample)
			fmt.Printf("   📌 UnifiedId để test API profile: %s\n", sample.UnifiedId)
		} else {
			fmt.Println("   ⚠️ Org của admin không có conversation activity — có thể do khách trong dashboard chủ yếu từ POS.")
		}
	}
}
