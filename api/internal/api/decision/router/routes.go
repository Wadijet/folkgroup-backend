// Package router — Route cho Decision Brain.
package router

import (
	"github.com/gofiber/fiber/v3"

	decisionhdl "meta_commerce/internal/api/decision/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route Decision Brain lên v1.
func Register(v1 fiber.Router, _ *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	readMiddleware := middleware.AuthMiddleware("MetaAdAccount.Read")
	actionMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")

	apirouter.RegisterRouteWithMiddleware(v1, "/decision/cases", "GET", "", []fiber.Handler{readMiddleware, orgContextMiddleware}, decisionhdl.HandleListDecisionCases)
	apirouter.RegisterRouteWithMiddleware(v1, "/decision/cases", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, decisionhdl.HandleCreateDecisionCase)
	apirouter.RegisterRouteWithMiddleware(v1, "/decision/cases", "GET", "/:id", []fiber.Handler{readMiddleware, orgContextMiddleware}, decisionhdl.HandleFindDecisionCaseById)

	return nil
}
