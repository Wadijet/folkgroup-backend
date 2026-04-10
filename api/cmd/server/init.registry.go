package main

import (
	"context"

	"meta_commerce/config"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	aidecisionhooks "meta_commerce/internal/api/aidecision/hooks"
	crmvc "meta_commerce/internal/api/crm/service"
	learningsvc "meta_commerce/internal/api/learning/service"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility/identity"
	pkgapproval "meta_commerce/pkg/approval"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func InitRegistry() {
	// Luồng: thay đổi collection nguồn → OnDataChanged → AI Decision → proposals → Executor → Learning
	logrus.Info("Initialized registry")

	// Khởi tạo registry và đăng ký các collections
	err := InitCollections(global.MongoDB_Session, global.MongoDB_ServerConfig)
	if err != nil {
		logrus.Fatalf("Failed to initialize collections: %v", err)
	}
	logrus.Info("Initialized collection registry")

	// Đăng ký identity resolver (external id → uid) cho enrich links khi InsertOne
	if crmSvc, err := crmvc.NewCrmCustomerService(); err == nil {
		identity.SetDefaultResolver(&crmvc.CrmResolver{CrmCustomerService: crmSvc})
		logrus.Info("Identity resolver (CRM) registered")
	} else {
		logrus.Warnf("Identity resolver chưa đăng ký (CRM service: %v)", err)
	}

	// Rule Intelligence (seed system Ads/CRM/CIX/AI Decision dispatch) — InitDefaultData Step 1b, sau System Organization.

	// Đồng bộ: L1 DoSyncUpsert giảm ghi DB. CRUD → OnDataChanged (L2 cổng enqueue) → decision_events_queue → consumer (CRM/Report/Ads).
	decSvc := aidecisionsvc.NewAIDecisionService()
	aidecisionhooks.RegisterAIDecisionOnDataChanged(decSvc)
	logrus.Info("AI Decision: OnDataChanged (L2 cổng queue) → decision_events_queue → consumer CRM/Report/Ads")

	// Đăng ký OnActionClosed: khi action đóng (executed/rejected/failed) — tham số closureType hiện là status cuối (engine), không dùng trực tiếp ở Learning.
	// Learning đọc decisionCaseId + trace từ ActionPending / payload; bỏ qua ghi khi closure decision case không đủ (vision_policy).
	pkgapproval.OnActionClosed = func(ctx context.Context, domain string, doc *pkgapproval.ActionPending, closureType string) {
		_ = closureType
		if doc == nil {
			return
		}
		_, _ = learningsvc.CreateLearningCaseFromAction(ctx, doc)
	}
	logrus.Info("Approval OnActionClosed (Learning per action) registered")
}

// InitCollections khởi tạo và đăng ký các collections MongoDB
func InitCollections(client *mongo.Client, cfg *config.Configuration) error {
	db := client.Database(cfg.MongoDB_DBName_Auth)
	colNames := []string{"auth_users", "auth_permissions", "auth_roles", "auth_role_permissions", "auth_user_roles", "auth_organizations", "auth_organization_config_items", "auth_organization_shares",
		"access_tokens", "fb_pages", "fb_conversations", "fb_messages", "fb_message_items", "fb_posts", "fb_customers", "pc_pos_customers", "pc_pos_shops", "pc_pos_warehouses", "pc_pos_products", "pc_pos_variations", "pc_pos_categories", "pc_pos_orders", "order_canonical",
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
		"customer_customers", "customer_activity_history", "customer_notes", "customer_pending_merge", "customer_bulk_jobs", "customer_intel_compute", "customer_intel_runs",
		"meta_ad_accounts", "meta_campaigns", "meta_adsets", "meta_ads", "meta_ad_insights", "meta_ad_insights_daily_snapshots",
		"action_pending_approval", "approval_mode_config", "ads_approval_config", "ads_activity_history", "ads_meta_config", "ads_metric_definitions", "ads_camp_thresholds",
		"ads_kill_snapshots", "ads_counterfactual_outcomes", "ads_self_competition_state",
		"ads_campaign_hourly", "ads_camp_peak_profiles", "ads_throttle_state", "decision_recompute_debounce_queue", "ads_intel_compute", "ads_meta_intel_runs",
		"learning_cases", "rule_suggestions",
		"rule_definitions", "rule_logic_definitions", "rule_param_sets", "rule_output_definitions", "rule_execution_logs",
		"cix_analysis_results", "cix_intel_compute",
		"order_intel_snapshots", "order_intel_compute", "order_intel_runs",
		"decision_events_queue", "decision_cases_runtime", "decision_debounce_state", "decision_trailing_debounce", "decision_routing_rules", "decision_context_policy_overrides",
		"decision_org_live_events"}

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
