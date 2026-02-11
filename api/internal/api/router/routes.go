package router

import (
	"github.com/gofiber/fiber/v3"

	"meta_commerce/internal/api/middleware"
)

// ============================================================================
// âš ï¸ QUAN TRá»ŒNG: BUG FIBER V3 - CÃCH ÄÄ‚NG KÃ MIDDLEWARE
// ============================================================================
//
// Fiber v3 cÃ³ BUG nghiÃªm trá»ng vá»›i cÃ¡ch Ä‘Äƒng kÃ½ middleware trá»±c tiáº¿p trong route.
// Middleware sáº½ KHÃ”NG Ä‘Æ°á»£c gá»i náº¿u dÃ¹ng cÃ¡ch trá»±c tiáº¿p!
//
// âŒ CÃCH SAI (KHÃ”NG HOáº T Äá»˜NG):
//    router.Get("/path", middleware.AuthMiddleware(""), handler)
//    router.Post("/path", middleware.AuthMiddleware(""), handler)
//    â†’ Middleware sáº½ KHÃ”NG Ä‘Æ°á»£c gá»i, request sáº½ bá» qua middleware!
//
// âœ… CÃCH ÄÃšNG (PHáº¢I DÃ™NG):
//    authMiddleware := middleware.AuthMiddleware("")
//    RegisterRouteWithMiddleware(router, "/prefix", "GET", "/path", []fiber.Handler{authMiddleware}, handler)
//    â†’ Middleware sáº½ Ä‘Æ°á»£c gá»i Ä‘Ãºng cÃ¡ch thÃ´ng qua .Use() method
//
// ðŸ“ Lá»ŠCH Sá»¬:
//    - NgÃ y: 2025-12-28
//    - Váº¥n Ä‘á»: Endpoint /api/v1/auth/roles tráº£ vá» 401 máº·c dÃ¹ token há»£p lá»‡
//    - NguyÃªn nhÃ¢n: DÃ¹ng cÃ¡ch trá»±c tiáº¿p router.Get(path, middleware, handler)
//    - Giáº£i phÃ¡p: ÄÃ£ test 7 cÃ¡ch khÃ¡c nhau, chá»‰ cÃ³ RegisterRouteWithMiddleware hoáº¡t Ä‘á»™ng
//    - Káº¿t quáº£: ÄÃ£ sá»­a táº¥t cáº£ 21 routes trong file nÃ y
//
// ðŸ“š TÃ€I LIá»†U:
//    - Xem chi tiáº¿t: docs/06-testing/fiber-v3-middleware-registration.md
//    - HÃ m Ä‘Ãºng: RegisterRouteWithMiddleware() (dÃ²ng 159-195)
//
// ðŸ” KIá»‚M TRA:
//    Náº¿u tháº¥y route nÃ o dÃ¹ng cÃ¡ch trá»±c tiáº¿p router.Get/Post/Put/Delete(path, middleware, handler)
//    â†’ PHáº¢I Sá»¬A NGAY thÃ nh RegisterRouteWithMiddleware!
//
// ============================================================================

// CONFIGS

// CRUDHandler Ä‘á»‹nh nghÄ©a interface cho cÃ¡c handler CRUD
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

// Router quáº£n lÃ½ viá»‡c Ä‘á»‹nh tuyáº¿n cho API
type Router struct {
	app *fiber.App
}

// CRUDConfig cáº¥u hÃ¬nh cÃ¡c operation Ä‘Æ°á»£c phÃ©p cho má»—i collection
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

