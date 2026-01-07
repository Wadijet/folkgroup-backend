package global

import (
	"database/sql"
	"meta_commerce/config"
	"meta_commerce/core/registry"

	_ "github.com/go-sql-driver/mysql"
	"go.mongodb.org/mongo-driver/mongo"
	validator "gopkg.in/go-playground/validator.v9"
)

// MongoDB_Auth_CollectionName chứa tên các collection trong MongoDB
type MongoDB_Auth_CollectionName struct {
	Users           string // Tên collection cho người dùng
	Permissions     string // Tên collection cho quyền
	Roles           string // Tên collection cho vai trò
	RolePermissions string // Tên collection cho vai trò và quyền
	UserRoles       string // Tên collection cho người dùng và vai trò
	Organizations   string // Tên collection cho tổ chức
	Agents          string // Tên collection cho bot
	AccessTokens    string // Tên collection cho token
	FbPages         string // Tên collection cho trang Facebook
	FbConvesations  string // Tên collection cho cuộc trò chuyện trên Facebook
	FbMessages      string // Tên collection cho metadata tin nhắn trên Facebook
	FbMessageItems  string // Tên collection cho từng message riêng lẻ trên Facebook
	FbPosts         string // Tên collection cho bài viết trên Facebook
	FbCustomers     string // Tên collection cho khách hàng từ Facebook (Pancake)
	PcOrders        string // Tên collection cho đơn hàng trên PanCake
	Customers       string // Tên collection cho khách hàng (deprecated - dùng FbCustomers và PcPosCustomers)
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
	DeliveryHistory  string // Tên collection cho delivery history (đổi từ notification_history)

	// CTA Module Collections
	CTALibrary  string // Tên collection cho CTA library
	CTATracking string // Tên collection cho CTA tracking

	// Agent Management System Collections (Bot Management)
	AgentRegistry    string // Tên collection cho agent registry (đã ghép với agent_status)
	AgentConfigs     string // Tên collection cho agent configs
	AgentCommands    string // Tên collection cho agent commands
	AgentActivityLogs string // Tên collection cho agent activity logs
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
