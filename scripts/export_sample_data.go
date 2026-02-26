// Script xuất dữ liệu mẫu từ MongoDB ra thư mục sample-data.
// Chạy: go run scripts/export_sample_data.go
// Xuất tối đa 20 document từ mỗi collection vào docs-shared/ai-context/folkform/sample-data/
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Danh sách collections cần xuất (theo init.go)
var collections = []string{
	"auth_users", "auth_permissions", "auth_roles", "auth_role_permissions", "auth_user_roles",
	"auth_organizations", "auth_organization_config_items", "access_tokens",
	"fb_pages", "fb_conversations", "fb_messages", "fb_message_items", "fb_posts", "fb_customers",
	"pc_orders", "pc_pos_customers", "pc_pos_shops", "pc_pos_warehouses", "pc_pos_products",
	"pc_pos_variations", "pc_pos_categories", "pc_pos_orders",
	"notification_senders", "notification_channels", "notification_templates", "notification_routing_rules",
	"delivery_queue", "delivery_history",
	"cta_library", "cta_tracking",
	"agent_registry", "agent_configs", "agent_commands", "agent_activity_logs",
	"webhook_logs",
	"content_nodes", "content_videos", "content_publications",
	"content_draft_nodes", "content_draft_videos", "content_draft_publications",
	"ai_workflows", "ai_steps", "ai_prompt_templates", "ai_provider_profiles",
	"ai_workflow_runs", "ai_step_runs", "ai_generation_batches", "ai_candidates", "ai_runs", "ai_workflow_commands",
	"report_definitions", "report_snapshots", "report_dirty_periods",
	"crm_customers", "crm_activity_history", "crm_notes",
}

const limitPerCollection = 20
const outputDir = "docs-shared/ai-context/folkform/sample-data"

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

// convertBSONToJSON chuyển document BSON sang JSON, xử lý ObjectID và các kiểu đặc biệt
func convertBSONToJSON(doc bson.M) (map[string]interface{}, error) {
	data, err := bson.MarshalExtJSON(doc, false, false)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("Kết nối MongoDB lỗi: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Tạo thư mục output lỗi: %v", err)
	}

	db := client.Database(dbName)
	success := 0
	skipped := 0

	for _, colName := range collections {
		coll := db.Collection(colName)
		cur, err := coll.Find(ctx, bson.M{}, options.Find().SetLimit(int64(limitPerCollection)))
		if err != nil {
			log.Printf("  [SKIP] %s: %v", colName, err)
			skipped++
			continue
		}

		var docs []bson.M
		if err := cur.All(ctx, &docs); err != nil {
			cur.Close(ctx)
			log.Printf("  [SKIP] %s decode: %v", colName, err)
			skipped++
			continue
		}
		cur.Close(ctx)

		if len(docs) == 0 {
			log.Printf("  [EMPTY] %s", colName)
			skipped++
			continue
		}

		// Chuyển sang JSON-friendly format (ObjectID -> hex string)
		var jsonDocs []map[string]interface{}
		for _, d := range docs {
			j, err := convertBSONToJSON(d)
			if err != nil {
				continue
			}
			jsonDocs = append(jsonDocs, j)
		}

		outPath := filepath.Join(outputDir, colName+".json")
		f, err := os.Create(outPath)
		if err != nil {
			log.Printf("  [SKIP] %s write: %v", colName, err)
			skipped++
			continue
		}
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(jsonDocs); err != nil {
			f.Close()
			log.Printf("  [SKIP] %s encode: %v", colName, err)
			skipped++
			continue
		}
		f.Close()

		log.Printf("  [OK] %s: %d documents -> %s", colName, len(jsonDocs), outPath)
		success++
	}

	// Xuất thêm file tổng hợp thống kê liên kết
	writeLinkageReport(outputDir, db, ctx)
	log.Printf("\nHoàn thành: %d collections, %d bỏ qua. Output: %s", success, skipped, outputDir)
}

// writeLinkageReport tạo file mô tả khả năng liên kết giữa các collection
func writeLinkageReport(dir string, _ *mongo.Database, _ context.Context) {
	report := `# Báo Cáo Liên Kết Dữ Liệu (Tự Động Sinh)

> File này được sinh bởi scripts/export_sample_data.go

## Các Khóa Liên Kết Chính

| Collection | Khóa | Liên kết với |
|------------|------|--------------|
| auth_user_roles | userId, roleId | auth_users, auth_roles |
| auth_role_permissions | roleId, permissionId | auth_roles, auth_permissions |
| auth_roles | ownerOrganizationId | auth_organizations |
| fb_conversations | ownerOrganizationId, pageId | auth_organizations, fb_pages |
| fb_messages | conversationId | fb_conversations |
| fb_message_items | messageId | fb_messages |
| fb_customers | pageId, ownerOrganizationId | fb_pages, auth_organizations |
| pc_pos_orders | shopId, customerId | pc_pos_shops, pc_pos_customers |
| pc_pos_customers | shopId | pc_pos_shops |
| crm_customers | sourceIds.pos, sourceIds.fb, ownerOrganizationId | pc_pos_customers, fb_customers, auth_organizations |
| crm_activity_history | unifiedId, ownerOrganizationId | crm_customers |
| crm_notes | unifiedId, ownerOrganizationId | crm_customers |
| report_snapshots | ownerOrganizationId, reportKey | auth_organizations |

## Luồng Merge Customer

crm_customers = merge(pc_pos_customers, fb_customers) qua:
- sourceIds.pos -> pc_pos_customers.id (Pancake UUID)
- sourceIds.fb -> fb_customers.id
- unifiedId: định danh thống nhất
`
	path := filepath.Join(dir, "_LINKAGE_KEYS.md")
	if err := os.WriteFile(path, []byte(report), 0644); err != nil {
		log.Printf("  [WARN] Không ghi _LINKAGE_KEYS.md: %v", err)
		return
	}
	log.Printf("  [OK] Ghi _LINKAGE_KEYS.md")
}
