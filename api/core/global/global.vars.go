package global

import (
	"database/sql"
	"meta_commerce/config"
	"meta_commerce/core/registry"

	validator "github.com/go-playground/validator/v10"
	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoDB_Auth_CollectionName chứa tên các collection trong MongoDB
type MongoDB_Auth_CollectionName struct {
	Users           string // Tên collection cho người dùng
	Permissions     string // Tên collection cho quyền
	Roles           string // Tên collection cho vai trò
	RolePermissions string // Tên collection cho vai trò và quyền
	UserRoles       string // Tên collection cho người dùng và vai trò
	Organizations   string // Tên collection cho tổ chức
	AccessTokens    string // Tên collection cho token
	FbPages         string // Tên collection cho trang Facebook
	FbConvesations  string // Tên collection cho cuộc trò chuyện trên Facebook
	FbMessages      string // Tên collection cho metadata tin nhắn trên Facebook
	FbMessageItems  string // Tên collection cho từng message riêng lẻ trên Facebook
	FbPosts         string // Tên collection cho bài viết trên Facebook
	FbCustomers     string // Tên collection cho khách hàng từ Facebook (Pancake)
	PcOrders        string // Tên collection cho đơn hàng trên PanCake
	PcPosCustomers  string // Tên collection cho khách hàng từ Pancake POS
	PcPosShops      string // Tên collection cho cửa hàng từ Pancake POS API
	PcPosWarehouses string // Tên collection cho kho hàng từ Pancake POS API
	PcPosProducts   string // Tên collection cho sản phẩm từ Pancake POS API
	PcPosVariations string // Tên collection cho biến thể sản phẩm từ Pancake POS API
	PcPosCategories string // Tên collection cho danh mục sản phẩm từ Pancake POS API
	PcPosOrders     string // Tên collection cho đơn hàng từ Pancake POS API

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
