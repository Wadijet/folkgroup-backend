package main

import (
	"context"
	"encoding/json"
	"meta_commerce/config"
	pkgapproval "meta_commerce/pkg/approval"
	agentmodels "meta_commerce/internal/api/agent/models"
	aimodels "meta_commerce/internal/api/ai/models"
	authmodels "meta_commerce/internal/api/auth/models"
	contentmodels "meta_commerce/internal/api/content/models"
	crmmodels "meta_commerce/internal/api/crm/models"
	adsmodels "meta_commerce/internal/api/ads_meta/models"
	metamodels "meta_commerce/internal/api/meta/models"
	ctamodels "meta_commerce/internal/api/cta/models"
	deliverymodels "meta_commerce/internal/api/delivery/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	notifmodels "meta_commerce/internal/api/notification/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	reportmodels "meta_commerce/internal/api/report/models"
	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	cixmodels "meta_commerce/internal/api/cix/models"
	orderintelmodels "meta_commerce/internal/api/orderintel/models"
	ordermodels "meta_commerce/internal/api/order/models"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	learningmodels "meta_commerce/internal/api/learning/models"
	"meta_commerce/internal/database"
	"meta_commerce/internal/global"
	"meta_commerce/internal/utility"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

// Hàm khởi tạo các biến toàn cục
func InitGlobal() {
	initColNames()         // Khởi tạo tên các collection trong database
	initValidator()        // Khởi tạo validator
	initConfig()           // Khởi tạo cấu hình server
	initDatabase_MongoDB() // Khởi tạo kết nối database
	initFirebase()         // Khởi tạo Firebase
}

// Hàm khởi tạo tên các collection trong database
func initColNames() {
	global.MongoDB_ColNames.Users = "auth_core_users"
	global.MongoDB_ColNames.Permissions = "auth_core_permissions"
	global.MongoDB_ColNames.Roles = "auth_core_roles"
	global.MongoDB_ColNames.RolePermissions = "auth_rel_role_permissions"
	global.MongoDB_ColNames.UserRoles = "auth_rel_user_roles"
	global.MongoDB_ColNames.Organizations = "auth_core_organizations"
	global.MongoDB_ColNames.OrganizationConfigItems = "auth_cfg_organization_items"
	global.MongoDB_ColNames.AccessTokens = "auth_core_access_tokens"
	global.MongoDB_ColNames.FbPages = "fb_src_pages"
	global.MongoDB_ColNames.FbConvesations = "fb_src_conversations"
	global.MongoDB_ColNames.FbMessages = "fb_src_messages"
	global.MongoDB_ColNames.FbMessageItems = "fb_src_message_items"
	global.MongoDB_ColNames.FbPosts = "fb_src_posts"
	global.MongoDB_ColNames.FbCustomers = "fb_src_customers"
	global.MongoDB_ColNames.PcPosCustomers = "pc_pos_src_customers"
	global.MongoDB_ColNames.PcPosShops = "pc_pos_src_shops"
	global.MongoDB_ColNames.PcPosWarehouses = "pc_pos_src_warehouses"
	global.MongoDB_ColNames.PcPosProducts = "order_src_pcpos_products"
	global.MongoDB_ColNames.PcPosVariations = "order_src_pcpos_variations"
	global.MongoDB_ColNames.PcPosCategories = "order_src_pcpos_categories"
	global.MongoDB_ColNames.PcPosOrders = "order_src_pcpos_orders"
	global.MongoDB_ColNames.OrderCanonical = "order_core_records"
	global.MongoDB_ColNames.ManualPosOrders = "order_src_manual_orders"
	global.MongoDB_ColNames.ManualPosProducts = "order_src_manual_products"
	global.MongoDB_ColNames.ManualPosVariations = "order_src_manual_variations"
	global.MongoDB_ColNames.ManualPosCategories = "order_src_manual_categories"
	global.MongoDB_ColNames.ManualPosCustomers = "order_src_manual_customers"
	global.MongoDB_ColNames.ManualPosShops = "order_src_manual_shops"
	global.MongoDB_ColNames.ManualPosWarehouses = "order_src_manual_warehouses"

	// Notification System Collections (Hệ thống 2 - Routing/Template)
	global.MongoDB_ColNames.NotificationSenders = "notification_cfg_senders"
	global.MongoDB_ColNames.NotificationChannels = "notification_cfg_channels"
	global.MongoDB_ColNames.NotificationTemplates = "notification_cfg_templates"
	global.MongoDB_ColNames.NotificationRoutingRules = "notification_cfg_routing_rules"

	// Delivery System Collections (Hệ thống 1 - Gửi)
	global.MongoDB_ColNames.DeliveryQueue = "delivery_job_queue"
	global.MongoDB_ColNames.DeliveryHistory = "delivery_run_history"

	// CTA Module Collections
	global.MongoDB_ColNames.CTALibrary = "cta_core_library"
	global.MongoDB_ColNames.CTATracking = "cta_run_tracking"

	// Agent Management System Collections (Bot Management)
	global.MongoDB_ColNames.AgentRegistry = "agent_core_registry"
	global.MongoDB_ColNames.AgentConfigs = "agent_cfg_configs"
	global.MongoDB_ColNames.AgentCommands = "agent_job_commands"
	// AgentStatus đã được ghép vào AgentRegistry, không cần collection riêng nữa
	global.MongoDB_ColNames.AgentActivityLogs = "agent_run_activity_logs"

	// Webhook Logs Collection
	global.MongoDB_ColNames.WebhookLogs = "webhook_run_logs"

	// Module 1: Content Storage Collections (tất cả đều có prefix "content_" để nhất quán)
	global.MongoDB_ColNames.ContentNodes = "content_core_nodes"
	global.MongoDB_ColNames.Videos = "content_core_videos"
	global.MongoDB_ColNames.Publications = "content_core_publications"
	global.MongoDB_ColNames.DraftContentNodes = "content_draft_nodes"
	global.MongoDB_ColNames.DraftVideos = "content_draft_videos"
	global.MongoDB_ColNames.DraftPublications = "content_draft_publications"
	// Module 2: AI Service Collections (tất cả đều có prefix "ai_" để nhất quán)
	global.MongoDB_ColNames.AIWorkflows = "ai_core_workflows"
	global.MongoDB_ColNames.AISteps = "ai_core_steps"
	global.MongoDB_ColNames.AIPromptTemplates = "ai_cfg_prompt_templates"
	global.MongoDB_ColNames.AIProviderProfiles = "ai_cfg_provider_profiles"
	global.MongoDB_ColNames.AIWorkflowRuns = "ai_run_workflows"
	global.MongoDB_ColNames.AIStepRuns = "ai_run_steps"
	global.MongoDB_ColNames.AIGenerationBatches = "ai_job_generation_batches"
	global.MongoDB_ColNames.AICandidates = "ai_core_candidates"
	global.MongoDB_ColNames.AIRuns = "ai_run_generations"
	global.MongoDB_ColNames.AIWorkflowCommands = "ai_job_workflow_commands"

	// Báo cáo theo chu kỳ (Phase 1)
	global.MongoDB_ColNames.ReportDefinitions = "report_cfg_definitions"
	global.MongoDB_ColNames.ReportSnapshots = "report_rm_snapshots"
	global.MongoDB_ColNames.ReportDirtyPeriods = "report_state_dirty_periods"

	// Module Customer (tiền tố customer_)
	global.MongoDB_ColNames.CustomerCustomers = "customer_core_records"
	global.MongoDB_ColNames.CustomerActivityHistory = "customer_run_activity_history"
	global.MongoDB_ColNames.CustomerNotes = "customer_core_notes"
	global.MongoDB_ColNames.CustomerPendingMerge = "customer_job_pending_merge"
	global.MongoDB_ColNames.CustomerBulkJobs = "customer_job_bulk"
	global.MongoDB_ColNames.CustomerIntelCompute = "customer_job_intel"
	global.MongoDB_ColNames.CustomerIntelRuns = "customer_run_intel"

	// Module Meta Ads
	global.MongoDB_ColNames.MetaAdAccounts = "meta_src_ad_accounts"
	global.MongoDB_ColNames.MetaCampaigns = "meta_src_campaigns"
	global.MongoDB_ColNames.MetaAdSets = "meta_src_adsets"
	global.MongoDB_ColNames.MetaAds = "meta_src_ads"
	global.MongoDB_ColNames.MetaAdInsights = "meta_src_ad_insights"
	global.MongoDB_ColNames.MetaAdInsightsDailySnapshots = "meta_rm_ad_insights_daily_snapshots"
	global.MongoDB_ColNames.ActionPendingApproval = "approval_job_pending_actions"
	global.MongoDB_ColNames.ApprovalModeConfig = "approval_cfg_mode"
	global.MongoDB_ColNames.AdsApprovalConfig = "ads_cfg_approval"
	global.MongoDB_ColNames.AdsActivityHistory = "ads_run_activity_history"
	global.MongoDB_ColNames.AdsMetaConfig = "ads_cfg_meta"
	global.MongoDB_ColNames.AdsMetricDefinitions = "ads_cfg_metric_definitions"
	global.MongoDB_ColNames.AdsCampThresholds = "ads_cfg_campaign_thresholds"
	global.MongoDB_ColNames.AdsKillSnapshots = "ads_rm_kill_snapshots"
	global.MongoDB_ColNames.AdsCounterfactualOutcomes = "ads_run_counterfactual_outcomes"
	global.MongoDB_ColNames.AdsCampaignHourly = "ads_rm_campaign_hourly"
	global.MongoDB_ColNames.AdsCampPeakProfiles = "ads_rm_campaign_peak_profiles"
	global.MongoDB_ColNames.AdsThrottleState = "ads_state_throttle"
	global.MongoDB_ColNames.RecomputeDebounceQueue = "decision_state_recompute_debounce"
	global.MongoDB_ColNames.AdsIntelCompute = "ads_job_intel"
	global.MongoDB_ColNames.AdsMetaIntelRuns = "ads_run_intel"
	global.MongoDB_ColNames.LearningCases = "learning_core_cases"
	global.MongoDB_ColNames.RuleSuggestions = "learning_rm_rule_suggestions"

	// Module Rule Intelligence
	global.MongoDB_ColNames.RuleDefinitions = "rule_cfg_definitions"
	global.MongoDB_ColNames.RuleLogicDefinitions = "rule_cfg_logic_definitions"
	global.MongoDB_ColNames.RuleParamSets = "rule_cfg_param_sets"
	global.MongoDB_ColNames.RuleOutputDefinitions = "rule_cfg_output_definitions"
	global.MongoDB_ColNames.RuleExecutionLogs = "rule_run_execution_logs"

	// Module CIX — Contextual Conversation Intelligence
	global.MongoDB_ColNames.CixAnalysisResults = "cix_run_analysis_results"
	global.MongoDB_ColNames.CixIntelCompute = "cix_job_intel"

	// Module Order Intelligence — Vision 07
	global.MongoDB_ColNames.OrderIntelSnapshots = "order_rm_intel"
	global.MongoDB_ColNames.OrderIntelCompute = "order_job_intel"
	global.MongoDB_ColNames.OrderIntelRuns = "order_run_intel"

	// Module AI Decision — Event & Decision Case (PLATFORM_L1_EVENT_DECISION_SUPPLEMENT)
	global.MongoDB_ColNames.DecisionEventsQueue = "decision_job_events"
	global.MongoDB_ColNames.DecisionCasesRuntime = "decision_state_cases_runtime"
	global.MongoDB_ColNames.DecisionDebounceState = "decision_state_debounce"
	global.MongoDB_ColNames.DecisionTrailingDebounce = "decision_state_trailing_debounce"
	global.MongoDB_ColNames.DecisionRoutingRules = "decision_cfg_routing_rules"
	global.MongoDB_ColNames.DecisionContextPolicyOverrides = "decision_cfg_context_policy_overrides"
	global.MongoDB_ColNames.AIDecisionOrgLiveEvents = "decision_run_org_live_events"

	logrus.Info("Initialized collection names") // Ghi log thông báo đã khởi tạo tên các collection
}

// Hàm khởi tạo validator (dùng global.InitValidator để đăng ký custom validators: no_xss, exists, config_value, ...)
func initValidator() {
	global.InitValidator()
	logrus.Info("Initialized validator") // Ghi log thông báo đã khởi tạo validator
}

// Hàm khởi tạo cấu hình server
func initConfig() {
	global.MongoDB_ServerConfig = config.NewConfig()
	if global.MongoDB_ServerConfig == nil {
		logrus.Fatalf("Failed to initialize config: config is nil") // Ghi log lỗi nếu khởi tạo cấu hình thất bại
	}
	logrus.Info("Initialized server config") // Ghi log thông báo đã khởi tạo cấu hình server
}

// Hàm khởi tạo kết nối database
func initDatabase_MongoDB() {
	var err error
	global.MongoDB_Session, err = database.GetInstance(global.MongoDB_ServerConfig)
	if err != nil {
		logrus.Fatalf("Failed to get database instance: %v", err) // Ghi log lỗi nếu kết nối database thất bại
	}
	logrus.Info("Connected to MongoDB") // Ghi log thông báo đã kết nối database thành công

	// DB cũ (tên collection trước refactor): đổi tên sang chuẩn mới — bật MONGO_LEGACY_COLLECTION_RENAME=1
	if err := database.InitRenameLegacyMongoCollectionsIfEnabled(global.MongoDB_Session, global.MongoDB_ServerConfig.MongoDB_DBName_Auth); err != nil {
		logrus.Fatalf("Legacy rename collection: %v", err)
	}

	// Khởi tạo các db và collections nếu chưa có
	database.EnsureDatabaseAndCollections(global.MongoDB_Session)
	logrus.Info("Ensured database and collections") // Ghi log thông báo đã đảm bảo database và các collection

	// Khơi tạo các index cho các collection
	dbName := global.MongoDB_ServerConfig.MongoDB_DBName_Auth
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.Users), authmodels.User{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.Permissions), authmodels.Permission{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.Roles), authmodels.Role{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.UserRoles), authmodels.UserRole{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RolePermissions), authmodels.RolePermission{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.Organizations), authmodels.Organization{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.OrganizationConfigItems), authmodels.OrganizationConfigItem{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AccessTokens), pcmodels.AccessToken{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.FbPages), fbmodels.FbPage{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.FbConvesations), fbmodels.FbConversation{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.FbMessages), fbmodels.FbMessage{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.FbMessageItems), fbmodels.FbMessageItem{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.FbPosts), fbmodels.FbPost{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.FbCustomers), fbmodels.FbCustomer{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosCustomers), pcmodels.PcPosCustomer{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosShops), pcmodels.PcPosShop{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosWarehouses), pcmodels.PcPosWarehouse{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosProducts), pcmodels.PcPosProduct{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosVariations), pcmodels.PcPosVariation{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosCategories), pcmodels.PcPosCategory{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosOrders), pcmodels.PcPosOrder{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ManualPosOrders), pcmodels.PcPosOrder{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ManualPosProducts), pcmodels.PcPosProduct{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ManualPosVariations), pcmodels.PcPosVariation{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ManualPosCategories), pcmodels.PcPosCategory{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ManualPosCustomers), pcmodels.PcPosCustomer{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ManualPosShops), pcmodels.PcPosShop{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ManualPosWarehouses), pcmodels.PcPosWarehouse{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.OrderCanonical), ordermodels.CommerceOrder{})

	// Notification System Indexes (Hệ thống 2 - Routing/Template)
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.NotificationSenders), notifmodels.NotificationChannelSender{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.NotificationChannels), notifmodels.NotificationChannel{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.NotificationTemplates), notifmodels.NotificationTemplate{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.NotificationRoutingRules), notifmodels.NotificationRoutingRule{})

	// Delivery System Indexes (Hệ thống 1 - Gửi)
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DeliveryQueue), deliverymodels.DeliveryQueueItem{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DeliveryHistory), deliverymodels.DeliveryHistory{})

	// CTA Module Indexes
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CTALibrary), ctamodels.CTALibrary{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CTATracking), ctamodels.CTATracking{})

	// Agent Management System Indexes (Bot Management)
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AgentRegistry), agentmodels.AgentRegistry{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AgentConfigs), agentmodels.AgentConfig{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AgentCommands), agentmodels.AgentCommand{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AgentActivityLogs), agentmodels.AgentActivityLog{})

	// Module 1: Content Storage Indexes
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ContentNodes), contentmodels.ContentNode{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.Videos), contentmodels.Video{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.Publications), contentmodels.Publication{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DraftContentNodes), contentmodels.DraftContentNode{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DraftVideos), contentmodels.DraftVideo{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DraftPublications), contentmodels.DraftPublication{})
	// Module 2: AI Service Indexes
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIWorkflows), aimodels.AIWorkflow{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AISteps), aimodels.AIStep{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIPromptTemplates), aimodels.AIPromptTemplate{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIProviderProfiles), aimodels.AIProviderProfile{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIWorkflowRuns), aimodels.AIWorkflowRun{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIStepRuns), aimodels.AIStepRun{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIGenerationBatches), aimodels.AIGenerationBatch{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AICandidates), aimodels.AICandidate{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIRuns), aimodels.AIRun{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIWorkflowCommands), aimodels.AIWorkflowCommand{})

	// Báo cáo theo chu kỳ (Phase 1)
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ReportDefinitions), reportmodels.ReportDefinition{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ReportSnapshots), reportmodels.ReportSnapshot{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ReportDirtyPeriods), reportmodels.ReportDirtyPeriod{})

	// Module CRM
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CustomerCustomers), crmmodels.CrmCustomer{})
	database.CreateCrmCustomerProfileIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CustomerCustomers))
	database.CreateCrmActivityIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CustomerActivityHistory))
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CustomerNotes), crmmodels.CrmNote{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CustomerPendingMerge), crmmodels.CrmPendingMerge{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CustomerBulkJobs), crmmodels.CrmBulkJob{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CustomerIntelCompute), crmmodels.CrmIntelComputeJob{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CustomerIntelRuns), crmmodels.CrmCustomerIntelRun{})

	// Module Meta Ads
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.MetaAdAccounts), metamodels.MetaAdAccount{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.MetaCampaigns), metamodels.MetaCampaign{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.MetaAdSets), metamodels.MetaAdSet{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.MetaAds), metamodels.MetaAd{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.MetaAdInsights), metamodels.MetaAdInsight{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.MetaAdInsightsDailySnapshots), metamodels.MetaAdInsightDailySnapshot{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ActionPendingApproval), pkgapproval.ActionPending{})
	database.CreateActionPendingIdempotencyIndex(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ActionPendingApproval))
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.ApprovalModeConfig), pkgapproval.ApprovalModeConfig{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsApprovalConfig), adsmodels.AdsApprovalConfig{})
	database.CreateAdsActivityIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsActivityHistory))
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsMetricDefinitions), adsmodels.AdsMetricDefinition{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsCampThresholds), adsmodels.AdsCampThresholds{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsKillSnapshots), adsmodels.AdsKillSnapshot{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsCounterfactualOutcomes), adsmodels.AdsCounterfactualOutcome{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsCampaignHourly), adsmodels.AdsCampaignHourly{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsCampPeakProfiles), adsmodels.AdsCampPeakProfile{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsThrottleState), adsmodels.AdsThrottleState{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RecomputeDebounceQueue), metamodels.RecomputeDebounceQueue{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsIntelCompute), adsmodels.AdsIntelComputeJob{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AdsMetaIntelRuns), adsmodels.AdsMetaIntelRun{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.LearningCases), learningmodels.LearningCase{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleSuggestions), learningmodels.RuleSuggestion{})

	// Module Rule Intelligence
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleDefinitions), ruleintelmodels.RuleDefinition{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleLogicDefinitions), ruleintelmodels.LogicScript{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleParamSets), ruleintelmodels.ParamSet{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleOutputDefinitions), ruleintelmodels.OutputContract{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleExecutionLogs), ruleintelmodels.RuleExecutionTrace{})

	// Module CIX — Contextual Conversation Intelligence
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CixAnalysisResults), cixmodels.CixAnalysisResult{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CixIntelCompute), cixmodels.CixIntelComputeJob{})

	// Module Order Intelligence
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.OrderIntelSnapshots), orderintelmodels.OrderIntelligenceSnapshot{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.OrderIntelCompute), orderintelmodels.OrderIntelComputeJob{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.OrderIntelRuns), orderintelmodels.OrderIntelRun{})

	// Module AI Decision — Event & Decision Case
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionEventsQueue), aidecisionmodels.DecisionEvent{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionCasesRuntime), aidecisionmodels.DecisionCase{})
	if err := database.EnsureDecisionCaseRuntimeExtraIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionCasesRuntime)); err != nil {
		logrus.Warnf("decision_cases_runtime extra indexes: %v", err)
	}
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionDebounceState), aidecisionmodels.DebounceState{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionTrailingDebounce), aidecisionmodels.TrailingDebounceSlot{})
	if err := database.EnsureDecisionTrailingDebounceIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionTrailingDebounce)); err != nil {
		logrus.Warnf("decision_trailing_debounce compound index: %v", err)
	}
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionRoutingRules), aidecisionmodels.DecisionRoutingRule{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionContextPolicyOverrides), aidecisionmodels.DecisionContextPolicyOverride{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.AIDecisionOrgLiveEvents), aidecisionmodels.AIDecisionOrgLiveEvent{})
}

