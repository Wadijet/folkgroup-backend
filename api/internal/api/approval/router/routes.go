// Package router — Route cơ chế duyệt (generic).
package router

import (
	"github.com/gofiber/fiber/v3"

	approvalhdl "meta_commerce/internal/api/approval/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route Approval lên v1.
func Register(v1 fiber.Router, _ *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	actionMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")

	apirouter.RegisterRouteWithMiddleware(v1, "/approval/actions", "POST", "/propose", []fiber.Handler{actionMiddleware, orgContextMiddleware}, approvalhdl.HandlePropose)
	apirouter.RegisterRouteWithMiddleware(v1, "/approval/actions", "POST", "/approve", []fiber.Handler{actionMiddleware, orgContextMiddleware}, approvalhdl.HandleApprove)
	apirouter.RegisterRouteWithMiddleware(v1, "/approval/actions", "POST", "/reject", []fiber.Handler{actionMiddleware, orgContextMiddleware}, approvalhdl.HandleReject)
	apirouter.RegisterRouteWithMiddleware(v1, "/approval/actions", "GET", "/pending", []fiber.Handler{middleware.AuthMiddleware("MetaAdAccount.Read"), orgContextMiddleware}, approvalhdl.HandleListPending)

	return nil
}
