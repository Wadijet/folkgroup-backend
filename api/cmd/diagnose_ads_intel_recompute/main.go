// Chẩn đoán nhanh: queue ads.intelligence.recompute_requested + currentMetrics trên meta_campaigns.
//
// Chạy (từ thư mục api):
//
//	go run ./cmd/diagnose_ads_intel_recompute
//	go run ./cmd/diagnose_ads_intel_recompute -campaign=120232233656750705 -org=<ownerOrganizationIdHex>
//	go run ./cmd/diagnose_ads_intel_recompute -limit=30 -since=168h
//	go run ./cmd/diagnose_ads_intel_recompute -db=folkform_data  // override nếu queue nằm DB khác
//
// Mặc định dùng MONGODB_DBNAME_AUTH (folkform_auth) — trùng init.go (decision_events_queue, meta_campaigns).
// Đọc env giống API (development.env hoặc biến môi trường).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"meta_commerce/config"
	"meta_commerce/internal/api/aidecision/eventtypes"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	colQueue         = "decision_events_queue"
	colMetaCampaigns = "meta_campaigns"
	colDebounceQueue = "decision_recompute_debounce_queue"
)

func main() {
	limit := flag.Int("limit", 20, "Số bản ghi queue mới nhất in chi tiết (mỗi loại event)")
	since := flag.Duration("since", 7*24*time.Hour, "Chỉ thống kê / lọc queue có createdAt trong khoảng thời gian này")
	campaignID := flag.String("campaign", "", "Meta campaignId — in trạng thái currentMetrics trên meta_campaigns (cần kèm -org)")
	orgHex := flag.String("org", "", "ownerOrganizationId (hex) — lọc queue và bắt buộc nếu dùng -campaign")
	dbOverride := flag.String("db", "", "Tên database Mongo (mặc định: MONGODB_DBNAME_AUTH — giống server init)")
	flag.Parse()

	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không đọc được config (kiểm tra MONGODB_* và JWT_SECRET trong env / development.env)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB_ConnectionURI))
	if err != nil {
		log.Fatalf("Kết nối MongoDB: %v", err)
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	dbName := cfg.MongoDB_DBName_Auth
	if strings.TrimSpace(*dbOverride) != "" {
		dbName = strings.TrimSpace(*dbOverride)
	}
	db := client.Database(dbName)
	fmt.Printf("=== DB: %s (mặc định = MONGODB_DBNAME_AUTH) | URI: %s ===\n\n", dbName, maskURI(cfg.MongoDB_ConnectionURI))

	cutoffMs := time.Now().Add(-*since).UnixMilli()
	baseMatch := bson.M{
		"createdAt": bson.M{"$gte": cutoffMs},
	}
	if strings.TrimSpace(*orgHex) != "" {
		oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(*orgHex))
		if err != nil {
			log.Fatalf("-org không phải ObjectId hex hợp lệ: %v", err)
		}
		baseMatch["ownerOrganizationId"] = oid
	}

	// --- 1) Thống kê status: recompute ---
	printSection("1) ads.intelligence.recompute_requested — thống kê theo status (trong -since)")
	matchRecompute := bson.M{
		"eventType": eventtypes.AdsIntelligenceRecomputeRequested,
		"createdAt": bson.M{"$gte": cutoffMs},
	}
	if oid, ok := baseMatch["ownerOrganizationId"]; ok {
		matchRecompute["ownerOrganizationId"] = oid
	}
	aggRecompute := mongo.Pipeline{
		bson.D{{Key: "$match", Value: matchRecompute}},
		bson.D{{Key: "$group", Value: bson.M{"_id": "$status", "n": bson.M{"$sum": 1}}}},
		bson.D{{Key: "$sort", Value: bson.M{"n": -1}}},
	}
	cur, err := db.Collection(colQueue).Aggregate(ctx, aggRecompute)
	if err != nil {
		fmt.Printf("  Lỗi aggregate: %v\n", err)
	} else {
		var rows []bson.M
		_ = cur.All(ctx, &rows)
		if len(rows) == 0 {
			fmt.Println("  (Không có bản ghi nào — có thể chưa emit, sai DB, hoặc -since quá ngắn)")
		}
		for _, r := range rows {
			fmt.Printf("  status=%v  count=%v\n", r["_id"], r["n"])
		}
		_ = cur.Close(ctx)
	}

	// --- 1b) Debounce queue theo objectType/status ---
	printSection("1b) decision_recompute_debounce_queue — thống kê theo objectType + status")
	aggDebounce := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"updatedAt": bson.M{"$gte": cutoffMs}}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id": bson.M{"objectType": "$recalcObjectType", "status": "$status"},
			"n":   bson.M{"$sum": 1},
		}}},
		bson.D{{Key: "$sort", Value: bson.M{"_id.objectType": 1, "_id.status": 1}}},
	}
	if oid, ok := baseMatch["ownerOrganizationId"]; ok {
		aggDebounce[0][0].Value.(bson.M)["ownerOrgId"] = oid
	}
	curDeb, err := db.Collection(colDebounceQueue).Aggregate(ctx, aggDebounce)
	if err != nil {
		fmt.Printf("  Lỗi aggregate debounce queue: %v\n", err)
	} else {
		var rows []bson.M
		_ = curDeb.All(ctx, &rows)
		if len(rows) == 0 {
			fmt.Println("  (Không có bản ghi debounce queue trong -since)")
		}
		for _, r := range rows {
			id, _ := r["_id"].(bson.M)
			fmt.Printf("  objectType=%v status=%v count=%v\n", id["objectType"], id["status"], r["n"])
		}
		_ = curDeb.Close(ctx)
	}

	// --- 2) Chi tiết N job recompute mới nhất ---
	printSection(fmt.Sprintf("2) %s — %d bản ghi mới nhất", eventtypes.AdsIntelligenceRecomputeRequested, *limit))
	if err := printRecentQueue(ctx, db, bson.M{
		"eventType": eventtypes.AdsIntelligenceRecomputeRequested,
		"createdAt": bson.M{"$gte": cutoffMs},
	}, baseMatch, *limit); err != nil {
		fmt.Printf("  Lỗi: %v\n", err)
	}

	// --- 3) Một vài job ads.context_ready (so sánh tần suất với đánh giá) ---
	printSection(fmt.Sprintf("3) %s — %d bản ghi mới nhất", eventtypes.AdsContextReady, min(*limit, 10)))
	if err := printRecentQueue(ctx, db, bson.M{
		"eventType": eventtypes.AdsContextReady,
		"createdAt": bson.M{"$gte": cutoffMs},
	}, baseMatch, min(*limit, 10)); err != nil {
		fmt.Printf("  Lỗi: %v\n", err)
	}

	// --- 4) campaign_intel_recomputed (worker sau recompute Intelligence; legacy: meta_campaign.updated) ---
	printSection(fmt.Sprintf("4) %s — %d bản ghi mới nhất (kèm legacy meta_campaign.updated nếu còn)", eventtypes.CampaignIntelRecomputed, min(*limit, 10)))
	if err := printRecentQueue(ctx, db, bson.M{
		"eventType": bson.M{"$in": bson.A{eventtypes.CampaignIntelRecomputed, eventtypes.MetaCampaignUpdated}},
		"createdAt": bson.M{"$gte": cutoffMs},
	}, baseMatch, min(*limit, 10)); err != nil {
		fmt.Printf("  Lỗi: %v\n", err)
	}

	// --- 4b) Debounce queue chi tiết ---
	printSection(fmt.Sprintf("4b) %s — %d bản ghi mới nhất", colDebounceQueue, min(*limit, 20)))
	if err := printRecentDebounceQueue(ctx, db, baseMatch, min(*limit, 20), cutoffMs); err != nil {
		fmt.Printf("  Lỗi: %v\n", err)
	}

	// --- 5) Campaign: currentMetrics ---
	cid := strings.TrimSpace(*campaignID)
	if cid != "" {
		printSection("5) meta_campaigns — kiểm tra currentMetrics")
		if strings.TrimSpace(*orgHex) == "" {
			fmt.Println("  Bỏ qua: cần -org=<ownerOrganizationIdHex> khi dùng -campaign")
			return
		}
		oid, _ := primitive.ObjectIDFromHex(strings.TrimSpace(*orgHex))
		var doc bson.M
		err := db.Collection(colMetaCampaigns).FindOne(ctx, bson.M{
			"campaignId":          cid,
			"ownerOrganizationId": oid,
		}).Decode(&doc)
		if err != nil {
			fmt.Printf("  Không tìm thấy campaign: %v\n", err)
		} else {
			fmt.Printf("  campaignId=%s  name=%v\n", cid, doc["name"])
			cm, _ := doc["currentMetrics"]
			if cm == nil {
				fmt.Println("  currentMetrics: ❌ nil / không có field")
			} else if m, ok := cm.(bson.M); ok {
				fmt.Printf("  currentMetrics: ✅ có object, ~%d key cấp 1\n", len(m))
			} else {
				fmt.Printf("  currentMetrics: có kiểu %T\n", cm)
			}
		}
	}

	fmt.Println("\n=== Xong ===")
}

