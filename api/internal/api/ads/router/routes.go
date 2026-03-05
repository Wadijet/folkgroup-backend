// Package router — Đăng ký route module Ads (cơ chế duyệt).
package router

import (
	"github.com/gofiber/fiber/v3"

	adshdl "meta_commerce/internal/api/ads/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route Ads lên v1.
// Module độc lập — có thể tách thành service riêng.
func Register(v1 fiber.Router, _ *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	// Permission: dùng MetaAdAccount hoặc tạo AdsAction.* sau
	actionMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")
	configMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")

	// Actions: propose, approve, reject, list pending
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "POST", "/propose", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandlePropose)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "POST", "/approve", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandleApprove)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "POST", "/reject", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandleReject)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "GET", "/pending", []fiber.Handler{middleware.AuthMiddleware("MetaAdAccount.Read"), orgContextMiddleware}, adshdl.HandleListPending)

	// Config: approvalConfig
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/config", "GET", "/approval", []fiber.Handler{configMiddleware, orgContextMiddleware}, adshdl.HandleGetApprovalConfig)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/config", "PUT", "/approval", []fiber.Handler{configMiddleware, orgContextMiddleware}, adshdl.HandleUpdateApprovalConfig)

	return nil
}
