// Đọc nhanh collection decision_org_live_events (MongoDB folkform_auth mặc định).
//
// Dùng để xem nội dung publish (uiTitle, phase, tóm tắt trong payload JSON) mà không cần mongosh.
//
// Chạy từ thư mục api (để bắt được config/env/development.env):
//
//	go run ./cmd/inspect_org_live_events
//	go run ./cmd/inspect_org_live_events -limit=30 -since=48h
//	go run ./cmd/inspect_org_live_events -org=507f1f77bcf86cd799439011
//	go run ./cmd/inspect_org_live_events -trace=trace_abc123
//	go run ./cmd/inspect_org_live_events -db=folkform_data
//
// Biến môi trường: giống server (MONGODB_CONNECTION_URI, MONGODB_DBNAME_AUTH, JWT_SECRET…).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"meta_commerce/config"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const colOrgLive = "decision_run_org_live_events"

// payloadLite — các trường thường dùng để đánh giá “dễ hiểu” trên UI.
type payloadLite struct {
	Phase              string   `json:"phase"`
	Summary            string   `json:"summary"`
	ReasoningSummary   string   `json:"reasoningSummary"`
	SourceTitle        string   `json:"sourceTitle"`
	FeedSourceLabelVi  string   `json:"feedSourceLabelVi"`
	FeedSourceCategory string   `json:"feedSourceCategory"`
	DetailBullets      []string `json:"detailBullets"`
}

func main() {
	limit := flag.Int("limit", 25, "Số bản ghi mới nhất (sort createdAt giảm dần)")
	since := flag.Duration("since", 7*24*time.Hour, "Chỉ lấy createdAt >= now - since (0 = không lọc thời gian)")
	orgHex := flag.String("org", "", "Lọc theo ownerOrganizationId (hex)")
	trace := flag.String("trace", "", "Lọc theo traceId (substring hoặc đủ)")
	dbOverride := flag.String("db", "", "Tên DB (mặc định: MONGODB_DBNAME_AUTH, thường là folkform_auth)")
	fullJSON := flag.Bool("full-json", false, "In thêm toàn bộ payload JSON (dài); mặc định chỉ in tóm tắt")
	flag.Parse()

	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không đọc được config (kiểm tra MONGODB_* và JWT_SECRET trong env / development.env)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	fmt.Printf("=== DB: %s | collection: %s | URI: %s ===\n\n", dbName, colOrgLive, maskURI(cfg.MongoDB_ConnectionURI))

	filter := bson.M{}
	if *since > 0 {
		cutoff := time.Now().Add(-*since).UnixMilli()
		filter["createdAt"] = bson.M{"$gte": cutoff}
	}
	if strings.TrimSpace(*orgHex) != "" {
		oid, err := primitive.ObjectIDFromHex(strings.TrimSpace(*orgHex))
		if err != nil {
			log.Fatalf("-org không phải ObjectId hex hợp lệ: %v", err)
		}
		filter["ownerOrganizationId"] = oid
	}
	if t := strings.TrimSpace(*trace); t != "" {
		// Khớp chuỗi con trong traceId (không cần gõ đủ cả id).
		filter["traceId"] = primitive.Regex{Pattern: regexp.QuoteMeta(t), Options: ""}
	}

	names, err := db.ListCollectionNames(ctx, bson.M{"name": colOrgLive})
	if err != nil {
		log.Fatalf("Liệt kê collection: %v", err)
	}
	if len(names) == 0 {
		log.Fatalf("Không thấy collection %s trong DB %s", colOrgLive, dbName)
	}

	count, err := db.Collection(colOrgLive).CountDocuments(ctx, filter)
	if err != nil {
		log.Fatalf("Đếm document: %v", err)
	}
	fmt.Printf("Số document khớp filter: %d (in tối đa %d bản ghi mới nhất)\n\n", count, *limit)

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(int64(*limit))

	cur, err := db.Collection(colOrgLive).Find(ctx, filter, opts)
	if err != nil {
		log.Fatalf("Find: %v", err)
	}
	defer cur.Close(ctx)

	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		log.Fatalf("Đọc kết quả: %v", err)
	}

	for i, doc := range docs {
		printDoc(i+1, doc, *fullJSON)
	}
	if len(docs) == 0 {
		fmt.Println("(Không có bản ghi — thử nới -since hoặc bỏ -org/-trace.)")
	}
}

func printDoc(idx int, doc bson.M, fullJSON bool) {
	id := doc["_id"]
	org := doc["ownerOrganizationId"]
	createdAt := msToTime(doc["createdAt"])
	phase := str(doc["phase"])
	uiTitle := str(doc["uiTitle"])
	traceID := str(doc["traceId"])
	caseID := str(doc["decisionCaseId"])

	fmt.Printf("────────── #%d ──────────\n", idx)
	fmt.Printf("_id: %v | org: %v | createdAt: %s\n", id, org, createdAt.Format(time.RFC3339))
	fmt.Printf("phase (flat): %q | uiTitle (flat): %q\n", phase, uiTitle)
	fmt.Printf("traceId: %q | decisionCaseId: %q\n", traceID, caseID)

	raw := payloadBytes(doc["payload"])
	if len(raw) == 0 {
		fmt.Println("payload: (trống hoặc không đọc được)")
		fmt.Println()
		return
	}

	var pl payloadLite
	if err := json.Unmarshal(raw, &pl); err != nil {
		fmt.Printf("payload JSON (lỗi parse): %v | len=%d\n", err, len(raw))
		if fullJSON {
			fmt.Println(string(raw))
		}
		fmt.Println()
		return
	}

	fmt.Printf("── Trong payload (JSON) ──\n")
	fmt.Printf("phase: %q | sourceTitle: %q | chip: %q / %q\n",
		pl.Phase, pl.SourceTitle, pl.FeedSourceCategory, pl.FeedSourceLabelVi)
	if pl.Summary != "" {
		fmt.Printf("summary: %s\n", truncate(pl.Summary, 500))
	}
	if pl.ReasoningSummary != "" {
		fmt.Printf("reasoningSummary: %s\n", truncate(pl.ReasoningSummary, 500))
	}
	for j, b := range pl.DetailBullets {
		if j >= 12 {
			fmt.Printf("… (+ %d gạch đầu dòng)\n", len(pl.DetailBullets)-j)
			break
		}
		fmt.Printf("  • %s\n", truncate(b, 300))
	}
	if fullJSON {
		var pretty interface{}
		_ = json.Unmarshal(raw, &pretty)
		b, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Printf("── full payload JSON ──\n%s\n", string(b))
	}
	fmt.Println()
}

func payloadBytes(v interface{}) []byte {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case []byte:
		return x
	case primitive.Binary:
		return x.Data
	case bson.Raw:
		return []byte(x)
	default:
		return nil
	}
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case primitive.ObjectID:
		return x.Hex()
	default:
		return fmt.Sprint(x)
	}
}

func msToTime(v interface{}) time.Time {
	switch x := v.(type) {
	case int64:
		if x <= 0 {
			return time.Time{}
		}
		return time.UnixMilli(x)
	case int32:
		return time.UnixMilli(int64(x))
	case float64:
		return time.UnixMilli(int64(x))
	default:
		return time.Time{}
	}
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
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