// Config cho từng collection. Các domain dùng chung: ReadOnlyConfig, ReadWriteConfig, OrgConfigItemConfig.
var (
	// ReadOnlyConfig chỉ cho phép đọc (find, find-one, count, distinct, exists).
	ReadOnlyConfig = CRUDConfig{
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

	// ReadWriteConfig cho phép đầy đủ CRUD.
	ReadWriteConfig = CRUDConfig{
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

	// OrgConfigItemConfig cho Organization Config Items (1 document per key): find-one, find, upsert-one, delete-one (+ resolved).
	OrgConfigItemConfig = CRUDConfig{
		InsOne: false, InsMany: false,
		Find: true, FindOne: true, FindById: false,
		FindIds: false, Paginate: false,
		UpdOne: false, UpdMany: false, UpdById: false,
		FindUpd: false,
		DelOne:  true, DelMany: false, DelById: false,
		FindDel: false,
		Count:   false, Distinct: false,
		Upsert: true, UpsMany: false, Exists: false,
	}
)

// RoutePrefix chá»©a cÃ¡c prefix cÆ¡ báº£n cho API
type RoutePrefix struct {
	Base string // Prefix cÆ¡ báº£n (/api)
	V1   string // Prefix cho API version 1 (/api/v1)
}

// NewRoutePrefix táº¡o má»›i má»™t instance cá»§a RoutePrefix vá»›i cÃ¡c giÃ¡ trá»‹ máº·c Ä‘á»‹nh
func NewRoutePrefix() RoutePrefix {
	base := "/api"
	return RoutePrefix{
		Base: base,
		V1:   base + "/v1",
	}
}

// NewRouter táº¡o má»›i má»™t instance cá»§a Router
func NewRouter(app *fiber.App) *Router {
	return &Router{
		app: app,
	}
}

// RegisterRouteWithMiddleware Ä‘Äƒng kÃ½ route vá»›i middleware sá»­ dá»¥ng .Use() method (cÃ¡ch Ä‘Ãºng theo Fiber v3)
//
// âš ï¸ QUAN TRá»ŒNG: ÄÃ¢y lÃ  CÃCH DUY NHáº¤T hoáº¡t Ä‘á»™ng Ä‘Ãºng trong Fiber v3!
//
// âŒ KHÃ”NG DÃ™NG cÃ¡ch trá»±c tiáº¿p: router.Get(path, middleware, handler) - middleware sáº½ KHÃ”NG Ä‘Æ°á»£c gá»i!
// âœ… PHáº¢I DÃ™NG cÃ¡ch nÃ y: RegisterRouteWithMiddleware vá»›i .Use() method
//
// Lá»‹ch sá»­: ÄÃ£ test 7 cÃ¡ch khÃ¡c nhau (2025-12-28) vÃ  chá»‰ cÃ³ cÃ¡ch nÃ y hoáº¡t Ä‘á»™ng.
// Xem thÃªm: docs/06-testing/fiber-v3-middleware-registration.md
//
// VÃ­ dá»¥ sá»­ dá»¥ng:
//
//	authMiddleware := middleware.AuthMiddleware("")
//	RegisterRouteWithMiddleware(router, "/auth", "GET", "/roles", []fiber.Handler{authMiddleware}, handler)
// RegisterRouteWithMiddleware đăng ký route với middleware (cách đúng theo Fiber v3). Dùng từ domain router.
func RegisterRouteWithMiddleware(router fiber.Router, prefix string, method string, path string, middlewares []fiber.Handler, handler fiber.Handler) {
	// Táº¡o group vá»›i prefix, middleware sáº½ chá»‰ Ã¡p dá»¥ng cho routes trong group nÃ y
	routeGroup := router.Group(prefix)
	for _, mw := range middlewares {
		routeGroup.Use(mw) // â† ÄÃ‚Y LÃ€ CÃCH ÄÃšNG - dÃ¹ng .Use() thay vÃ¬ truyá»n trá»±c tiáº¿p
	}

	// ÄÄƒng kÃ½ route vá»›i path tÆ°Æ¡ng Ä‘á»‘i (khÃ´ng cÃ³ prefix vÃ¬ Ä‘Ã£ cÃ³ trong group)
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

// registerCRUDRoutes Ä‘Äƒng kÃ½ cÃ¡c route CRUD cho má»™t collection
//
// âš ï¸ LÆ¯U Ã: HÃ m nÃ y Ä‘Ã£ dÃ¹ng RegisterRouteWithMiddleware (cÃ¡ch Ä‘Ãºng), khÃ´ng cáº§n sá»­a.
// Náº¿u thÃªm route má»›i bÃªn ngoÃ i hÃ m nÃ y, PHáº¢I dÃ¹ng RegisterRouteWithMiddleware (xem comment á»Ÿ Ä‘áº§u file)
// RegisterCRUDRoutes đăng ký các route CRUD cho một collection. Dùng từ domain router.
func (r *Router) RegisterCRUDRoutes(router fiber.Router, prefix string, h CRUDHandler, config CRUDConfig, permissionPrefix string) {
	// Táº¡o middleware chain: AuthMiddleware â†’ OrganizationContextMiddleware
	// ÄÃ£ táº¯t log Ä‘á»ƒ giáº£m log khi khá»Ÿi Ä‘á»™ng
	authMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Insert")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	authReadMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Read")
	authUpdateMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Update")
	authDeleteMiddleware := middleware.AuthMiddleware(permissionPrefix + ".Delete")

	// Create operations
	if config.InsOne {
		RegisterRouteWithMiddleware(router, prefix, "POST", "/insert-one", []fiber.Handler{authMiddleware, orgContextMiddleware}, h.InsertOne)
	}
	if config.InsMany {
		RegisterRouteWithMiddleware(router, prefix, "POST", "/insert-many", []fiber.Handler{authMiddleware, orgContextMiddleware}, h.InsertMany)
	}

	// Read operations
	if config.Find {
		RegisterRouteWithMiddleware(router, prefix, "GET", "/find", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.Find)
	}
	if config.FindOne {
		RegisterRouteWithMiddleware(router, prefix, "GET", "/find-one", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.FindOne)
	}
	if config.FindById {
		RegisterRouteWithMiddleware(router, prefix, "GET", "/find-by-id/:id", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.FindOneById)
	}
	if config.FindIds {
		RegisterRouteWithMiddleware(router, prefix, "POST", "/find-by-ids", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.FindManyByIds)
	}
	if config.Paginate {
		RegisterRouteWithMiddleware(router, prefix, "GET", "/find-with-pagination", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.FindWithPagination)
	}

	// Update operations
	if config.UpdOne {
		RegisterRouteWithMiddleware(router, prefix, "PUT", "/update-one", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.UpdateOne)
	}
	if config.UpdMany {
		RegisterRouteWithMiddleware(router, prefix, "PUT", "/update-many", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.UpdateMany)
	}
	if config.UpdById {
		RegisterRouteWithMiddleware(router, prefix, "PUT", "/update-by-id/:id", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.UpdateById)
	}
	if config.FindUpd {
		RegisterRouteWithMiddleware(router, prefix, "PUT", "/find-one-and-update", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.FindOneAndUpdate)
	}

	// Delete operations
	if config.DelOne {
		RegisterRouteWithMiddleware(router, prefix, "DELETE", "/delete-one", []fiber.Handler{authDeleteMiddleware, orgContextMiddleware}, h.DeleteOne)
	}
	if config.DelMany {
		RegisterRouteWithMiddleware(router, prefix, "DELETE", "/delete-many", []fiber.Handler{authDeleteMiddleware, orgContextMiddleware}, h.DeleteMany)
	}
	if config.DelById {
		RegisterRouteWithMiddleware(router, prefix, "DELETE", "/delete-by-id/:id", []fiber.Handler{authDeleteMiddleware, orgContextMiddleware}, h.DeleteById)
	}
	if config.FindDel {
		RegisterRouteWithMiddleware(router, prefix, "DELETE", "/find-one-and-delete", []fiber.Handler{authDeleteMiddleware, orgContextMiddleware}, h.FindOneAndDelete)
	}

	// Other operations
	if config.Count {
		// Count chá»‰ cáº§n Ä‘Äƒng nháº­p, khÃ´ng cáº§n permission cá»¥ thá»ƒ
		authOnlyMiddleware := middleware.AuthMiddleware("")
		RegisterRouteWithMiddleware(router, prefix, "GET", "/count", []fiber.Handler{authOnlyMiddleware}, h.CountDocuments)
	}
	if config.Distinct {
		RegisterRouteWithMiddleware(router, prefix, "GET", "/distinct", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.Distinct)
	}
	if config.Upsert {
		RegisterRouteWithMiddleware(router, prefix, "POST", "/upsert-one", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.Upsert)
	}
	if config.UpsMany {
		RegisterRouteWithMiddleware(router, prefix, "POST", "/upsert-many", []fiber.Handler{authUpdateMiddleware, orgContextMiddleware}, h.UpsertMany)
	}
	if config.Exists {
		RegisterRouteWithMiddleware(router, prefix, "GET", "/exists", []fiber.Handler{authReadMiddleware, orgContextMiddleware}, h.DocumentExists)
	}
}

// CÃ¡c hÃ m Ä‘Äƒng kÃ½ route theo domain náº±m trong: auth_routes.go, facebook_routes.go, notification_routes.go, cta_routes.go, delivery_routes.go, agent_routes.go, content_routes.go, ai_routes.go


// RegisterFunc là hàm đăng ký route của một domain (do domain/router export).
type RegisterFunc func(v1 fiber.Router, r *Router) error

// SetupRoutes thiết lập tất cả các route cho ứng dụng. Caller truyền lần lượt Register của từng domain để tránh import cycle.
func SetupRoutes(app *fiber.App, regs ...RegisterFunc) error {
	prefix := NewRoutePrefix()
	v1 := app.Group(prefix.V1)
	r := NewRouter(app)
	for _, reg := range regs {
		if err := reg(v1, r); err != nil {
			return err
		}
	}
	return nil
}
