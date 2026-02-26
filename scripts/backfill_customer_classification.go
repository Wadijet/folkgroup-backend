// Script backfill classification (valueTier, lifecycleStage, journeyStage, channel, loyaltyStage, momentumStage)
// cho crm_customers từ metrics đã có. Chạy sau khi deploy model mới có các field classification.
//
// Chạy: go run scripts/backfill_customer_classification.go
// Hoặc chỉ 1 org: go run scripts/backfill_customer_classification.go <ownerOrganizationId>
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

const (
	valueVip    = 50_000_000
	valueHigh   = 20_000_000
	valueMedium = 5_000_000
	valueLow    = 1_000_000
	lifecycleActive   = 30
	lifecycleCooling  = 90
	lifecycleInactive = 180
	loyaltyCore   = 5
	loyaltyRepeat = 2
	momentumRising   = 0.5
	momentumStableLo = 0.2
	momentumStableHi = 0.5
	msPerDay = 24 * 60 * 60 * 1000
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

func daysSinceLastOrder(lastOrderAt int64) int64 {
	if lastOrderAt <= 0 {
		return -1
	}
	return (time.Now().UnixMilli() - lastOrderAt) / msPerDay
}

func computeValueTier(totalSpent float64) string {
	if totalSpent >= valueVip {
		return "vip"
	}
	if totalSpent >= valueHigh {
		return "high"
	}
	if totalSpent >= valueMedium {
		return "medium"
	}
	if totalSpent >= valueLow {
		return "low"
	}
	return "new"
}

func computeLifecycleStage(lastOrderAt int64) string {
	daysSince := daysSinceLastOrder(lastOrderAt)
	if daysSince < 0 {
		return "never_purchased"
	}
	if daysSince <= lifecycleActive {
		return "active"
	}
	if daysSince <= lifecycleCooling {
		return "cooling"
	}
	if daysSince <= lifecycleInactive {
		return "inactive"
	}
	return "dead"
}

func computeJourneyStage(orderCount int, hasConversation bool, totalSpent float64, lastOrderAt int64) string {
	if orderCount == 0 {
		if hasConversation {
			return "engaged"
		}
		return "visitor"
	}
	daysSince := daysSinceLastOrder(lastOrderAt)
	if daysSince > lifecycleCooling {
		return "inactive"
	}
	if totalSpent >= valueVip {
		return "vip"
	}
	if orderCount >= 2 {
		return "repeat"
	}
	return "first"
}

func computeChannel(orderCount, orderCountOnline, orderCountOffline int) string {
	if orderCount == 0 {
		return ""
	}
	if orderCountOnline > 0 && orderCountOffline > 0 {
		return "omnichannel"
	}
	if orderCountOnline > 0 {
		return "online"
	}
	if orderCountOffline > 0 {
		return "offline"
	}
	return ""
}

func computeLoyaltyStage(orderCount int) string {
	if orderCount >= loyaltyCore {
		return "core"
	}
	if orderCount >= loyaltyRepeat {
		return "repeat"
	}
	if orderCount >= 1 {
		return "one_time"
	}
	return ""
}

func computeMomentumStage(lastOrderAt int64, rev30, rev90, totalSpent float64) string {
	daysSince := daysSinceLastOrder(lastOrderAt)
	if daysSince > lifecycleCooling {
		return "lost"
	}
	if rev90 <= 0 && totalSpent > 0 {
		return "lost"
	}
	if rev90 > 0 && rev30 <= 0 && daysSince <= lifecycleCooling {
		return "declining"
	}
	if rev30 <= 0 {
		return "lost"
	}
	denom := rev90
	if denom < 1 {
		denom = 1
	}
	ratio := rev30 / denom
	if ratio > momentumRising {
		return "rising"
	}
	if ratio >= momentumStableLo && ratio <= momentumStableHi {
		return "stable"
	}
	return "stable"
}

type customerDoc struct {
	ID                primitive.ObjectID `bson:"_id"`
	TotalSpent        float64           `bson:"totalSpent"`
	OrderCount        int               `bson:"orderCount"`
	LastOrderAt       int64             `bson:"lastOrderAt"`
	RevenueLast30d    float64           `bson:"revenueLast30d"`
	RevenueLast90d    float64           `bson:"revenueLast90d"`
	OrderCountOnline  int               `bson:"orderCountOnline"`
	OrderCountOffline int               `bson:"orderCountOffline"`
	HasConversation   bool              `bson:"hasConversation"`
	OwnerOrgID        primitive.ObjectID `bson:"ownerOrganizationId"`
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

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	coll := client.Database(dbName).Collection("crm_customers")

	filter := bson.M{}
	if len(os.Args) > 1 && os.Args[1] != "" {
		orgID, err := primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
		filter = bson.M{"ownerOrganizationId": orgID}
		log.Printf("Lọc theo org: %s\n", os.Args[1])
	}

	projection := bson.M{
		"totalSpent": 1, "orderCount": 1, "lastOrderAt": 1,
		"revenueLast30d": 1, "revenueLast90d": 1,
		"orderCountOnline": 1, "orderCountOffline": 1,
		"hasConversation": 1, "ownerOrganizationId": 1,
	}

	cursor, err := coll.Find(ctx, filter, options.Find().SetProjection(projection).SetBatchSize(500))
	if err != nil {
		log.Fatalf("Find lỗi: %v", err)
	}
	defer cursor.Close(ctx)

	updated := 0
	for cursor.Next(ctx) {
		var doc customerDoc
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		classification := bson.M{
			"valueTier":      computeValueTier(doc.TotalSpent),
			"lifecycleStage": computeLifecycleStage(doc.LastOrderAt),
			"journeyStage":   computeJourneyStage(doc.OrderCount, doc.HasConversation, doc.TotalSpent, doc.LastOrderAt),
			"channel":        computeChannel(doc.OrderCount, doc.OrderCountOnline, doc.OrderCountOffline),
			"loyaltyStage":   computeLoyaltyStage(doc.OrderCount),
			"momentumStage":  computeMomentumStage(doc.LastOrderAt, doc.RevenueLast30d, doc.RevenueLast90d, doc.TotalSpent),
		}
		_, err := coll.UpdateOne(ctx,
			bson.M{"_id": doc.ID},
			bson.M{"$set": bson.M{
				"valueTier":      classification["valueTier"],
				"lifecycleStage": classification["lifecycleStage"],
				"journeyStage":   classification["journeyStage"],
				"channel":        classification["channel"],
				"loyaltyStage":   classification["loyaltyStage"],
				"momentumStage":  classification["momentumStage"],
				"updatedAt":     time.Now().UnixMilli(),
			}},
		)
		if err != nil {
			log.Printf("Update lỗi _id=%v: %v", cursor.Current.Lookup("_id"), err)
			continue
		}
		updated++
		if updated%1000 == 0 {
			log.Printf("Đã cập nhật %d khách...", updated)
		}
	}

	log.Printf("Hoàn thành. Cập nhật %d khách hàng.", updated)
	fmt.Printf("Backfill classification: %d customers updated\n", updated)
}
