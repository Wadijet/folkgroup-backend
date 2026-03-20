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
	adsmodels "meta_commerce/internal/api/ads/models"
	metamodels "meta_commerce/internal/api/meta/models"
	ctamodels "meta_commerce/internal/api/cta/models"
	deliverymodels "meta_commerce/internal/api/delivery/models"
	fbmodels "meta_commerce/internal/api/fb/models"
	notifmodels "meta_commerce/internal/api/notification/models"
	pcmodels "meta_commerce/internal/api/pc/models"
	reportmodels "meta_commerce/internal/api/report/models"
	ruleintelmodels "meta_commerce/internal/api/ruleintel/models"
	cixmodels "meta_commerce/internal/api/cix/models"
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
	global.MongoDB_ColNames.Users = "auth_users"
	global.MongoDB_ColNames.Permissions = "auth_permissions"
	global.MongoDB_ColNames.Roles = "auth_roles"
	global.MongoDB_ColNames.RolePermissions = "auth_role_permissions"
	global.MongoDB_ColNames.UserRoles = "auth_user_roles"
	global.MongoDB_ColNames.Organizations = "auth_organizations"
	global.MongoDB_ColNames.OrganizationConfigItems = "auth_organization_config_items"
	global.MongoDB_ColNames.AccessTokens = "access_tokens"
	global.MongoDB_ColNames.FbPages = "fb_pages"
	global.MongoDB_ColNames.FbConvesations = "fb_conversations"
	global.MongoDB_ColNames.FbMessages = "fb_messages"
	global.MongoDB_ColNames.FbMessageItems = "fb_message_items"
	global.MongoDB_ColNames.FbPosts = "fb_posts"
	global.MongoDB_ColNames.FbCustomers = "fb_customers"
	global.MongoDB_ColNames.PcOrders = "pc_orders"
	global.MongoDB_ColNames.PcPosCustomers = "pc_pos_customers"
	global.MongoDB_ColNames.PcPosShops = "pc_pos_shops"
	global.MongoDB_ColNames.PcPosWarehouses = "pc_pos_warehouses"
	global.MongoDB_ColNames.PcPosProducts = "pc_pos_products"
	global.MongoDB_ColNames.PcPosVariations = "pc_pos_variations"
	global.MongoDB_ColNames.PcPosCategories = "pc_pos_categories"
	global.MongoDB_ColNames.PcPosOrders = "pc_pos_orders"

	// Notification System Collections (Hệ thống 2 - Routing/Template)
	global.MongoDB_ColNames.NotificationSenders = "notification_senders"
	global.MongoDB_ColNames.NotificationChannels = "notification_channels"
	global.MongoDB_ColNames.NotificationTemplates = "notification_templates"
	global.MongoDB_ColNames.NotificationRoutingRules = "notification_routing_rules"

	// Delivery System Collections (Hệ thống 1 - Gửi)
	global.MongoDB_ColNames.DeliveryQueue = "delivery_queue"
	global.MongoDB_ColNames.DeliveryHistory = "delivery_history"

	// CTA Module Collections
	global.MongoDB_ColNames.CTALibrary = "cta_library"
	global.MongoDB_ColNames.CTATracking = "cta_tracking"

	// Agent Management System Collections (Bot Management)
	global.MongoDB_ColNames.AgentRegistry = "agent_registry"
	global.MongoDB_ColNames.AgentConfigs = "agent_configs"
	global.MongoDB_ColNames.AgentCommands = "agent_commands"
	// AgentStatus đã được ghép vào AgentRegistry, không cần collection riêng nữa
	global.MongoDB_ColNames.AgentActivityLogs = "agent_activity_logs"

	// Webhook Logs Collection
	global.MongoDB_ColNames.WebhookLogs = "webhook_logs"

	// Module 1: Content Storage Collections (tất cả đều có prefix "content_" để nhất quán)
	global.MongoDB_ColNames.ContentNodes = "content_nodes"
	global.MongoDB_ColNames.Videos = "content_videos"
	global.MongoDB_ColNames.Publications = "content_publications"
	global.MongoDB_ColNames.DraftContentNodes = "content_draft_nodes"
	global.MongoDB_ColNames.DraftVideos = "content_draft_videos"
	global.MongoDB_ColNames.DraftPublications = "content_draft_publications"
	// Module 2: AI Service Collections (tất cả đều có prefix "ai_" để nhất quán)
	global.MongoDB_ColNames.AIWorkflows = "ai_workflows"
	global.MongoDB_ColNames.AISteps = "ai_steps"
	global.MongoDB_ColNames.AIPromptTemplates = "ai_prompt_templates"
	global.MongoDB_ColNames.AIProviderProfiles = "ai_provider_profiles"
	global.MongoDB_ColNames.AIWorkflowRuns = "ai_workflow_runs"
	global.MongoDB_ColNames.AIStepRuns = "ai_step_runs"
	global.MongoDB_ColNames.AIGenerationBatches = "ai_generation_batches"
	global.MongoDB_ColNames.AICandidates = "ai_candidates"
	global.MongoDB_ColNames.AIRuns = "ai_runs"
	global.MongoDB_ColNames.AIWorkflowCommands = "ai_workflow_commands"

	// Báo cáo theo chu kỳ (Phase 1)
	global.MongoDB_ColNames.ReportDefinitions = "report_definitions"
	global.MongoDB_ColNames.ReportSnapshots = "report_snapshots"
	global.MongoDB_ColNames.ReportDirtyPeriods = "report_dirty_periods"

	// Module CRM (tiền tố crm_)
	global.MongoDB_ColNames.CrmCustomers = "crm_customers"
	global.MongoDB_ColNames.CrmActivityHistory = "crm_activity_history"
	global.MongoDB_ColNames.CrmNotes = "crm_notes"
	global.MongoDB_ColNames.CrmPendingIngest = "crm_pending_ingest"
	global.MongoDB_ColNames.CrmBulkJobs = "crm_bulk_jobs"

	// Module Meta Ads
	global.MongoDB_ColNames.MetaAdAccounts = "meta_ad_accounts"
	global.MongoDB_ColNames.MetaCampaigns = "meta_campaigns"
	global.MongoDB_ColNames.MetaAdSets = "meta_adsets"
	global.MongoDB_ColNames.MetaAds = "meta_ads"
	global.MongoDB_ColNames.MetaAdInsights = "meta_ad_insights"
	global.MongoDB_ColNames.MetaAdInsightsDailySnapshots = "meta_ad_insights_daily_snapshots"
	global.MongoDB_ColNames.ActionPendingApproval = "action_pending_approval"
	global.MongoDB_ColNames.ApprovalModeConfig = "approval_mode_config"
	global.MongoDB_ColNames.AdsApprovalConfig = "ads_approval_config"
	global.MongoDB_ColNames.AdsActivityHistory = "ads_activity_history"
	global.MongoDB_ColNames.AdsMetaConfig = "ads_meta_config"
	global.MongoDB_ColNames.AdsMetricDefinitions = "ads_metric_definitions"
	global.MongoDB_ColNames.AdsCampThresholds = "ads_camp_thresholds"
	global.MongoDB_ColNames.AdsKillSnapshots = "ads_kill_snapshots"
	global.MongoDB_ColNames.AdsCounterfactualOutcomes = "ads_counterfactual_outcomes"
	global.MongoDB_ColNames.AdsCampaignHourly = "ads_campaign_hourly"
	global.MongoDB_ColNames.AdsCampPeakProfiles = "ads_camp_peak_profiles"
	global.MongoDB_ColNames.AdsThrottleState = "ads_throttle_state"
	global.MongoDB_ColNames.DecisionCases = "decision_cases"
	global.MongoDB_ColNames.LearningCases = "learning_cases"
	global.MongoDB_ColNames.RuleSuggestions = "rule_suggestions"
	global.MongoDB_ColNames.LearningInsightsAggregate = "learning_insights_aggregate"

	// Module Rule Intelligence
	global.MongoDB_ColNames.RuleDefinitions = "rule_definitions"
	global.MongoDB_ColNames.RuleLogicDefinitions = "rule_logic_definitions"
	global.MongoDB_ColNames.RuleParamSets = "rule_param_sets"
	global.MongoDB_ColNames.RuleOutputDefinitions = "rule_output_definitions"
	global.MongoDB_ColNames.RuleExecutionLogs = "rule_execution_logs"

	// Module CIX — Contextual Conversation Intelligence
	global.MongoDB_ColNames.CixAnalysisResults = "cix_analysis_results"
	global.MongoDB_ColNames.CixPendingAnalysis = "cix_pending_analysis"

	// Module AI Decision — Event & Decision Case (PLATFORM_L1_EVENT_DECISION_SUPPLEMENT)
	global.MongoDB_ColNames.DecisionEventsQueue = "decision_events_queue"
	global.MongoDB_ColNames.DecisionCasesRuntime = "decision_cases_runtime"
	global.MongoDB_ColNames.DecisionDebounceState = "decision_debounce_state"

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
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcOrders), pcmodels.PcOrder{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosCustomers), pcmodels.PcPosCustomer{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosShops), pcmodels.PcPosShop{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosWarehouses), pcmodels.PcPosWarehouse{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosProducts), pcmodels.PcPosProduct{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosVariations), pcmodels.PcPosVariation{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosCategories), pcmodels.PcPosCategory{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.PcPosOrders), pcmodels.PcPosOrder{})

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
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CrmCustomers), crmmodels.CrmCustomer{})
	database.CreateCrmCustomerProfileIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CrmCustomers))
	database.CreateCrmActivityIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CrmActivityHistory))
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CrmNotes), crmmodels.CrmNote{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CrmPendingIngest), crmmodels.CrmPendingIngest{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CrmBulkJobs), crmmodels.CrmBulkJob{})

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
	database.CreateDecisionCaseIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionCases))
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.LearningCases), learningmodels.LearningCase{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleSuggestions), learningmodels.RuleSuggestion{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.LearningInsightsAggregate), learningmodels.LearningInsightAggregate{})

	// Module Rule Intelligence
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleDefinitions), ruleintelmodels.RuleDefinition{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleLogicDefinitions), ruleintelmodels.LogicScript{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleParamSets), ruleintelmodels.ParamSet{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleOutputDefinitions), ruleintelmodels.OutputContract{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.RuleExecutionLogs), ruleintelmodels.RuleExecutionTrace{})

	// Module CIX — Contextual Conversation Intelligence
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CixAnalysisResults), cixmodels.CixAnalysisResult{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.CixPendingAnalysis), cixmodels.CixPendingAnalysis{})

	// Module AI Decision — Event & Decision Case
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionEventsQueue), aidecisionmodels.DecisionEvent{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionCasesRuntime), aidecisionmodels.DecisionCase{})
	database.CreateIndexes(context.TODO(), global.MongoDB_Session.Database(dbName).Collection(global.MongoDB_ColNames.DecisionDebounceState), aidecisionmodels.DebounceState{})
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