// initFirebase khởi tạo Firebase Admin SDK
func initFirebase() {
	// #region agent log
	wd, _ := os.Getwd()
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	logData, _ := json.Marshal(map[string]interface{}{
		"sessionId":    "debug-session",
		"runId":        "run1",
		"hypothesisId": "A",
		"location":     "init.go:90",
		"message":      "initFirebase entry - working directory và executable path",
		"data": map[string]interface{}{
			"workingDirectory": wd,
			"executablePath":   execPath,
			"executableDir":    execDir,
		},
		"timestamp": time.Now().UnixMilli(),
	})
	if f, err := os.OpenFile("d:\\Crossborder\\ff_be_auth\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		f.WriteString(string(logData) + "\n")
		f.Close()
	}
	// #endregion

	cfg := global.MongoDB_ServerConfig

	// #region agent log
	logData2, _ := json.Marshal(map[string]interface{}{
		"sessionId":    "debug-session",
		"runId":        "run1",
		"hypothesisId": "E",
		"location":     "init.go:94",
		"message":      "Firebase config values từ env",
		"data": map[string]interface{}{
			"firebaseProjectID":       cfg.FirebaseProjectID,
			"firebaseCredentialsPath": cfg.FirebaseCredentialsPath,
		},
		"timestamp": time.Now().UnixMilli(),
	})
	if f, err := os.OpenFile("d:\\Crossborder\\ff_be_auth\\.cursor\\debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		f.WriteString(string(logData2) + "\n")
		f.Close()
	}
	// #endregion

	// Kiểm tra Firebase config có đầy đủ không
	if cfg.FirebaseProjectID == "" || cfg.FirebaseCredentialsPath == "" {
		logrus.Warn("Firebase config không đầy đủ, bỏ qua khởi tạo Firebase")
		return
	}

	err := utility.InitFirebase(cfg.FirebaseProjectID, cfg.FirebaseCredentialsPath)
	if err != nil {
		logrus.Errorf("Failed to initialize Firebase: %v", err)
		// Không fatal, chỉ log warning để hệ thống vẫn chạy được
		return
	}

	logrus.Info("Firebase initialized successfully")
}
