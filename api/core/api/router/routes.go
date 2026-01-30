package router

import (
	"fmt"
	"meta_commerce/core/api/handler"
	"meta_commerce/core/api/middleware"
	"meta_commerce/core/api/services"

	"github.com/gofiber/fiber/v3"
)

// ============================================================================
// ⚠️ QUAN TRỌNG: BUG FIBER V3 - CÁCH ĐĂNG KÝ MIDDLEWARE
// ============================================================================
//
// Fiber v3 có BUG nghiêm trọng với cách đăng ký middleware trực tiếp trong route.
// Middleware sẽ KHÔNG được gọi nếu dùng cách trực tiếp!
//
// ❌ CÁCH SAI (KHÔNG HOẠT ĐỘNG):
//    router.Get("/path", middleware.AuthMiddleware(""), handler)
//    router.Post("/path", middleware.AuthMiddleware(""), handler)
//    → Middleware sẽ KHÔNG được gọi, request sẽ bỏ qua middleware!
//
// ✅ CÁCH ĐÚNG (PHẢI DÙNG):
//    authMiddleware := middleware.AuthMiddleware("")
//    registerRouteWithMiddleware(router, "/prefix", "GET", "/path", []fiber.Handler{authMiddleware}, handler)
//    → Middleware sẽ được gọi đúng cách thông qua .Use() method
//
// 📝 LỊCH SỬ:
//    - Ngày: 2025-12-28
//    - Vấn đề: Endpoint /api/v1/auth/roles trả về 401 mặc dù token hợp lệ
//    - Nguyên nhân: Dùng cách trực tiếp router.Get(path, middleware, handler)
//    - Giải pháp: Đã test 7 cách khác nhau, chỉ có registerRouteWithMiddleware hoạt động
//    - Kết quả: Đã sửa tất cả 21 routes trong file này
//
// 📚 TÀI LIỆU:
//    - Xem chi tiết: docs/06-testing/fiber-v3-middleware-registration.md
//    - Hàm đúng: registerRouteWithMiddleware() (dòng 159-195)
//
// 🔍 KIỂM TRA:
//    Nếu thấy route nào dùng cách trực tiếp router.Get/Post/Put/Delete(path, middleware, handler)
//    → PHẢI SỬA NGAY thành registerRouteWithMiddleware!
//
// ============================================================================

// CONFIGS

// CRUDHandler định nghĩa interface cho các handler CRUD
type CRUDHandler interface {
	// Create
	InsertOne(c fiber.Ctx) error
	InsertMany(c fiber.Ctx) error

	// Read
	Find(c fiber.Ctx) error
	FindOne(c fiber.Ctx) error
	FindOneById(c fiber.Ctx) error
	FindManyByIds(c fiber.Ctx) error
	FindWithPagination(c fiber.Ctx) error

	// Update
	UpdateOne(c fiber.Ctx) error
	UpdateMany(c fiber.Ctx) error
	UpdateById(c fiber.Ctx) error
	FindOneAndUpdate(c fiber.Ctx) error

	// Delete
	DeleteOne(c fiber.Ctx) error
	DeleteMany(c fiber.Ctx) error
	DeleteById(c fiber.Ctx) error
	FindOneAndDelete(c fiber.Ctx) error

	// Other
	CountDocuments(c fiber.Ctx) error
	Distinct(c fiber.Ctx) error
	Upsert(c fiber.Ctx) error
	UpsertMany(c fiber.Ctx) error
	DocumentExists(c fiber.Ctx) error
}

// Router quản lý việc định tuyến cho API
type Router struct {
	app *fiber.App
}

// CRUDConfig cấu hình các operation được phép cho mỗi collection
type CRUDConfig struct {
	// Create
	InsOne  bool // Insert One
	InsMany bool // Insert Many

	// Read
	Find     bool // Find All
	FindOne  bool // Find One
	FindById bool // Find By Id
	FindIds  bool // Find Many By Ids
	Paginate bool // Find With Pagination

	// Update
	UpdOne  bool // Update One
	UpdMany bool // Update Many
	UpdById bool // Update By Id
	FindUpd bool // Find One And Update

	// Delete
	DelOne  bool // Delete One
	DelMany bool // Delete Many
	DelById bool // Delete By Id
	FindDel bool // Find One And Delete

	// Other
	Count    bool // Count Documents
	Distinct bool // Distinct
	Upsert   bool // Upsert One
	UpsMany  bool // Upsert Many
	Exists   bool // Document Exists
}

// Config cho từng collection
var (
	readOnlyConfig = CRUDConfig{
		InsOne: false, InsMany: false,
		Find: true, FindOne: true, FindById: true,
		FindIds: true, Paginate: true,
		UpdOne: false, UpdMany: false, UpdById: false,
		FindUpd: false,
		DelOne:  false, DelMany: false, DelById: false,
		FindDel: false,
		Count:   true, Distinct: true,
		Upsert: false, UpsMany: false, Exists: true,
	}

	readWriteConfig = CRUDConfig{
		InsOne: true, InsMany: true,
		Find: true, FindOne: true, FindById: true,
		FindIds: true, Paginate: true,
		UpdOne: true, UpdMany: true, UpdById: true,
		FindUpd: true,
		DelOne:  true, DelMany: true, DelById: true,
		FindDel: true,
		Count:   true, Distinct: true,
		Upsert: true, UpsMany: true, Exists: true,
	}

	// Auth Module Collections
	userConfig              = readOnlyConfig
	permConfig              = readOnlyConfig
	roleConfig              = readWriteConfig
	rolePermConfig          = readWriteConfig
	userRoleConfig          = readWriteConfig
	organizationShareConfig = readWriteConfig

	// Pancake Module Collections
	accessTokenConfig   = readWriteConfig
	fbPageConfig        = readWriteConfig
	fbPostConfig        = readWriteConfig
	fbConvConfig        = readWriteConfig
	fbMessageConfig     = readWriteConfig
	fbMessageItemConfig = readWriteConfig
	pcOrderConfig       = readWriteConfig

	// Notification Module Collections
	notificationSenderConfig   = readWriteConfig
	notificationChannelConfig  = readWriteConfig
	notificationTemplateConfig = readWriteConfig
	notificationRoutingConfig  = readWriteConfig
	notificationHistoryConfig  = readOnlyConfig // History chỉ đọc

	// Webhook Logs Module Collections
	webhookLogConfig = readWriteConfig // Webhook logs có thể xem, tạo, sửa, xóa để debug
)

