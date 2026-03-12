// Script chẩn đoán sâu: tại sao nhiều visitor trong activity snapshot trong khi crm_customers có engaged?
//
// Chạy: go run scripts/diagnose_visitor_engaged_mismatch.go [ownerOrganizationId]
//
// Kiểm tra:
// 1. Khách engaged trong crm_customers nhưng visitor trong last activity snapshot — có conversation không?
// 2. Phân tích inserted_at vs updated_at trong fb_conversations — có mismatch với filter asOf không?
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

func extractJourneyStage(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	for _, layer := range []string{"layer1", "layer2", "raw"} {
		if sub, ok := m[layer].(map[string]interface{}); ok {
			if v, ok := sub["journeyStage"]; ok && v != nil {
				if s, ok := v.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// getFromCurrentMetrics lấy giá trị từ currentMetrics khi top-level đã bị unset (migration).
func getFromCurrentMetrics(m map[string]interface{}, key string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	cm, ok := m["currentMetrics"].(map[string]interface{})
	if !ok || cm == nil {
		return nil, false
	}
	v, ok := cm[key]
	return v, ok
}

func getConvCountFromDoc(m map[string]interface{}) int {
	if v, ok := getFromCurrentMetrics(m, "conversationCount"); ok && v != nil {
		switch x := v.(type) {
		case int:
			return x
		case int32:
			return int(x)
		case int64:
			return int(x)
		case float64:
			return int(x)
		}
	}
	return 0
}

func getHasConvFromDoc(m map[string]interface{}) bool {
	if v, ok := getFromCurrentMetrics(m, "hasConversation"); ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func getTimestampFromMap(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case string:
		t, err := time.Parse(time.RFC3339, x)
		if err == nil {
			return t.UnixMilli()
		}
	}
	return 0
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

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối: %v", err)
	}
	defer client.Disconnect(ctx)

	db := client.Database(dbName)
	crmColl := db.Collection("crm_customers")
	actColl := db.Collection("crm_activity_history")
	convColl := db.Collection("fb_conversations")

	var orgID primitive.ObjectID
	if len(os.Args) >= 2 {
		orgID, err = primitive.ObjectIDFromHex(os.Args[1])
		if err != nil {
			log.Fatalf("ownerOrganizationId không hợp lệ: %v", err)
		}
	} else {
		var doc struct {
			OwnerOrganizationID primitive.ObjectID `bson:"ownerOrganizationId"`
		}
		if crmColl.FindOne(ctx, bson.M{}, options.FindOne().SetProjection(bson.M{"ownerOrganizationId": 1})).Decode(&doc) == nil {
			orgID = doc.OwnerOrganizationID
		} else {
			log.Fatal("Chạy với: go run scripts/diagnose_visitor_engaged_mismatch.go <ownerOrganizationId>")
		}
	}

	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)
	endMs := endDate.UnixMilli()

	fmt.Printf("=== Chẩn đoán visitor/engaged mismatch — org: %s ===\n\n", orgID.Hex())

	// 1. Lấy last snapshot per customer từ activity
	pipe := []bson.M{
		{"$match": bson.M{
			"ownerOrganizationId":     orgID,
			"activityAt":              bson.M{"$lte": endMs},
			"metadata.metricsSnapshot": bson.M{"$exists": true},
		}},
		{"$sort": bson.M{"activityAt": -1}},
		{"$group": bson.M{
			"_id":             "$unifiedId",
			"metricsSnapshot": bson.M{"$first": "$metadata.metricsSnapshot"},
			"activityAt":     bson.M{"$first": "$activityAt"},
		}},
	}
	cursor, err := actColl.Aggregate(ctx, pipe)
	if err != nil {
		log.Fatalf("Lỗi aggregation activity: %v", err)
	}
	activitySnapshot := make(map[string]struct {
		Stage      string
		ActivityAt int64
	})
	for cursor.Next(ctx) {
		var doc struct {
			ID              string                 `bson:"_id"`
			MetricsSnapshot map[string]interface{} `bson:"metricsSnapshot"`
			ActivityAt      int64                  `bson:"activityAt"`
		}
		if cursor.Decode(&doc) != nil || doc.MetricsSnapshot == nil {
			continue
		}
		stage := extractJourneyStage(doc.MetricsSnapshot)
		if stage == "" {
			stage = "_unspecified"
		}
		activitySnapshot[doc.ID] = struct {
			Stage      string
			ActivityAt int64
		}{Stage: stage, ActivityAt: doc.ActivityAt}
	}
	cursor.Close(ctx)

	// 2. Lấy crm_customers engaged (đọc cả currentMetrics — top-level đã unset sau migration)
	cursor, err = crmColl.Find(ctx, bson.M{
		"ownerOrganizationId": orgID,
		"journeyStage":        "engaged",
	}, options.Find().SetProjection(bson.M{"unifiedId": 1, "conversationCount": 1, "hasConversation": 1, "orderCount": 1, "primarySource": 1, "currentMetrics": 1}))
	if err != nil {
		log.Fatalf("Lỗi: %v", err)
	}
	var engagedDocs []map[string]interface{}
	if err := cursor.All(ctx, &engagedDocs); err != nil {
		log.Fatalf("Lỗi decode: %v", err)
	}
	cursor.Close(ctx)

	// 3. Đếm mismatch: engaged trong crm nhưng visitor trong activity
	mismatchCount := 0
	mismatchWithConv := 0
	mismatchNoConv := 0
	var sampleMismatch []string
	for _, doc := range engagedDocs {
		unifiedId, _ := doc["unifiedId"].(string)
		if unifiedId == "" {
			continue
		}
		snap, ok := activitySnapshot[unifiedId]
		if !ok {
			continue
		}
		if snap.Stage == "visitor" {
			mismatchCount++
			convCount := getConvCountFromDoc(doc)
			hasConv := getHasConvFromDoc(doc)
			if convCount > 0 || hasConv {
				mismatchWithConv++
				if len(sampleMismatch) < 5 {
					sampleMismatch = append(sampleMismatch, unifiedId)
				}
			} else {
				mismatchNoConv++
				if len(sampleMismatch) < 5 {
					sampleMismatch = append(sampleMismatch, unifiedId)
				}
			}
		}
	}

	fmt.Println("--- 1. Mismatch: engaged (crm) vs visitor (activity) ---")
	fmt.Printf("  Tổng engaged trong crm_customers: %d\n", len(engagedDocs))
	fmt.Printf("  Mismatch (engaged crm, visitor activity): %d\n", mismatchCount)
	fmt.Printf("    - Có conversation (conversationCount>0 hoặc hasConversation): %d\n", mismatchWithConv)
	fmt.Printf("    - Không có conversation: %d\n", mismatchNoConv)
	if len(sampleMismatch) > 0 {
		fmt.Printf("  Mẫu unifiedId: %v\n", sampleMismatch)
		// Chi tiết mẫu: đọc từ currentMetrics (top-level có thể đã unset)
		for _, uid := range sampleMismatch[:min(2, len(sampleMismatch))] {
			var doc map[string]interface{}
			if crmColl.FindOne(ctx, bson.M{"unifiedId": uid, "ownerOrganizationId": orgID}).Decode(&doc) != nil {
				continue
			}
			convCount := getConvCountFromDoc(doc)
			hasConv := getHasConvFromDoc(doc)
			orderCount := 0
			if v, ok := getFromCurrentMetrics(doc, "orderCount"); ok && v != nil {
				switch x := v.(type) {
				case int:
					orderCount = x
				case int32:
					orderCount = int(x)
				case int64:
					orderCount = int(x)
				case float64:
					orderCount = int(x)
				}
			}
			primarySource, _ := doc["primarySource"].(string)
			fmt.Printf("    [%s] convCount=%d hasConv=%v orderCount=%d primarySource=%s\n",
				uid, convCount, hasConv, orderCount, primarySource)
		}
	}

	// 4. Kiểm tra fb_conversations: inserted_at vs updated_at (sample)
	fmt.Println("\n--- 2. Phân tích fb_conversations: inserted_at vs updated_at ---")
	cursor, _ = convColl.Find(ctx, bson.M{"ownerOrganizationId": orgID}, options.Find().SetLimit(2000))
	totalWithInserted := 0
	updatedGtInserted := 0
	var sumDiff int64
	for cursor.Next(ctx) {
		var doc struct {
			PanCakeData      map[string]interface{} `bson:"panCakeData"`
			PanCakeUpdatedAt int64                  `bson:"panCakeUpdatedAt"`
			UpdatedAt       int64                  `bson:"updatedAt"`
		}
		if cursor.Decode(&doc) != nil || doc.PanCakeData == nil {
			continue
		}
		ins := getTimestampFromMap(doc.PanCakeData, "inserted_at")
		if ins == 0 {
			ins = getTimestampFromMap(doc.PanCakeData, "insertedAt")
		}
		if ins == 0 {
			ins = getTimestampFromMap(doc.PanCakeData, "created_at")
		}
		if ins == 0 {
			ins = getTimestampFromMap(doc.PanCakeData, "createdAt")
		}
		if ins <= 0 {
			continue
		}
		totalWithInserted++
		upd := doc.PanCakeUpdatedAt
		if upd == 0 {
			upd = doc.UpdatedAt
		}
		if upd > ins {
			updatedGtInserted++
			sumDiff += upd - ins
		}
	}
	cursor.Close(ctx)
	fmt.Printf("  Mẫu 2000 conv: có inserted_at: %d\n", totalWithInserted)
	if totalWithInserted > 0 {
		fmt.Printf("  Số conv có updated_at > inserted_at: %d (%.1f%%)\n",
			updatedGtInserted, float64(updatedGtInserted)/float64(totalWithInserted)*100)
		if updatedGtInserted > 0 {
			fmt.Printf("  Trung bình diff (updated - inserted) ms: %d\n", sumDiff/int64(updatedGtInserted))
		}
	}
	fmt.Println("  → Nếu GetMetricsForSnapshotAt dùng activityAt=inserted_at, filter asOf dùng updated_at <= asOf,")
	fmt.Println("    thì conv có updated_at > inserted_at sẽ BỊ LOẠI → metrics = visitor (sai)!")

	// 5. Với 1 khách mẫu mismatch, kiểm tra chi tiết
	if len(sampleMismatch) > 0 {
		fmt.Println("\n--- 3. Chi tiết 1 khách mẫu mismatch ---")
		unifiedId := sampleMismatch[0]
		var custDoc map[string]interface{}
		if crmColl.FindOne(ctx, bson.M{"unifiedId": unifiedId, "ownerOrganizationId": orgID}).Decode(&custDoc) != nil {
			return
		}
		convCount := getConvCountFromDoc(custDoc)
		hasConv := getHasConvFromDoc(custDoc)
		fmt.Printf("  unifiedId: %s, conversationCount: %d, hasConversation: %v\n", unifiedId, convCount, hasConv)
		sourceIds, _ := custDoc["sourceIds"].(map[string]interface{})
		posId, fbId := "", ""
		if sourceIds != nil {
			posId, _ = sourceIds["pos"].(string)
			fbId, _ = sourceIds["fb"].(string)
		}
		fmt.Printf("  sourceIds: pos=%s fb=%s\n", posId, fbId)

		ids := []string{posId, fbId, unifiedId}
		var convSample []struct {
			CustomerId string `bson:"customerId"`
			InsertedAt int64  `bson:"insertedAt"`
			UpdatedAt  int64  `bson:"updatedAt"`
		}
		filter := bson.M{"ownerOrganizationId": orgID}
		var orCond []bson.M
		for _, id := range ids {
			if id != "" {
				orCond = append(orCond, bson.M{"customerId": id}, bson.M{"panCakeData.customer_id": id})
			}
		}
		if len(orCond) > 0 {
			filter["$or"] = orCond
		}
		cursor, _ = convColl.Find(ctx, filter, options.Find().SetLimit(3))
		for cursor.Next(ctx) {
			var doc struct {
				CustomerId string `bson:"customerId"`
				PanCakeData map[string]interface{} `bson:"panCakeData"`
				PanCakeUpdatedAt int64 `bson:"panCakeUpdatedAt"`
				UpdatedAt int64 `bson:"updatedAt"`
			}
			if cursor.Decode(&doc) == nil {
				ins := getTimestampFromMap(doc.PanCakeData, "inserted_at")
				if ins == 0 {
					ins = getTimestampFromMap(doc.PanCakeData, "insertedAt")
				}
				upd := doc.PanCakeUpdatedAt
				if upd == 0 {
					upd = doc.UpdatedAt
				}
				convSample = append(convSample, struct {
					CustomerId string `bson:"customerId"`
					InsertedAt int64  `bson:"insertedAt"`
					UpdatedAt  int64  `bson:"updatedAt"`
				}{doc.CustomerId, ins, upd})
			}
		}
		cursor.Close(ctx)
		for _, c := range convSample {
			fmt.Printf("  Conv: customerId=%s inserted_at=%d updated_at=%d (updated>inserted: %v)\n",
				c.CustomerId, c.InsertedAt, c.UpdatedAt, c.UpdatedAt > c.InsertedAt)
		}
	}

	fmt.Println("\n✓ Hoàn thành")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
