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
	actionMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")
	configMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")
	// Read: user tạo lệnh chờ duyệt; Update: approve/reject
	createCommandMiddleware := middleware.AuthMiddleware("MetaAdAccount.Read")

	// Tạo lệnh chờ duyệt — user có Read có thể tạo
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/commands", "POST", "", []fiber.Handler{createCommandMiddleware, orgContextMiddleware}, adshdl.HandleCreateCommand)
	// Resume Ads sau Circuit Breaker — /resume_ads
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/commands", "POST", "/resume-ads", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandleResumeAds)
	// Pancake OK — gỡ pancakeDownOverride /pancake_ok
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/commands", "POST", "/pancake-ok", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandlePancakeOk)
	// Actions: propose (alias), approve, reject, list pending
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "POST", "/propose", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandlePropose)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "POST", "/approve", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandleApprove)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "POST", "/reject", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandleReject)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "POST", "/execute", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandleExecute)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "GET", "/find-by-id/:id", []fiber.Handler{createCommandMiddleware, orgContextMiddleware}, adshdl.HandleFindById)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "GET", "/find", []fiber.Handler{createCommandMiddleware, orgContextMiddleware}, adshdl.HandleFind)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "GET", "/find-with-pagination", []fiber.Handler{createCommandMiddleware, orgContextMiddleware}, adshdl.HandleFindWithPagination)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "GET", "/count", []fiber.Handler{createCommandMiddleware, orgContextMiddleware}, adshdl.HandleCount)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "POST", "/cancel", []fiber.Handler{actionMiddleware, orgContextMiddleware}, adshdl.HandleCancel)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/actions", "GET", "/pending", []fiber.Handler{middleware.AuthMiddleware("MetaAdAccount.Read"), orgContextMiddleware}, adshdl.HandleListPending)

	// Config: approvalConfig
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/config", "GET", "/approval", []fiber.Handler{configMiddleware, orgContextMiddleware}, adshdl.HandleGetApprovalConfig)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/config", "PUT", "/approval", []fiber.Handler{configMiddleware, orgContextMiddleware}, adshdl.HandleUpdateApprovalConfig)
	// Config: meta (common, flagRule, actionRule, automation)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/config", "GET", "/meta", []fiber.Handler{configMiddleware, orgContextMiddleware}, adshdl.HandleGetMetaConfig)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/config", "PUT", "/meta", []fiber.Handler{configMiddleware, orgContextMiddleware}, adshdl.HandleUpdateMetaConfig)
	// Config: metric definitions (7d, 2h, 1h, 30p) — FolkForm v4.1
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/config", "GET", "/metric-definitions", []fiber.Handler{middleware.AuthMiddleware("MetaAdAccount.Read"), orgContextMiddleware}, adshdl.HandleGetMetricDefinitions)

	// Counterfactual Kill Tracker (FolkForm v4.1 Section 2.3) — B4–B5
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/counterfactual", "GET", "/accuracy", []fiber.Handler{middleware.AuthMiddleware("MetaAdAccount.Read"), orgContextMiddleware}, adshdl.HandleGetKillAccuracy)
	apirouter.RegisterRouteWithMiddleware(v1, "/ads/counterfactual", "GET", "/suggestion", []fiber.Handler{middleware.AuthMiddleware("MetaAdAccount.Read"), orgContextMiddleware}, adshdl.HandleGetThresholdSuggestion)

	return nil
}