// RoutePrefix chứa các prefix cơ bản cho API
type RoutePrefix struct {
	Base string // Prefix cơ bản (/api)
	V1   string // Prefix cho API version 1 (/api/v1)
}

// NewRoutePrefix tạo mới một instance của RoutePrefix với các giá trị mặc định
func NewRoutePrefix() RoutePrefix {
	base := "/api"
	return RoutePrefix{
		Base: base,
		V1:   base + "/v1",
	}
}

// NewRouter tạo mới một instance của Router
func NewRouter(app *fiber.App) *Router {
	return &Router{
		app: app,
	}
}

// registerRouteWithMiddleware đăng ký route với middleware sử dụng .Use() method (cách đúng theo Fiber v3)
//
// ⚠️ QUAN TRỌNG: Đây là CÁCH DUY NHẤT hoạt động đúng trong Fiber v3!
//
// ❌ KHÔNG DÙNG cách trực tiếp: router.Get(path, middleware, handler) - middleware sẽ KHÔNG được gọi!
// ✅ PHẢI DÙNG cách này: registerRouteWithMiddleware với .Use() method
//
// Lịch sử: Đã test 7 cách khác nhau (2025-12-28) và chỉ có cách này hoạt động.
// Xem thêm: docs/06-testing/fiber-v3-middleware-registration.md
//
// Ví dụ sử dụng:
//
//	authMiddleware := middleware.AuthMiddleware("")
//	registerRouteWithMiddleware(router, "/auth", "GET", "/roles", []fiber.Handler{authMiddleware}, handler)
func registerRouteWithMiddleware(router fiber.Router, prefix string, method string, path string, middlewares []fiber.Handler, handler fiber.Handler) {
	// Tạo group với prefix, middleware sẽ chỉ áp dụng cho routes trong group này
	routeGroup := router.Group(prefix)
	for _, mw := range middlewares {
		routeGroup.Use(mw) // ← ĐÂY LÀ CÁCH ĐÚNG - dùng .Use() thay vì truyền trực tiếp
	}

	// Đăng ký route với path tương đối (không có prefix vì đã có trong group)
	switch method {
	case "GET":
		routeGroup.Get(path, handler)
	case "POST":
		routeGroup.Post(path, handler)
	case "PUT":
		routeGroup.Put(path, handler)
	case "DELETE":
		routeGroup.Delete(path, handler)
	}
}

// registerCRUDRoutes đăng ký các route CRUD cho một collection
//
// ⚠️ LƯU Ý: Hàm này đã dùng registerRouteWithMiddleware (cách đúng), không cần sửa.
// Nếu thêm route mới bên ngoài hàm này, PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func (r *Router) registerCRUDRoutes(router fiber.Router, prefix string, h CRUDHandler, config CRUDConfig, permissionPrefix string) {
	// Tạo middleware chain: AuthMiddleware → OrganizationContextMiddleware
	// Đã tắt log để giảm log khi khởi động
	authMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Insert")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	authReadMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Read")
	authUpdateMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Update")
	authDeleteMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Delete")

	// Create operations
	if config.InsOne {
		registerRouteWithMiddleware(router, prefix, "POST", "/insert-one", []fiber.Handler{authMiddleware, orgContextMiddleware}, h.InsertOne)
	}
	if config.InsMany {
		registerRouteWithMiddleware(router, prefix, "POST", "/insert-many", []fiber.Handler{authMiddleware, orgContextMiddleware}, h.InsertMany)
	}

	// Read operations
	if config.Find {
		registerRouteWithMiddleware(router, prefix, "GET", "/find", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.Find)
	}
	if config.FindOne {
		registerRouteWithMiddleware(router, prefix, "GET", "/find-one", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.FindOne)
	}
	if config.FindById {
		registerRouteWithMiddleware(router, prefix, "GET", "/find-by-id/:id", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.FindOneById)
	}
	if config.FindIds {
		registerRouteWithMiddleware(router, prefix, "POST", "/find-by-ids", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.FindManyByIds)
	}
	if config.Paginate {
		registerRouteWithMiddleware(router, prefix, "GET", "/find-with-pagination", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.FindWithPagination)
	}

	// Update operations
	if config.UpdOne {
		registerRouteWithMiddleware(router, prefix, "PUT", "/update-one", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.UpdateOne)
	}
	if config.UpdMany {
		registerRouteWithMiddleware(router, prefix, "PUT", "/update-many", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.UpdateMany)
	}
	if config.UpdById {
		registerRouteWithMiddleware(router, prefix, "PUT", "/update-by-id/:id", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.UpdateById)
	}
	if config.FindUpd {
		registerRouteWithMiddleware(router, prefix, "PUT", "/find-one-and-update", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.FindOneAndUpdate)
	}

	// Delete operations
	if config.DelOne {
		registerRouteWithMiddleware(router, prefix, "DELETE", "/delete-one", []fiber.Handler{authDeleteMiddleware, orgContextMiddleware}, h.DeleteOne)
	}
	if config.DelMany {
		registerRouteWithMiddleware(router, prefix, "DELETE", "/delete-many", []fiber.Handler{authDeleteMiddleware, orgContextMiddleware}, h.DeleteMany)
	}
	if config.DelById {
		registerRouteWithMiddleware(router, prefix, "DELETE", "/delete-by-id/:id", []fiber.Handler{authDeleteMiddleware, orgContextMiddleware}, h.DeleteById)
	}
	if config.FindDel {
		registerRouteWithMiddleware(router, prefix, "DELETE", "/find-one-and-delete", []fiber.Handler{authDeleteMiddleware, orgContextMiddleware}, h.FindOneAndDelete)
	}

	// Other operations
	if config.Count {
		// Count chỉ cần đăng nhập, không cần permission cụ thể
		authOnlyMiddleware := middleware.AuthMiddleware("")
		registerRouteWithMiddleware(router, prefix, "GET", "/count", []fiber.Handler{authOnlyMiddleware}, h.CountDocuments)
	}
	if config.Distinct {
		registerRouteWithMiddleware(router, prefix, "GET", "/distinct", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.Distinct)
	}
	if config.Upsert {
		registerRouteWithMiddleware(router, prefix, "POST", "/upsert-one", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.Upsert)
	}
	if config.UpsMany {
		registerRouteWithMiddleware(router, prefix, "POST", "/upsert-many", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.UpsertMany)
	}
	if config.Exists {
		registerRouteWithMiddleware(router, prefix, "GET", "/exists", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.DocumentExists)
	}
}

