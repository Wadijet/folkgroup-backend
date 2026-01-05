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
	agentConfig             = readWriteConfig
	organizationShareConfig = readWriteConfig

	// Pancake Module Collections
	accessTokenConfig   = readWriteConfig
	fbPageConfig        = readWriteConfig
	fbPostConfig        = readWriteConfig
	fbConvConfig        = readWriteConfig
	fbMessageConfig     = readWriteConfig
	fbMessageItemConfig = readWriteConfig
	pcOrderConfig       = readWriteConfig
	customerConfig      = readWriteConfig

	// Notification Module Collections
	notificationSenderConfig   = readWriteConfig
	notificationChannelConfig  = readWriteConfig
	notificationTemplateConfig = readWriteConfig
	notificationRoutingConfig  = readWriteConfig
	notificationHistoryConfig  = readOnlyConfig // History chỉ đọc
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
	fmt.Printf("[ROUTER] Registering CRUD routes for prefix: %s, permissionPrefix: %s\n", prefix, permissionPrefix)
	authMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Insert")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	authReadMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Read")
	authUpdateMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Update")
	authDeleteMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Delete")
	fmt.Printf("[ROUTER] Middleware created for prefix: %s\n", prefix)

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
	fmt.Printf("Registering permission routes with prefix: /permission\n")
	// Route đặc biệt cho lấy permissions theo category
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	permReadMiddleware := middleware.AuthMiddleware("Permission.Read")
	registerRouteWithMiddleware(router, "/permission", "GET", "/by-category/:category", []fiber.Handler{permReadMiddleware}, permHandler.HandleGetPermissionsByCategory)
	// Route đặc biệt cho lấy permissions theo group
	registerRouteWithMiddleware(router, "/permission", "GET", "/by-group/:group", []fiber.Handler{permReadMiddleware}, permHandler.HandleGetPermissionsByGroup)
	// CRUD routes
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
	fmt.Printf("Registering organization routes with prefix: /organization\n")
	r.registerCRUDRoutes(router, "/organization", organizationHandler, readWriteConfig, "Organization")
	fmt.Printf("Organization routes registered successfully\n")

	// Organization Share routes
	organizationShareHandler, err := handler.NewOrganizationShareHandler()
	if err != nil {
		return fmt.Errorf("failed to create organization share handler: %v", err)
	}
	// Route đặc biệt với logic riêng cho CreateShare và DeleteShare (có validation đặc biệt về quyền với fromOrg)
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	orgShareCreateMiddleware := middleware.AuthMiddleware("OrganizationShare.Create")
	orgShareDeleteMiddleware := middleware.AuthMiddleware("OrganizationShare.Delete")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	registerRouteWithMiddleware(router, "/organization-share", "POST", "", []fiber.Handler{orgShareCreateMiddleware, orgContextMiddleware}, organizationShareHandler.CreateShare)
	registerRouteWithMiddleware(router, "/organization-share", "DELETE", "/:id", []fiber.Handler{orgShareDeleteMiddleware, orgContextMiddleware}, organizationShareHandler.DeleteShare)
	// CRUD routes - đăng ký đầy đủ các operation CRUD (Find, FindById, Update, v.v.)
	r.registerCRUDRoutes(router, "/organization-share", organizationShareHandler, organizationShareConfig, "OrganizationShare")

	// Agent routes
	agentHandler, err := handler.NewAgentHandler()
	if err != nil {
		return fmt.Errorf("failed to create agent handler: %v", err)
	}
	// Đăng ký các route đặc biệt cho agent: check-in/check-out
	// FIX: Dùng registerRouteWithMiddleware với .Use() method (cách đúng) thay vì cách trực tiếp có bug trong Fiber v3
	agentCheckInMiddleware := middleware.AuthMiddleware("Agent.CheckIn")
	agentCheckOutMiddleware := middleware.AuthMiddleware("Agent.CheckOut")
	registerRouteWithMiddleware(router, "/agent", "POST", "/check-in/:id", []fiber.Handler{agentCheckInMiddleware}, agentHandler.HandleCheckIn)    // Route check-in cho agent
	registerRouteWithMiddleware(router, "/agent", "POST", "/check-out/:id", []fiber.Handler{agentCheckOutMiddleware}, agentHandler.HandleCheckOut) // Route check-out cho agent
	r.registerCRUDRoutes(router, "/agent", agentHandler, agentConfig, "Agent")

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

	// Customer routes (deprecated - dùng fb-customer và pc-pos-customer)
	customerHandler, err := handler.NewCustomerHandler()
	if err != nil {
		return fmt.Errorf("failed to create customer handler: %v", err)
	}
	// CRUD routes chuẩn (bao gồm upsert-one với filter)
	r.registerCRUDRoutes(router, "/customer", customerHandler, readWriteConfig, "Customer")

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
	registerRouteWithMiddleware(router, "/notification", "POST", "/trigger", []fiber.Handler{notificationTriggerMiddleware}, triggerHandler.HandleTriggerNotification)

	// Notification Tracking routes (public, không cần auth)
	trackHandler, err := handler.NewNotificationTrackHandler()
	if err != nil {
		return fmt.Errorf("failed to create notification track handler: %v", err)
	}
	router.Get("/notification/track/open/:historyId", trackHandler.HandleTrackOpen)
	router.Get("/notification/track/:historyId/:ctaIndex", trackHandler.HandleTrackClick)
	router.Get("/notification/confirm/:historyId", trackHandler.HandleTrackConfirm)

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

	// CTA Tracking route (public, không cần auth) - endpoint đặc biệt cần thiết cho tracking clicks
	ctaTrackHandler := handler.NewCTATrackHandler()
	router.Get("/cta/track/:historyId/:ctaIndex", ctaTrackHandler.TrackCTAClick)

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
	// - Tracking: Dùng /notification/track/* (cùng chức năng)

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

	return nil
}
