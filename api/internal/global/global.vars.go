package global

import (
	"database/sql"
	"meta_commerce/config"
	"meta_commerce/internal/registry"

	validator "github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoDB_Auth_CollectionName chứa tên các collection trong MongoDB
type MongoDB_Auth_CollectionName struct {
	Users                   string // Tên collection cho người dùng
	Permissions             string // Tên collection cho quyền
	Roles                   string // Tên collection cho vai trò
	RolePermissions         string // Tên collection cho vai trò và quyền
	UserRoles               string // Tên collection cho người dùng và vai trò
	Organizations           string // Tên collection cho tổ chức
	OrganizationConfigItems string // Tên collection cho config item (1 document per key): auth_organization_config_items
	AccessTokens            string // Tên collection cho token
	FbPages                 string // Tên collection cho trang Facebook
	FbConvesations          string // Tên collection cho cuộc trò chuyện trên Facebook
	FbMessages              string // Tên collection cho metadata tin nhắn trên Facebook
	FbMessageItems          string // Tên collection cho từng message riêng lẻ trên Facebook
	FbPosts                 string // Tên collection cho bài viết trên Facebook
	FbCustomers             string // Tên collection cho khách hàng từ Facebook (Pancake)
	PcPosCustomers          string // Tên collection cho khách hàng từ Pancake POS
	PcPosShops              string // Tên collection cho cửa hàng từ Pancake POS API
	PcPosWarehouses         string // Tên collection cho kho hàng từ Pancake POS API
	PcPosProducts           string // order_src_pcpos_products: mirror sản phẩm từ Pancake POS API
	PcPosVariations         string // order_src_pcpos_variations: mirror biến thể từ Pancake POS API
	PcPosCategories         string // order_src_pcpos_categories: mirror danh mục từ Pancake POS API
	PcPosOrders             string // order_src_pcpos_orders: mirror đơn hàng từ Pancake POS API
	OrderCanonical          string // order_core_records: canonical đơn đa nguồn (Order Intel đọc từ đây)
	// L1 mirror nhập tay — cùng layout model với Pancake (PcPosOrder / PcPosProduct / …); đơn đồng bộ → order_core_records với source=manual.
	ManualPosOrders     string // order_src_manual_orders
	ManualPosProducts   string // order_src_manual_products
	ManualPosVariations string // order_src_manual_variations
	ManualPosCategories string // order_src_manual_categories
	ManualPosCustomers  string // order_src_manual_customers
	ManualPosShops      string // order_src_manual_shops
	ManualPosWarehouses string // order_src_manual_warehouses

	// Notification System Collections (Hệ thống 2 - Routing/Template)
	NotificationSenders      string // Tên collection cho notification senders
	NotificationChannels     string // Tên collection cho notification channels
	NotificationTemplates    string // Tên collection cho notification templates
	NotificationRoutingRules string // Tên collection cho notification routing rules

	// Delivery System Collections (Hệ thống 1 - Gửi)
	DeliveryQueue   string // Tên collection cho delivery queue (đổi từ notification_queue)
	DeliveryHistory string // Tên collection cho delivery history (đổi từ notification_history)

	// CTA Module Collections
	CTALibrary  string // Tên collection cho CTA library
	CTATracking string // Tên collection cho CTA tracking

	// Agent Management System Collections (Bot Management)
	AgentRegistry     string // Tên collection cho agent registry (đã ghép với agent_status)
	AgentConfigs      string // Tên collection cho agent configs
	AgentCommands     string // Tên collection cho agent commands
	AgentActivityLogs string // Tên collection cho agent activity logs

	// Webhook Logs Collection
	WebhookLogs string // Tên collection cho webhook logs (để debug)

	// Module 1: Content Storage Collections (tất cả đều có prefix "content_" để nhất quán)
	ContentNodes      string // Tên collection cho content nodes (L1-L6): content_nodes
	Videos            string // Tên collection cho videos (L7): content_videos
	Publications      string // Tên collection cho publications (L8): content_publications
	DraftContentNodes string // Tên collection cho draft content nodes: content_draft_nodes
	DraftVideos       string // Tên collection cho draft videos: content_draft_videos
	DraftPublications string // Tên collection cho draft publications: content_draft_publications

	// Module 2: AI Service Collections (tất cả đều có prefix "ai_" để nhất quán)
	AIWorkflows         string // Tên collection cho workflows: ai_workflows
	AISteps             string // Tên collection cho steps: ai_steps
	AIPromptTemplates   string // Tên collection cho prompt templates: ai_prompt_templates
	AIProviderProfiles  string // Tên collection cho provider profiles: ai_provider_profiles
	AIWorkflowRuns      string // Tên collection cho workflow runs: ai_workflow_runs
	AIStepRuns          string // Tên collection cho step runs: ai_step_runs
	AIGenerationBatches string // Tên collection cho generation batches: ai_generation_batches
	AICandidates        string // Tên collection cho candidates: ai_candidates
	AIRuns              string // Tên collection cho AI runs: ai_runs
	AIWorkflowCommands  string // Tên collection cho workflow commands: ai_workflow_commands

	// Báo cáo theo chu kỳ (Phase 1)
	ReportDefinitions  string // report_definitions: định nghĩa báo cáo
	ReportSnapshots    string // report_snapshots: kết quả snapshot theo chu kỳ
	ReportDirtyPeriods string // report_dirty_periods: đánh dấu chu kỳ cần tính lại

	// Module Customer (canonical khách — tiền tố customer_, đồng bộ order_/meta_/cix_)
	CustomerCustomers      string // customer_customers: khách đã merge (L2-persist)
	CustomerActivityHistory string // customer_activity_history: lịch sử hoạt động
	CustomerNotes          string // customer_notes: ghi chú khách
	CustomerPendingMerge   string // customer_pending_merge: queue merge L1→L2 (khác CIO ingest)
	CustomerBulkJobs       string // customer_bulk_jobs: queue worker sync, backfill, recalculate
	// CustomerIntelCompute — cùng quy ước *_intel_compute với Ads/Order/CIX.
	CustomerIntelCompute string // customer_intel_compute
	// CustomerIntelRuns — lớp A: mỗi lần chạy intel khách; customer_customers giữ pointer mới nhất.
	CustomerIntelRuns string // customer_intel_runs

	// Module Meta Ads (tiền tố meta_)
	MetaAdAccounts  string // meta_ad_accounts: ad accounts (act_xxx)
	MetaCampaigns    string // meta_campaigns: campaigns
	MetaAdSets       string // meta_adsets: ad sets
	MetaAds          string // meta_ads: ads
	MetaAdInsights   string // meta_ad_insights: insights theo ngày
	MetaAdInsightsDailySnapshots string // meta_ad_insights_daily_snapshots: snapshot mỗi 30p để suy ra hourly

	// Module Approval — Cơ chế duyệt độc lập (ads, content, ... dùng chung)
	ActionPendingApproval string // action_pending_approval: queue đề xuất chờ duyệt (generic)
	ApprovalModeConfig    string // approval_mode_config: config mode duyệt theo domain/scope (Vision 08)

	// Module Ads — Cấu hình duyệt theo ad account (tách khỏi meta)
	AdsApprovalConfig string // ads_approval_config: cấu hình duyệt theo adAccountId

	// Module Ads — Activity History (khi currentMetrics thay đổi)
	AdsActivityHistory string // ads_activity_history: lịch sử thay đổi metrics

	// Module Ads — Meta Config (cấu hình FLAG_RULE, ACTION_RULE, automation)
	AdsMetaConfig string // ads_meta_config: cấu hình quản lý Meta Ads theo ad account

	// Module Ads — Metric Definitions (định nghĩa metrics theo window, FolkForm v4.1)
	AdsMetricDefinitions string // ads_metric_definitions: định nghĩa metrics (7d, 2h, 1h, 30p)

	// Module Ads — Per-Camp Adaptive Threshold (FolkForm v4.1 Section 2.2)
	AdsCampThresholds string // ads_camp_thresholds: P25/P50/P75 theo campaign

	// Module Ads — Counterfactual Kill Tracker (FolkForm v4.1 Section 2.3)
	AdsKillSnapshots         string // ads_kill_snapshots: snapshot khi kill
	AdsCounterfactualOutcomes string // ads_counterfactual_outcomes: kết quả siblings 4h sau kill

	// Module Ads — Hourly Peak Matrix (FolkForm v4.1 Section 05)
	AdsCampaignHourly   string // ads_campaign_hourly: dữ liệu theo giờ
	AdsCampPeakProfiles string // ads_camp_peak_profiles: peak hours mỗi camp

	// Module Ads — Rule 13 Throttle Gỡ cap (FolkForm v4.1)
	AdsThrottleState string // ads_throttle_state: ad set đang bị cap, dùng cho logic remove
	// Module Recompute Debounce Queue — theo dõi giảm chấn tính lại theo entity (dùng chung multi-domain)
	RecomputeDebounceQueue string // decision_recompute_debounce_queue: hàng đợi giảm chấn trước queue domain
	AdsIntelCompute string // ads_intel_compute — job ApplyAdsIntelligenceRecompute / RecalculateAll
	// AdsMetaIntelRuns — lớp A: mỗi lần worker ads_intel_compute kết thúc; meta_campaigns giữ pointer khi recompute_one + có campaign.
	AdsMetaIntelRuns string // ads_meta_intel_runs

	// Module Decision Brain — Learning memory cho AI Commerce
	LearningCases   string // learning_cases: ký ức học tập — 1 case per action, sau outcome (PLATFORM_L1)
	RuleSuggestions string // rule_suggestions: gợi ý điều chỉnh rule từ learning (Phase 3)

	// Module Rule Intelligence — Script-Only Logic Architecture
	RuleDefinitions      string // rule_definitions: Rule Definition
	RuleLogicDefinitions string // rule_logic_definitions: Logic Script
	RuleParamSets        string // rule_param_sets: Parameter Set
	RuleOutputDefinitions string // rule_output_definitions: Output Contract
	RuleExecutionLogs    string // rule_execution_logs: Execution Trace

	// Module CIX — Contextual Conversation Intelligence
	CixAnalysisResults string // cix_analysis_results: lớp A mỗi lần chạy (success/failed), rawFacts tóm tắt, parentJobId, causalOrderingAt, sequence
	CixIntelCompute    string // cix_intel_compute: job phân tích (CIO → enqueue; WorkerCixIntelCompute), cùng quy ước *_intel_compute

	// Module Order Intelligence — Vision 07 (Raw→L1→L2→L3→Flags per order)
	OrderIntelSnapshots string // order_rm_intel: read model B theo đơn (upsert theo orderUid + org)
	OrderIntelCompute   string // order_job_intel — worker tính Raw→L3→Flags, không tính trong consumer AI Decision
	// OrderIntelRuns — lớp A: mỗi lần worker kết thúc (thành công/thất bại); order_core_records giữ pointer mới nhất.
	OrderIntelRuns string // order_run_intel

	// Module AI Decision — Event & Decision Case (PLATFORM_L1_EVENT_DECISION_SUPPLEMENT)
	DecisionEventsQueue   string // decision_events_queue: hàng đợi event chờ AI Decision xử lý
	DecisionCasesRuntime string // decision_cases_runtime: case đang vận hành — từ trigger đến outcome
	DecisionDebounceState string // decision_debounce_state: gom message trước message.batch_ready
	// DecisionTrailingDebounce — trailing defer datachanged (report/crm/cix/order…) + crm intel sau ingest; một doc/khóa, xóa khi flush.
	DecisionTrailingDebounce string
	DecisionRoutingRules  string // decision_routing_rules: override noop/pass_through theo org + eventType
	DecisionContextPolicyOverrides string // decision_context_policy_overrides: matrix required/optional theo org + caseType
	// AIDecisionOrgLiveEvents — Timeline org-live persist: mỗi mốc Publish (live bật + persist bật) một document.
	// Nội dung: BSON BuildOrgLivePersistDocument — trường phẳng (query/UI) + payload ([]byte JSON DecisionLiveEvent). Model index: aidecisionmodels.AIDecisionOrgLiveEvent.
	AIDecisionOrgLiveEvents string
}

// Các biến toàn cục
var Validate *validator.Validate                                                     // Biến để xác thực dữ liệu
var MongoDB_Session *mongo.Client                                                    // Phiên kết nối tới MongoDB
var MongoDB_ServerConfig *config.Configuration                                       // Cấu hình của server
var MongoDB_ColNames MongoDB_Auth_CollectionName = *new(MongoDB_Auth_CollectionName) // Tên các collection
var MySQL_Session *sql.DB                                                            // Add this line to define MySQLDB

// Các Registry
var RegistryCollections = registry.NewRegistry[*mongo.Collection]() // Registry chứa các collections
var RegistryDatabase = registry.NewRegistry[*mongo.Database]()      // Registry chứa các databases