// CÁC HÀM ĐĂNG KÝ ROUTES

// registerAdminRoutes đăng ký các route cho admin operations
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func registerAdminRoutes(router fiber.Router) error {
	// Admin routes
	adminHandler, err := handler.NewAdminHandler()
	if err != nil {
		return fmt.Errorf("failed to create admin handler: %v", err)
	}

	// Các route đặc biệt cho quản trị viên
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	blockMiddleware := middleware.AuthMiddleware("User.Block")
	registerRouteWithMiddleware(router, "/admin/user", "POST", "/block", []fiber.Handler{blockMiddleware}, adminHandler.HandleBlockUser)
	registerRouteWithMiddleware(router, "/admin/user", "POST", "/unblock", []fiber.Handler{blockMiddleware}, adminHandler.HandleUnBlockUser)

	setRoleMiddleware := middleware.AuthMiddleware("User.SetRole")
	registerRouteWithMiddleware(router, "/admin/user", "POST", "/role", []fiber.Handler{setRoleMiddleware}, adminHandler.HandleSetRole)

	// Thiết lập administrator (yêu cầu quyền Init.SetAdmin)
	setAdminMiddleware := middleware.AuthMiddleware("Init.SetAdmin")
	registerRouteWithMiddleware(router, "/admin/user", "POST", "/set-administrator/:id", []fiber.Handler{setAdminMiddleware}, adminHandler.HandleAddAdministrator)
	// Đồng bộ quyền cho Administrator (yêu cầu quyền Init.SetAdmin)
	registerRouteWithMiddleware(router, "/admin", "POST", "/sync-administrator-permissions", []fiber.Handler{setAdminMiddleware}, adminHandler.HandleSyncAdministratorPermissions)

	return nil
}

// registerSystemRoutes đăng ký các route cho system operations
func registerSystemRoutes(router fiber.Router) error {
	// Khởi tạo SystemHandler
	systemHandler, err := handler.NewSystemHandler()
	if err != nil {
		return fmt.Errorf("failed to create system handler: %v", err)
	}

	// System routes
	router.Get("/system/health", systemHandler.HandleHealth)

	return nil
}

// registerAuthRoutes đăng ký các route cho authentication cá nhân
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func (r *Router) registerAuthRoutes(router fiber.Router) error {
	// User routes
	userHandler, err := handler.NewUserHandler()
	if err != nil {
		return fmt.Errorf("failed to create user handler: %v", err)
	}

	// Các route xác thực cá nhân
	// Firebase Authentication - Nhận Firebase ID token và tạo JWT
	router.Post("/auth/login/firebase", userHandler.HandleLoginWithFirebase)

	// Logout - Xóa JWT token
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	authOnlyMiddleware := middleware.AuthMiddleware("")
	registerRouteWithMiddleware(router, "/auth", "POST", "/logout", []fiber.Handler{authOnlyMiddleware}, userHandler.HandleLogout)

	// Profile - Lấy và cập nhật thông tin user
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	registerRouteWithMiddleware(router, "/auth", "GET", "/profile", []fiber.Handler{authOnlyMiddleware}, userHandler.HandleGetProfile)
	registerRouteWithMiddleware(router, "/auth", "PUT", "/profile", []fiber.Handler{authOnlyMiddleware}, userHandler.HandleUpdateProfile)

	// Roles - Lấy danh sách tất cả roles của user hiện tại
	// Endpoint đặc biệt: Có xác thực (cần token) nhưng KHÔNG yêu cầu permission
	// Mục đích: Cho phép user xem tất cả roles của mình để chọn context làm việc
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng đã test) thay vì cách trực tiếp có bug trong Fiber v3
	authRolesMiddleware := middleware.AuthMiddleware("")
	registerRouteWithMiddleware(router, "/auth", "GET", "/roles", []fiber.Handler{authRolesMiddleware}, userHandler.HandleGetUserRoles)

	return nil
}

