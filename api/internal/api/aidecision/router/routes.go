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
	readMiddleware := middleware.AuthMiddleware("MetaAdAccount.Read")

	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/execute", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleExecute)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/traces/:traceId/timeline", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleTraceTimeline)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/traces/:traceId/live", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleTraceLiveWS)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/org-live/timeline", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleOrgLiveTimeline)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/org-live/persisted-events", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleOrgLivePersistedEvents)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/org-live/metrics", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleOrgLiveMetrics)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/org-live", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleOrgLiveWS)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/events", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleIngestEvent)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/cases", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleListDecisionCases)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/cases/:decisionCaseId", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleGetDecisionCase)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/cases/:decisionCaseId/close", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleCloseCase)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision", "GET", "/queue-events", []fiber.Handler{readMiddleware, orgContextMiddleware}, aidecisionhdl.HandleListQueueEvents)

	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/routing-rules", "GET", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleListRoutingRules)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/routing-rules", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleUpsertRoutingRule)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/routing-rules", "DELETE", "/:id", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleDeleteRoutingRule)

	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/context-policy-overrides", "GET", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleListContextPolicyOverrides)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/context-policy-overrides", "POST", "", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleUpsertContextPolicyOverride)
	apirouter.RegisterRouteWithMiddleware(v1, "/ai-decision/context-policy-overrides", "DELETE", "/:id", []fiber.Handler{actionMiddleware, orgContextMiddleware}, aidecisionhdl.HandleDeleteContextPolicyOverride)

	return nil
}
