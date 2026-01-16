package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"meta_commerce/config"
	"os"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Script này export documents mẫu ra JSON để phân tích cấu trúc
func main() {
	fmt.Println("=== Export Documents Mẫu để Phân Tích ===\n")

	cfg := config.NewConfig()
	if cfg == nil {
		log.Fatal("Không thể đọc cấu hình từ file env")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoDB_ConnectionURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatalf("Không thể kết nối với MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Không thể ping MongoDB: %v", err)
	}

	fmt.Printf("✓ Đã kết nối với MongoDB\n\n")

	// Sử dụng database auth (hầu hết collections nằm ở đây)
	db := client.Database(cfg.MongoDB_DBName_Auth)

	// Tạo thư mục output (từ thư mục gốc dự án)
	currentDir, _ := os.Getwd()
	// Nếu đang ở thư mục api, đi lên 1 cấp
	if filepath.Base(currentDir) == "api" {
		currentDir = filepath.Dir(currentDir)
	}
	outputDir := filepath.Join(currentDir, "docs-shared", "ai-context", "folkform", "sample-data")
	os.MkdirAll(outputDir, 0755)
	fmt.Printf("Output directory: %s\n\n", outputDir)

	// Danh sách tất cả các collections cần export
	collections := []string{
		// Auth & RBAC
		"auth_users",
		"auth_roles",
		"auth_permissions",
		"auth_role_permissions",
		"auth_user_roles",
		"auth_organizations",
		"auth_organization_shares",

		// Facebook Integration
		"fb_pages",
		"fb_posts",
		"fb_conversations",
		"fb_messages",
		"fb_message_items",
		"fb_customers",

		// Customers
		"customers",
		"pc_pos_customers",

		// Pancake POS
		"pc_pos_shops",
		"pc_pos_warehouses",
		"pc_pos_products",
		"pc_pos_variations",
		"pc_pos_categories",
		"pc_pos_orders",
		"pc_orders",

		// Content Storage (Module 1)
		"content_nodes",
		"content_videos",
		"content_publications",
		"content_draft_nodes",
		"content_draft_videos",
		"content_draft_publications",
		"content_draft_approvals",

		// AI Service (Module 2)
		"ai_workflows",
		"ai_steps",
		"ai_prompt_templates",
		"ai_provider_profiles",
		"ai_workflow_runs",
		"ai_step_runs",
		"ai_generation_batches",
		"ai_candidates",
		"ai_workflow_commands",

		// Notification System
		"notification_channels",
		"notification_templates",
		"notification_senders",
		"notification_routing_rules",

		// Delivery System
		"delivery_history",
		"delivery_queue",

		// CTA Module
		"cta_library",
		"cta_tracking",

		// Agent Management
		"agent_registry",
		"agent_configs",
		"agent_commands",
		"agent_activity_logs",

		// Webhook Logs
		"webhook_logs",

		// Access Tokens
		"access_tokens",
	}

	// Export từ database auth
	for _, collName := range collections {
		exportCollection(ctx, db, collName, outputDir)
	}

	// Có thể export từ các database khác nếu cần
	// dbStaging := client.Database(cfg.MongoDB_DBName_Staging)
	// dbData := client.Database(cfg.MongoDB_DBName_Data)

	fmt.Println("\n✓ Hoàn thành export documents mẫu")
}

func exportCollection(ctx context.Context, db *mongo.Database, collName string, outputDir string) {
	collection := db.Collection(collName)

	count, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		fmt.Printf("⚠ %s: Không thể đếm documents\n", collName)
		return
	}

	if count == 0 {
		fmt.Printf("⏭ %s: Không có documents\n", collName)
		return
	}

	// Lấy 10 documents mẫu
	var samples []bson.M
	cursor, err := collection.Find(ctx, bson.M{}, options.Find().SetLimit(10))
	if err != nil {
		fmt.Printf("⚠ %s: Không thể lấy documents\n", collName)
		return
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &samples); err != nil {
		fmt.Printf("⚠ %s: Không thể decode documents\n", collName)
		return
	}

	if len(samples) == 0 {
		return
	}

	// Export ra file JSON
	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s-sample.json", collName))
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("⚠ %s: Không thể tạo file\n", collName)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(samples); err != nil {
		fmt.Printf("⚠ %s: Không thể encode JSON\n", collName)
		return
	}

	fmt.Printf("✓ %s: %d documents → %s\n", collName, count, outputFile)
}