func printSection(title string) {
	fmt.Println()
	fmt.Println("--- " + title + " ---")
}

func printRecentQueue(ctx context.Context, db *mongo.Database, typeMatch bson.M, orgFilter bson.M, limit int) error {
	m := bson.M{}
	for k, v := range typeMatch {
		m[k] = v
	}
	if oid, ok := orgFilter["ownerOrganizationId"]; ok {
		m["ownerOrganizationId"] = oid
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(int64(limit))
	cur, err := db.Collection(colQueue).Find(ctx, m, opts)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)

	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return err
	}
	if len(docs) == 0 {
		fmt.Println("  (trống)")
		return nil
	}
	for i, d := range docs {
		eid, _ := d["eventId"].(string)
		st, _ := d["status"].(string)
		ca, _ := d["createdAt"].(int64)
		ts := time.UnixMilli(ca).Format(time.RFC3339)
		errMsg, _ := d["error"].(string)
		payload, _ := d["payload"].(bson.M)
		var payShort string
		if payload != nil {
			payShort = fmt.Sprintf("objectType=%v objectId=%v adAccountId=%v recomputeMode=%v",
				payload["objectType"], payload["objectId"], payload["adAccountId"], payload["recomputeMode"])
		}
		line := fmt.Sprintf("  [%d] %s | %s | status=%s | %s", i+1, ts, eid, st, payShort)
		if errMsg != "" {
			line += fmt.Sprintf(" | error=%q", truncate(errMsg, 120))
		}
		fmt.Println(line)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func maskURI(uri string) string {
	if strings.Contains(uri, "@") {
		return "mongodb://***@" + afterAt(uri)
	}
	return uri
}

func afterAt(uri string) string {
	i := strings.Index(uri, "@")
	if i < 0 {
		return uri
	}
	return uri[i+1:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func printRecentDebounceQueue(ctx context.Context, db *mongo.Database, baseFilter bson.M, limit int, cutoffMs int64) error {
	f := bson.M{"updatedAt": bson.M{"$gte": cutoffMs}}
	if oid, ok := baseFilter["ownerOrganizationId"]; ok {
		f["ownerOrgId"] = oid
	}
	opts := options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}).SetLimit(int64(limit))
	cur, err := db.Collection(colDebounceQueue).Find(ctx, f, opts)
	if err != nil {
		return err
	}
	defer cur.Close(ctx)
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return err
	}
	if len(docs) == 0 {
		fmt.Println("  (trống)")
		return nil
	}
	for i, d := range docs {
		fmt.Printf("  [%d] key=%v | obj=%v:%v | status=%v | next=%v | lastEmit=%v | emitCount=%v | sources=%v\n",
			i+1,
			d["debounceKey"],
			d["recalcObjectType"],
			d["recalcObjectId"],
			d["status"],
			tsMs(d["nextEmitAt"]),
			tsMs(d["lastEmitAt"]),
			d["emitCount"],
			d["sourceKinds"],
		)
	}
	return nil
}

func tsMs(v interface{}) string {
	switch t := v.(type) {
	case int64:
		if t <= 0 {
			return "-"
		}
		return time.UnixMilli(t).Format(time.RFC3339)
	case float64:
		x := int64(t)
		if x <= 0 {
			return "-"
		}
		return time.UnixMilli(x).Format(time.RFC3339)
	default:
		return "-"
	}
}
