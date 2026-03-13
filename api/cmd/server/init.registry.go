package main

import (
	"context"
	"meta_commerce/config"
	basesvc "meta_commerce/internal/api/base/service"
	ruleintelmigration "meta_commerce/internal/api/ruleintel/migration"
	"meta_commerce/internal/global"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func InitRegistry() {

	logrus.Info("Initialized registry") // Ghi log thông báo đã khởi tạo registry

	// Khởi tạo registry và đăng ký các collections
	err := InitCollections(global.MongoDB_Session, global.MongoDB_ServerConfig)
	if err != nil {
		logrus.Fatalf("Failed to initialize collections: %v", err)
	}
	logrus.Info("Initialized collection registry")

	// Seed Rule Intelligence — toàn bộ rules Ads (OwnerOrganizationID + IsSystem, chuẩn init)
	initCtx := basesvc.WithSystemDataInsertAllowed(context.Background())
	if err := ruleintelmigration.SeedRuleAdsSystem(initCtx); err != nil {
		logrus.Warnf("Rule Intelligence seed (optional): %v", err)
	} else {
		logrus.Info("Rule Intelligence seed completed (system rules)")
	}
}

// InitCollections khởi tạo và đăng ký các collections MongoDB
func InitCollections(client *mongo.Client, cfg *config.Configuration) error {
	db := client.Database(cfg.MongoDB_DBName_Auth)
	colNames := []string{"auth_users", "auth_permissions", "auth_roles", "auth_role_permissions", "auth_user_roles", "auth_organizations", "auth_organization_config_items", "auth_organization_shares",
		"access_tokens", "fb_pages", "fb_conversations", "fb_messages", "fb_message_items", "fb_posts", "fb_customers", "pc_orders", "pc_pos_customers", "pc_pos_shops", "pc_pos_warehouses", "pc_pos_products", "pc_pos_variations", "pc_pos_categories", "pc_pos_orders",
		"notification_senders", "notification_channels", "notification_templates", "notification_routing_rules",
		"delivery_queue", "delivery_history",
		"cta_library", "cta_tracking",
		"agent_registry", "agent_configs", "agent_commands", "agent_activity_logs",
		"webhook_logs",
		// Module 1: Content Storage Collections (tất cả đều có prefix "content_" để nhất quán)
		"content_nodes", "content_videos", "content_publications",
		"content_draft_nodes", "content_draft_videos", "content_draft_publications",
		// Module 2: AI Service Collections (tất cả đều có prefix "ai_" để nhất quán)
		"ai_workflows", "ai_steps", "ai_prompt_templates", "ai_provider_profiles", "ai_workflow_runs", "ai_step_runs",
		"ai_generation_batches", "ai_candidates", "ai_runs", "ai_workflow_commands",
		"report_definitions", "report_snapshots", "report_dirty_periods",
		"crm_customers", "crm_activity_history", "crm_notes", "crm_pending_ingest", "crm_bulk_jobs",
		"meta_ad_accounts", "meta_campaigns", "meta_adsets", "meta_ads", "meta_ad_insights", "meta_ad_insights_daily_snapshots",
		"action_pending_approval", "ads_approval_config", "ads_activity_history", "ads_meta_config", "ads_metric_definitions", "ads_camp_thresholds",
		"ads_kill_snapshots", "ads_counterfactual_outcomes", "ads_self_competition_state",
		"ads_campaign_hourly", "ads_camp_peak_profiles", "ads_throttle_state",
		"decision_cases",
		"rule_definitions", "rule_logic_definitions", "rule_param_sets", "rule_output_definitions", "rule_execution_logs"}

	for _, name := range colNames {
		registered, err := global.RegistryCollections.Register(name, db.Collection(name))
		if err != nil {
			logrus.Errorf("Failed to register collection %s: %v", name, err)
			return err
		}

		if registered {
			logrus.Infof("Collection %s registered successfully", name)
		} else {
			logrus.Errorf("Collection %s already registered", name)
		}

	}

	return nil
}