// registerRBACRoutes đăng ký các route cho Role-Based Access Control
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func (r *Router) registerRBACRoutes(router fiber.Router) error {
	// User routes (Quản lý người dùng)
	userHandler, err := handler.NewUserHandler()
	if err != nil {
		return fmt.Errorf("failed to create user handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/user", userHandler, userConfig, "User")

	// Permission routes
	permHandler, err := handler.NewPermissionHandler()
	if err != nil {
		return fmt.Errorf("failed to create permission handler: %v", err)
	}
	// CRUD routes - có thể dùng filter để lấy permissions theo category/group
	// Ví dụ: GET /api/v1/permission/find?filter={"category":"..."} hoặc filter={"group":"..."}
	r.registerCRUDRoutes(router, "/permission", permHandler, permConfig, "Permission")

	// Role routes
	roleHandler, err := handler.NewRoleHandler()
	if err != nil {
		return fmt.Errorf("failed to create role handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/role", roleHandler, roleConfig, "Role")

	// RolePermission routes
	rolePermHandler, err := handler.NewRolePermissionHandler()
	if err != nil {
		return fmt.Errorf("failed to create role permission handler: %v", err)
	}
	// Route đặc biệt cho cập nhật quyền của vai trò
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	rolePermUpdateMiddleware := middleware.AuthMiddleware("RolePermission.Update")
	registerRouteWithMiddleware(router, "/role-permission", "PUT", "/update-role", []fiber.Handler{rolePermUpdateMiddleware}, rolePermHandler.HandleUpdateRolePermissions)
	// CRUD routes
	r.registerCRUDRoutes(router, "/role-permission", rolePermHandler, rolePermConfig, "RolePermission")

	// UserRole routes
	userRoleHandler, err := handler.NewUserRoleHandler()
	if err != nil {
		return fmt.Errorf("failed to create user role handler: %v", err)
	}
	// Route đặc biệt cho cập nhật vai trò của người dùng
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	userRoleUpdateMiddleware := middleware.AuthMiddleware("UserRole.Update")
	registerRouteWithMiddleware(router, "/user-role", "PUT", "/update-user-roles", []fiber.Handler{userRoleUpdateMiddleware}, userRoleHandler.HandleUpdateUserRoles)
	// CRUD routes
	r.registerCRUDRoutes(router, "/user-role", userRoleHandler, userRoleConfig, "UserRole")

	// Organization routes
	organizationHandler, err := handler.NewOrganizationHandler()
	if err != nil {
		return fmt.Errorf("failed to create organization handler: %v", err)
	}
	// Đã tắt log để giảm log khi khởi động
	r.registerCRUDRoutes(router, "/organization", organizationHandler, readWriteConfig, "Organization")

	// Organization Share routes - dùng CRUD chuẩn
	// Logic nghiệp vụ (duplicate check, validation) đã được đưa vào service.InsertOne override
	organizationShareHandler, err := handler.NewOrganizationShareHandler()
	if err != nil {
		return fmt.Errorf("failed to create organization share handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/organization-share", organizationShareHandler, organizationShareConfig, "OrganizationShare")

	return nil
}

// registerFacebookRoutes đăng ký các route cho Facebook integration
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func (r *Router) registerFacebookRoutes(router fiber.Router) error {
	// Access Token routes
	accessTokenHandler, err := handler.NewAccessTokenHandler()
	if err != nil {
		return fmt.Errorf("failed to create access token handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/access-token", accessTokenHandler, accessTokenConfig, "AccessToken")

	// Facebook Page routes
	fbPageHandler, err := handler.NewFbPageHandler()
	if err != nil {
		return fmt.Errorf("failed to create facebook page handler: %v", err)
	}
	// Route đặc biệt cho tìm page theo PageID
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	fbPageReadMiddleware := middleware.AuthMiddleware("FbPage.Read")
	fbPageUpdateMiddleware := middleware.AuthMiddleware("FbPage.Update")
	registerRouteWithMiddleware(router, "/facebook/page", "GET", "/find-by-page-id/:id", []fiber.Handler{fbPageReadMiddleware}, fbPageHandler.HandleFindOneByPageID)
	// Route đặc biệt cho cập nhật token của page
	registerRouteWithMiddleware(router, "/facebook/page", "PUT", "/update-token", []fiber.Handler{fbPageUpdateMiddleware}, fbPageHandler.HandleUpdateToken)
	// CRUD routes
	r.registerCRUDRoutes(router, "/facebook/page", fbPageHandler, fbPageConfig, "FbPage")

	// Facebook Post routes
	fbPostHandler, err := handler.NewFbPostHandler()
	if err != nil {
		return fmt.Errorf("failed to create facebook post handler: %v", err)
	}
	// Route đặc biệt cho tìm post theo PostID
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	fbPostReadMiddleware := middleware.AuthMiddleware("FbPost.Read")
	registerRouteWithMiddleware(router, "/facebook/post", "GET", "/find-by-post-id/:id", []fiber.Handler{fbPostReadMiddleware}, fbPostHandler.HandleFindOneByPostID)

	// CRUD routes
	r.registerCRUDRoutes(router, "/facebook/post", fbPostHandler, fbPostConfig, "FbPost")

	// Facebook Conversation routes
	fbConvHandler, err := handler.NewFbConversationHandler()
	if err != nil {
		return fmt.Errorf("failed to create facebook conversation handler: %v", err)
	}
	// Route đặc biệt cho lấy cuộc trò chuyện sắp xếp theo thời gian cập nhật API
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	fbConvReadMiddleware := middleware.AuthMiddleware("FbConversation.Read")
	registerRouteWithMiddleware(router, "/facebook/conversation", "GET", "/sort-by-api-update", []fiber.Handler{fbConvReadMiddleware}, fbConvHandler.HandleFindAllSortByApiUpdate)
	// CRUD routes
	r.registerCRUDRoutes(router, "/facebook/conversation", fbConvHandler, fbConvConfig, "FbConversation")

	// Facebook Message routes
	fbMessageHandler, err := handler.NewFbMessageHandler()
	if err != nil {
		return fmt.Errorf("failed to create facebook message handler: %v", err)
	}

	// ============================================
	// ENDPOINT ĐẶC BIỆT: Upsert Messages (Tách biệt với CRUD)
	// ============================================
	// Endpoint này xử lý logic đặc biệt: tự động tách messages[] ra khỏi panCakeData
	// và lưu vào 2 collections (fb_messages cho metadata, fb_message_items cho messages)
	// Route: POST /api/v1/facebook/message/upsert-messages
	// DTO: FbMessageUpsertMessagesInput (có field HasMore)
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	fbMessageUpdateMiddleware := middleware.AuthMiddleware("FbMessage.Update")
	registerRouteWithMiddleware(router, "/facebook/message", "POST", "/upsert-messages", []fiber.Handler{fbMessageUpdateMiddleware}, fbMessageHandler.HandleUpsertMessages)

	// ============================================
	// CRUD ROUTES: Giữ nguyên logic chung (không tách messages)
	// ============================================
	// Các endpoint CRUD (insert-one, update-one, find, delete, ...) hoạt động bình thường
	// - Không có logic tách messages
	// - PanCakeData có thể chứa messages[] (tương thích ngược)
	// - DTO: FbMessageCreateInput (không có field HasMore)
	r.registerCRUDRoutes(router, "/facebook/message", fbMessageHandler, fbMessageConfig, "FbMessage")

	// Facebook Message Item routes
	fbMessageItemHandler, err := handler.NewFbMessageItemHandler()
	if err != nil {
		return fmt.Errorf("failed to create facebook message item handler: %v", err)
	}
	// Route đặc biệt cho lấy message items theo conversationId với phân trang
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	fbMessageItemReadMiddleware := middleware.AuthMiddleware("FbMessageItem.Read")
	registerRouteWithMiddleware(router, "/facebook/message-item", "GET", "/find-by-conversation/:conversationId", []fiber.Handler{fbMessageItemReadMiddleware}, fbMessageItemHandler.HandleFindByConversationId)
	// Route đặc biệt cho tìm message item theo messageId
	registerRouteWithMiddleware(router, "/facebook/message-item", "GET", "/find-by-message-id/:messageId", []fiber.Handler{fbMessageItemReadMiddleware}, fbMessageItemHandler.HandleFindOneByMessageId)
	// CRUD routes
	r.registerCRUDRoutes(router, "/facebook/message-item", fbMessageItemHandler, fbMessageItemConfig, "FbMessageItem")

	// Pancake Order routes
	pcOrderHandler, err := handler.NewPcOrderHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake order handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/pancake/order", pcOrderHandler, pcOrderConfig, "PcOrder")

	// Pancake Webhook routes (public, không cần auth - Pancake gọi trực tiếp)
	pancakeWebhookHandler, err := handler.NewPancakeWebhookHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake webhook handler: %v", err)
	}
	// Webhook endpoint không cần authentication middleware
	router.Post("/pancake/webhook", pancakeWebhookHandler.HandlePancakeWebhook)

	// Pancake POS Webhook routes (public, không cần auth - Pancake POS gọi trực tiếp)
	pancakePosWebhookHandler, err := handler.NewPancakePosWebhookHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake pos webhook handler: %v", err)
	}
	// Webhook endpoint không cần authentication middleware
	router.Post("/pancake-pos/webhook", pancakePosWebhookHandler.HandlePancakePosWebhook)

	// Webhook Log CRUD routes (cần auth - để admin xem và debug webhooks)
	webhookLogHandler, err := handler.NewWebhookLogHandler()
	if err != nil {
		return fmt.Errorf("failed to create webhook log handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/webhook-log", webhookLogHandler, webhookLogConfig, "WebhookLog")

	// Facebook Customer routes
	fbCustomerHandler, err := handler.NewFbCustomerHandler()
	if err != nil {
		return fmt.Errorf("failed to create fb customer handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/fb-customer", fbCustomerHandler, readWriteConfig, "FbCustomer")

	// Pancake POS Customer routes
	pcPosCustomerHandler, err := handler.NewPcPosCustomerHandler()
	if err != nil {
		return fmt.Errorf("failed to create pc pos customer handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/pc-pos-customer", pcPosCustomerHandler, readWriteConfig, "PcPosCustomer")

	// Pancake POS Shop routes
	pcPosShopHandler, err := handler.NewPcPosShopHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake pos shop handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/pancake-pos/shop", pcPosShopHandler, readWriteConfig, "PcPosShop")

	// Pancake POS Warehouse routes
	pcPosWarehouseHandler, err := handler.NewPcPosWarehouseHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake pos warehouse handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/pancake-pos/warehouse", pcPosWarehouseHandler, readWriteConfig, "PcPosWarehouse")

	// Pancake POS Product routes
	pcPosProductHandler, err := handler.NewPcPosProductHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake pos product handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/pancake-pos/product", pcPosProductHandler, readWriteConfig, "PcPosProduct")

	// Pancake POS Variation routes
	pcPosVariationHandler, err := handler.NewPcPosVariationHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake pos variation handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/pancake-pos/variation", pcPosVariationHandler, readWriteConfig, "PcPosVariation")

	// Pancake POS Category routes
	pcPosCategoryHandler, err := handler.NewPcPosCategoryHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake pos category handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/pancake-pos/category", pcPosCategoryHandler, readWriteConfig, "PcPosCategory")

	// Pancake POS Order routes
	pcPosOrderHandler, err := handler.NewPcPosOrderHandler()
	if err != nil {
		return fmt.Errorf("failed to create pancake pos order handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/pancake-pos/order", pcPosOrderHandler, readWriteConfig, "PcPosOrder")

	return nil
}

// registerInitRoutes đăng ký các route cho khởi tạo hệ thống
func (r *Router) registerInitRoutes(router fiber.Router) error {
	// Kiểm tra xem đã có admin chưa
	// Nếu đã có admin, không đăng ký bất kỳ init endpoint nào (tối ưu hiệu suất và bảo mật)
	initService, err := services.NewInitService()
	if err == nil {
		hasAdmin, err := initService.HasAnyAdministrator()
		if err == nil && hasAdmin {
			// Đã có admin, không đăng ký bất kỳ init endpoint nào
			// Endpoint thêm admin sẽ ở /admin/user/set-administrator/:id
			return nil
		}
	}

	// Chưa có admin, đăng ký tất cả init endpoints
	initHandler, err := handler.NewInitHandler()
	if err != nil {
		return fmt.Errorf("failed to create init handler: %v", err)
	}

	// Route kiểm tra trạng thái init (chỉ khi chưa có admin)
	router.Get("/init/status", initHandler.HandleInitStatus)

	// Các route khởi tạo các đơn vị cơ bản
	router.Post("/init/organization", initHandler.HandleInitOrganization)
	router.Post("/init/permissions", initHandler.HandleInitPermissions)
	router.Post("/init/roles", initHandler.HandleInitRoles)
	router.Post("/init/admin-user", initHandler.HandleInitAdminUser)
	router.Post("/init/all", initHandler.HandleInitAll) // One-click setup

	// Route thiết lập administrator lần đầu (chưa có admin, không cần quyền)
	// Handler sẽ tự check xem đã có admin chưa
	router.Post("/init/set-administrator/:id", initHandler.HandleSetAdministrator)

	return nil
}

// registerNotificationRoutes đăng ký các route cho Notification Module
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func (r *Router) registerNotificationRoutes(router fiber.Router) error {
	// Notification Sender routes
	senderHandler, err := handler.NewNotificationSenderHandler()
	if err != nil {
		return fmt.Errorf("failed to create notification sender handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/notification/sender", senderHandler, notificationSenderConfig, "NotificationSender")

	// Notification Channel routes
	channelHandler, err := handler.NewNotificationChannelHandler()
	if err != nil {
		return fmt.Errorf("failed to create notification channel handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/notification/channel", channelHandler, notificationChannelConfig, "NotificationChannel")

	// Notification Template routes
	templateHandler, err := handler.NewNotificationTemplateHandler()
	if err != nil {
		return fmt.Errorf("failed to create notification template handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/notification/template", templateHandler, notificationTemplateConfig, "NotificationTemplate")

	// Notification Routing Rule routes
	routingHandler, err := handler.NewNotificationRoutingHandler()
	if err != nil {
		return fmt.Errorf("failed to create notification routing handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/notification/routing", routingHandler, notificationRoutingConfig, "NotificationRouting")

	// Notification History routes (read-only)
	historyHandler, err := handler.NewNotificationHistoryHandler()
	if err != nil {
		return fmt.Errorf("failed to create notification history handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/notification/history", historyHandler, notificationHistoryConfig, "DeliveryHistory")

	// Notification Trigger route
	triggerHandler, err := handler.NewNotificationTriggerHandler()
	if err != nil {
		return fmt.Errorf("failed to create notification trigger handler: %v", err)
	}
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	notificationTriggerMiddleware := middleware.AuthMiddleware("Notification.Trigger")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	registerRouteWithMiddleware(router, "/notification", "POST", "/trigger", []fiber.Handler{notificationTriggerMiddleware, orgContextMiddleware}, triggerHandler.HandleTriggerNotification)

	// Tracking routes (public, không cần auth) - gộp tất cả tracking actions vào 1 endpoint
	// Format: /api/v1/track/:action/:historyId?ctaIndex=...
	// Actions: "open", "click", "confirm", "cta"
	// - "open": Track email open (không cần ctaIndex) - trả về 1x1 PNG pixel
	// - "click": Track notification click (cần ctaIndex trong query) - redirect về original URL
	// - "confirm": Track notification confirm (không cần ctaIndex) - trả về JSON
	// - "cta": Track CTA click (cần ctaIndex trong query) - redirect về original URL
	trackingHandler, err := handler.NewTrackingHandler()
	if err != nil {
		return fmt.Errorf("failed to create tracking handler: %v", err)
	}
	// Chỉ 1 route duy nhất, ctaIndex lấy từ query param
	router.Get("/track/:action/:historyId", trackingHandler.HandleAction)

	return nil
}

// registerCTARoutes đăng ký các route cho CTA Module
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func (r *Router) registerCTARoutes(router fiber.Router) error {
	// CTA Library routes (CRUD) - dùng CRUD standard
	ctaLibraryHandler, err := handler.NewCTALibraryHandler()
	if err != nil {
		return fmt.Errorf("failed to create CTA library handler: %v", err)
	}
	// Sử dụng readWriteConfig cho CTA Library
	ctaLibraryConfig := readWriteConfig
	r.registerCRUDRoutes(router, "/cta/library", ctaLibraryHandler, ctaLibraryConfig, "CTALibrary")

	// CTA Action route đã được gộp vào /api/v1/track/:action/:historyId với action="cta"

	// Lưu ý: CTA Render không có endpoint riêng vì được gọi trực tiếp từ code (internal)
	// Hệ thống 1 và 2 sẽ gọi trực tiếp cta.Renderer.RenderCTAs() thay vì qua HTTP

	return nil
}

// registerDeliveryRoutes đăng ký các route cho Delivery Module (Hệ thống 1)
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func (r *Router) registerDeliveryRoutes(router fiber.Router) error {
	// Delivery Send route (gửi notification trực tiếp)
	sendHandler, err := handler.NewDeliverySendHandler()
	if err != nil {
		return fmt.Errorf("failed to create delivery send handler: %v", err)
	}
	sendMiddleware := middleware.AuthMiddleware("Delivery.Send")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	registerRouteWithMiddleware(router, "/delivery", "POST", "/send", []fiber.Handler{sendMiddleware, orgContextMiddleware}, sendHandler.HandleSend)

	// Delivery History routes (read-only)
	// Lưu ý: History thuộc Delivery System (cùng với Queue), nên đặt endpoint trong /delivery namespace
	// để nhất quán với model DeliveryHistory và collection delivery_history
	historyHandler, err := handler.NewNotificationHistoryHandler()
	if err != nil {
		return fmt.Errorf("failed to create delivery history handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/delivery/history", historyHandler, notificationHistoryConfig, "DeliveryHistory")

	// Lưu ý: Delivery Sender và Tracking routes
	// - Sender: Dùng /notification/sender (cùng resource, thuộc Notification System)
	// - Tracking: Dùng /track/:action/:historyId/:ctaIndex? (unified tracking endpoint cho tất cả actions)

	return nil
}

// registerAgentManagementRoutes đăng ký các route cho Agent Management System (Bot Management)
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
func (r *Router) registerAgentManagementRoutes(router fiber.Router) error {
	// Agent Management Handler (chỉ cho check-in endpoint đặc biệt)
	agentManagementHandler, err := handler.NewAgentManagementHandler()
	if err != nil {
		return fmt.Errorf("failed to create agent management handler: %v", err)
	}

	// Enhanced Check-In endpoint (cần auth với permission AgentManagement.CheckIn)
	// Bot gửi check-in với thông tin chi tiết, server trả về commands và config updates
	checkInMiddleware := middleware.AuthMiddleware("AgentManagement.CheckIn")
	registerRouteWithMiddleware(router, "/agent-management", "POST", "/check-in", []fiber.Handler{checkInMiddleware}, agentManagementHandler.HandleEnhancedCheckIn)

	// Agent Registry CRUD routes
	agentRegistryHandler, err := handler.NewAgentRegistryHandler()
	if err != nil {
		return fmt.Errorf("failed to create agent registry handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/agent-management/registry", agentRegistryHandler, readWriteConfig, "AgentRegistry")

	// Agent Config CRUD routes
	agentConfigHandler, err := handler.NewAgentConfigHandler()
	if err != nil {
		return fmt.Errorf("failed to create agent config handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/agent-management/config", agentConfigHandler, readWriteConfig, "AgentConfig")

	// Agent Config Update Data endpoint (tạo version mới)
	// Endpoint riêng để update configData với versioning logic
	configUpdateMiddleware := middleware.AuthMiddleware("AgentConfig.Update")
	registerRouteWithMiddleware(router, "/agent-management/config", "PUT", "/:agentId/update-data", []fiber.Handler{configUpdateMiddleware}, agentConfigHandler.HandleUpdateConfigData)

	// Agent Command CRUD routes
	agentCommandHandler, err := handler.NewAgentCommandHandler()
	if err != nil {
		return fmt.Errorf("failed to create agent command handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/agent-management/command", agentCommandHandler, readWriteConfig, "AgentCommand")

	// Endpoint đặc biệt: Claim pending commands (atomic operation)
	claimAgentCommandsMiddleware := middleware.AuthMiddleware("AgentCommand.Update")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	registerRouteWithMiddleware(router, "/agent-management/command", "POST", "/claim-pending", []fiber.Handler{claimAgentCommandsMiddleware, orgContextMiddleware}, agentCommandHandler.ClaimPendingCommands)

	// Endpoint đặc biệt: Update heartbeat/progress (agent gọi định kỳ)
	updateAgentHeartbeatMiddleware := middleware.AuthMiddleware("AgentCommand.Update")
	registerRouteWithMiddleware(router, "/agent-management/command", "POST", "/update-heartbeat", []fiber.Handler{updateAgentHeartbeatMiddleware, orgContextMiddleware}, agentCommandHandler.UpdateHeartbeat)
	// Hỗ trợ cả URL params: /update-heartbeat/:commandId
	registerRouteWithMiddleware(router, "/agent-management/command", "POST", "/update-heartbeat/:commandId", []fiber.Handler{updateAgentHeartbeatMiddleware, orgContextMiddleware}, agentCommandHandler.UpdateHeartbeat)

	// Endpoint đặc biệt: Release stuck commands (admin/background job)
	releaseStuckAgentCommandsMiddleware := middleware.AuthMiddleware("AgentCommand.Update")
	registerRouteWithMiddleware(router, "/agent-management/command", "POST", "/release-stuck", []fiber.Handler{releaseStuckAgentCommandsMiddleware, orgContextMiddleware}, agentCommandHandler.ReleaseStuckCommands)

	// Lưu ý: Agent Status đã được ghép vào Agent Registry, không cần route riêng nữa
	// Status có thể được xem/update qua Agent Registry endpoints

	// Agent Activity Log CRUD routes (read-only cho admin, bot tự log qua check-in)
	agentActivityHandler, err := handler.NewAgentActivityLogHandler()
	if err != nil {
		return fmt.Errorf("failed to create agent activity handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/agent-management/activity", agentActivityHandler, readOnlyConfig, "AgentActivityLog")

	return nil
}

// registerContentStorageRoutes đăng ký các route cho Module 1: Content Storage
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
//
// Cấu trúc routes:
// - Production content: /api/v1/content/{nodes|videos|publications}/*
// - Drafts: /api/v1/content/drafts/{nodes|videos|publications}/*
// - Approve/Reject: POST /drafts/nodes/:id/approve|reject (với validation để bảo vệ luồng)
//
// Tất cả đều dùng prefix /content/ để tránh lẫn sang module khác (Module 2, Module 3)
func (r *Router) registerContentStorageRoutes(router fiber.Router) error {
	// ===== PRODUCTION CONTENT (đã được duyệt và commit) =====

	// Content Node CRUD routes (L1-L6) - collection: content_nodes
	contentNodeHandler, err := handler.NewContentNodeHandler()
	if err != nil {
		return fmt.Errorf("failed to create content node handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/content/nodes", contentNodeHandler, readWriteConfig, "ContentNodes")

	// Custom endpoint: GetTree (recursive tree)
	contentNodeReadMiddleware := middleware.AuthMiddleware("ContentNodes.Read")
	registerRouteWithMiddleware(router, "/content/nodes", "GET", "/tree/:id", []fiber.Handler{contentNodeReadMiddleware}, contentNodeHandler.GetTree)

	// Video CRUD routes (L7) - collection: content_videos
	videoHandler, err := handler.NewVideoHandler()
	if err != nil {
		return fmt.Errorf("failed to create video handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/content/videos", videoHandler, readWriteConfig, "ContentVideos")

	// Publication CRUD routes (L8) - collection: content_publications
	publicationHandler, err := handler.NewPublicationHandler()
	if err != nil {
		return fmt.Errorf("failed to create publication handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/content/publications", publicationHandler, readWriteConfig, "ContentPublications")

	// ===== DRAFTS (bản nháp chưa được duyệt) =====

	// Draft Content Node CRUD routes - collection: content_draft_nodes
	draftContentNodeHandler, err := handler.NewDraftContentNodeHandler()
	if err != nil {
		return fmt.Errorf("failed to create draft content node handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/content/drafts/nodes", draftContentNodeHandler, readWriteConfig, "ContentDraftNodes")

	// Custom endpoint: CommitDraftNode (commit draft → production)
	draftContentNodeCommitMiddleware := middleware.AuthMiddleware("ContentDraftNodes.Commit")
	registerRouteWithMiddleware(router, "/content/drafts/nodes", "POST", "/:id/commit", []fiber.Handler{draftContentNodeCommitMiddleware}, draftContentNodeHandler.CommitDraftNode)

	// Approve/Reject draft (với validation để bảo vệ luồng)
	draftApproveMiddleware := middleware.AuthMiddleware("ContentDraftNodes.Approve")
	draftRejectMiddleware := middleware.AuthMiddleware("ContentDraftNodes.Reject")
	registerRouteWithMiddleware(router, "/content/drafts/nodes", "POST", "/:id/approve", []fiber.Handler{draftApproveMiddleware}, draftContentNodeHandler.ApproveDraft)
	registerRouteWithMiddleware(router, "/content/drafts/nodes", "POST", "/:id/reject", []fiber.Handler{draftRejectMiddleware}, draftContentNodeHandler.RejectDraft)

	// Draft Video CRUD routes - collection: content_draft_videos
	draftVideoHandler, err := handler.NewDraftVideoHandler()
	if err != nil {
		return fmt.Errorf("failed to create draft video handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/content/drafts/videos", draftVideoHandler, readWriteConfig, "ContentDraftVideos")

	// Draft Publication CRUD routes - collection: content_draft_publications
	draftPublicationHandler, err := handler.NewDraftPublicationHandler()
	if err != nil {
		return fmt.Errorf("failed to create draft publication handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/content/drafts/publications", draftPublicationHandler, readWriteConfig, "ContentDraftPublications")

	return nil
}

// registerAIServiceRoutes đăng ký các route cho Module 2: AI Service
//
// ⚠️ LƯU Ý: Tất cả routes ở đây PHẢI dùng registerRouteWithMiddleware (xem comment ở đầu file)
//
// Cấu trúc routes:
// - Workflows: /api/v1/ai/workflows/*
// - Steps: /api/v1/ai/steps/*
// - Prompt Templates: /api/v1/ai/prompt-templates/*
// - Provider Profiles: /api/v1/ai/provider-profiles/*
// - Workflow Runs: /api/v1/ai/workflow-runs/*
// - Step Runs: /api/v1/ai/step-runs/*
// - Generation Batches: /api/v1/ai/generation-batches/*
// - Candidates: /api/v1/ai/candidates/*
// - AI Runs: /api/v1/ai/ai-runs/*
// - Workflow Commands: /api/v1/ai/workflow-commands/*
//
// Tất cả đều dùng prefix /api/v1/ai/ để phân biệt với Module 1 (/api/v1/content/)
func (r *Router) registerAIServiceRoutes(router fiber.Router) error {
	// ===== WORKFLOWS =====
	aiWorkflowHandler, err := handler.NewAIWorkflowHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI workflow handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/workflows", aiWorkflowHandler, readWriteConfig, "AIWorkflows")

	// ===== STEPS =====
	aiStepHandler, err := handler.NewAIStepHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI step handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/steps", aiStepHandler, readWriteConfig, "AISteps")
	// Custom endpoint: Render prompt cho step (bot gọi để lấy prompt đã render và AI config)
	authMiddleware := middleware.AuthMiddleware("AISteps.Read")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	registerRouteWithMiddleware(router, "/api/v2", "POST", "/ai/steps/:id/render-prompt", []fiber.Handler{authMiddleware, orgContextMiddleware}, aiStepHandler.RenderPrompt)

	// ===== PROMPT TEMPLATES =====
	aiPromptTemplateHandler, err := handler.NewAIPromptTemplateHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI prompt template handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/prompt-templates", aiPromptTemplateHandler, readWriteConfig, "AIPromptTemplates")

	// ===== PROVIDER PROFILES =====
	aiProviderProfileHandler, err := handler.NewAIProviderProfileHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI provider profile handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/provider-profiles", aiProviderProfileHandler, readWriteConfig, "AIProviderProfiles")

	// ===== WORKFLOW RUNS =====
	aiWorkflowRunHandler, err := handler.NewAIWorkflowRunHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI workflow run handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/workflow-runs", aiWorkflowRunHandler, readWriteConfig, "AIWorkflowRuns")

	// ===== STEP RUNS =====
	aiStepRunHandler, err := handler.NewAIStepRunHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI step run handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/step-runs", aiStepRunHandler, readWriteConfig, "AIStepRuns")

	// ===== GENERATION BATCHES =====
	aiGenerationBatchHandler, err := handler.NewAIGenerationBatchHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI generation batch handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/generation-batches", aiGenerationBatchHandler, readWriteConfig, "AIGenerationBatches")

	// ===== CANDIDATES =====
	aiCandidateHandler, err := handler.NewAICandidateHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI candidate handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/candidates", aiCandidateHandler, readWriteConfig, "AICandidates")

	// ===== AI RUNS =====
	aiRunHandler, err := handler.NewAIRunHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI run handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/ai-runs", aiRunHandler, readWriteConfig, "AIRuns")

	// ===== WORKFLOW COMMANDS =====
	aiWorkflowCommandHandler, err := handler.NewAIWorkflowCommandHandler()
	if err != nil {
		return fmt.Errorf("failed to create AI workflow command handler: %v", err)
	}
	r.registerCRUDRoutes(router, "/ai/workflow-commands", aiWorkflowCommandHandler, readWriteConfig, "AIWorkflowCommands")

	// Endpoint đặc biệt: Claim pending commands (atomic operation)
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	claimCommandsMiddleware := middleware.AuthMiddleware("AIWorkflowCommands.Update")
	orgContextMiddlewareCmd := middleware.OrganizationContextMiddleware()
	registerRouteWithMiddleware(router, "/ai/workflow-commands", "POST", "/claim-pending", []fiber.Handler{claimCommandsMiddleware, orgContextMiddlewareCmd}, aiWorkflowCommandHandler.ClaimPendingCommands)

	// Endpoint đặc biệt: Update heartbeat/progress (agent gọi định kỳ)
	// Lưu ý: Endpoint này có thể không cần auth nếu agent có cách xác thực khác (ví dụ: agentId trong header)
	// Tạm thời dùng auth middleware, sau này có thể thay bằng agent authentication
	updateHeartbeatMiddleware := middleware.AuthMiddleware("AIWorkflowCommands.Update")
	registerRouteWithMiddleware(router, "/ai/workflow-commands", "POST", "/update-heartbeat", []fiber.Handler{updateHeartbeatMiddleware, orgContextMiddlewareCmd}, aiWorkflowCommandHandler.UpdateHeartbeat)
	// Hỗ trợ cả URL params: /update-heartbeat/:commandId
	registerRouteWithMiddleware(router, "/ai/workflow-commands", "POST", "/update-heartbeat/:commandId", []fiber.Handler{updateHeartbeatMiddleware, orgContextMiddleware}, aiWorkflowCommandHandler.UpdateHeartbeat)

	// Endpoint đặc biệt: Release stuck commands (admin/background job)
	releaseStuckMiddleware := middleware.AuthMiddleware("AIWorkflowCommands.Update")
	registerRouteWithMiddleware(router, "/ai/workflow-commands", "POST", "/release-stuck", []fiber.Handler{releaseStuckMiddleware, orgContextMiddleware}, aiWorkflowCommandHandler.ReleaseStuckCommands)

	return nil
}

// SetupRoutes thiết lập tất cả các route cho ứng dụng
func SetupRoutes(app *fiber.App) error {
	// Khởi tạo route prefix
	prefix := NewRoutePrefix()
	v1 := app.Group(prefix.V1)

	// Khởi tạo router
	router := NewRouter(app)

	// 1. Init Routes
	if err := router.registerInitRoutes(v1); err != nil {
		return fmt.Errorf("failed to register init routes: %v", err)
	}

	// 2. Admin Routes
	if err := registerAdminRoutes(v1); err != nil {
		return fmt.Errorf("failed to register admin routes: %v", err)
	}

	// 3. System Routes
	if err := registerSystemRoutes(v1); err != nil {
		return fmt.Errorf("failed to register system routes: %v", err)
	}

	// 4. Auth Routes (Xác thực cá nhân)
	if err := router.registerAuthRoutes(v1); err != nil {
		return fmt.Errorf("failed to register auth routes: %v", err)
	}

	// 5. RBAC Routes (Bao gồm User Management)
	if err := router.registerRBACRoutes(v1); err != nil {
		return fmt.Errorf("failed to register RBAC routes: %v", err)
	}

	// 6. Facebook Routes
	if err := router.registerFacebookRoutes(v1); err != nil {
		return fmt.Errorf("failed to register Facebook routes: %v", err)
	}

	// 7. Notification Routes
	if err := router.registerNotificationRoutes(v1); err != nil {
		return fmt.Errorf("failed to register notification routes: %v", err)
	}

	// 8. CTA Routes
	if err := router.registerCTARoutes(v1); err != nil {
		return fmt.Errorf("failed to register CTA routes: %v", err)
	}

	// 9. Delivery Routes (Hệ thống 1)
	if err := router.registerDeliveryRoutes(v1); err != nil {
		return fmt.Errorf("failed to register delivery routes: %v", err)
	}

	// 10. Agent Management Routes (Bot Management System)
	if err := router.registerAgentManagementRoutes(v1); err != nil {
		return fmt.Errorf("failed to register agent management routes: %v", err)
	}

	// 11. Content Storage Routes (Module 1)
	if err := router.registerContentStorageRoutes(v1); err != nil {
		return fmt.Errorf("failed to register content storage routes: %v", err)
	}

	// 12. AI Service Routes (Module 2)
	if err := router.registerAIServiceRoutes(v1); err != nil {
		return fmt.Errorf("failed to register AI service routes: %v", err)
	}

	return nil
}
