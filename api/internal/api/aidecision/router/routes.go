// Package router — Route cho AI Decision.
package router

import (
	"github.com/gofiber/fiber/v3"

	aidecisionhdl "meta_commerce/internal/api/aidecision/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route AI Decision lên v1.
func Register(v1 fiber.Router, _ *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	actionMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")

	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/execute", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleExecute)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/events", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleIngestEvent)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/cases/:decisionCaseId/close", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleCloseCase)

	return nil
}
