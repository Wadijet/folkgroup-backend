// Package router — Route cho Learning engine.
package router

import (
	"github.com/gofiber/fiber/v3"

	learninghdl "meta_commerce/internal/api/learning/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route Learning engine lên v1.
func Register(v1 fiber.Router, _ *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	readMiddleware := middleware.AuthMiddleware("MetaAdAccount.Read")
	actionMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")

	apirouter.RegisterRouteWithMiddleware(v1, "/learning/cases", "GET", "", []fiber.Handler{readMiddleware, orgContextMiddleware}, learninghdl.HandleListLearningCases)
	apirouter.RegisterRouteWithMiddleware(v1, "/learning/rule-suggestions", "GET", "", []fiber.Handler{readMiddleware, orgContextMiddleware}, learninghdl.HandleListRuleSuggestions)
	apirouter.RegisterRouteWithMiddleware(v1, "/learning/rule-suggestions", "PATCH", "/:id", []fiber.Handler{actionMiddleware, orgContextMiddleware}, learninghdl.HandlePatchRuleSuggestion)
	apirouter.RegisterRouteWithMiddleware(v1, "/learning/cases", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, learninghdl.HandleCreateLearningCase)
	apirouter.RegisterRouteWithMiddleware(v1, "/learning/cases", "GET", "/:id", []fiber.Handler{readMiddleware, orgContextMiddleware}, learninghdl.HandleFindLearningCaseById)

	return nil
}
