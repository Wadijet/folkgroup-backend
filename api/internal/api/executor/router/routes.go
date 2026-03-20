// Package router — Route cho Executor (Approval Gate + Execution).
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	deliveryhdl "meta_commerce/internal/api/delivery/handler"
	executorhdl "meta_commerce/internal/api/executor/handler"
	notifhdl "meta_commerce/internal/api/notification/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký route Executor lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	actionMiddleware := middleware.AuthMiddleware("MetaAdAccount.Update")
	readMiddleware := middleware.AuthMiddleware("MetaAdAccount.Read")

	// Executor actions (từ approval)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "GET", "/find-by-id/:id", []fiber.Handler{readMiddleware, orgContextMiddleware}, executorhdl.HandleFindById)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "GET", "/find", []fiber.Handler{readMiddleware, orgContextMiddleware}, executorhdl.HandleFind)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "GET", "/find-with-pagination", []fiber.Handler{readMiddleware, orgContextMiddleware}, executorhdl.HandleFindWithPagination)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "GET", "/count", []fiber.Handler{readMiddleware, orgContextMiddleware}, executorhdl.HandleCount)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "POST", "/propose", []fiber.Handler{actionMiddleware, orgContextMiddleware}, executorhdl.HandlePropose)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "POST", "/cancel", []fiber.Handler{actionMiddleware, orgContextMiddleware}, executorhdl.HandleCancel)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "POST", "/approve", []fiber.Handler{actionMiddleware, orgContextMiddleware}, executorhdl.HandleApprove)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "POST", "/reject", []fiber.Handler{actionMiddleware, orgContextMiddleware}, executorhdl.HandleReject)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "POST", "/execute", []fiber.Handler{actionMiddleware, orgContextMiddleware}, executorhdl.HandleExecute)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor/actions", "GET", "/pending", []fiber.Handler{readMiddleware, orgContextMiddleware}, executorhdl.HandleListPending)

	// Executor send, execute (từ delivery)
	sendHandler, err := deliveryhdl.NewDeliverySendHandler()
	if err != nil {
		return fmt.Errorf("create delivery send handler: %w", err)
	}
	sendMiddleware := middleware.AuthMiddleware("Delivery.Send")
	apirouter.RegisterRouteWithMiddleware(v1, "/executor", "POST", "/send", []fiber.Handler{sendMiddleware, orgContextMiddleware}, sendHandler.HandleSend)
	apirouter.RegisterRouteWithMiddleware(v1, "/executor", "POST", "/execute", []fiber.Handler{sendMiddleware, orgContextMiddleware}, sendHandler.HandleExecute)

	// Executor history (từ delivery/history)
	historyHandler, err := notifhdl.NewNotificationHistoryHandler()
	if err != nil {
		return fmt.Errorf("create delivery history handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/executor/history", historyHandler, apirouter.ReadOnlyConfig, "DeliveryHistory")

	return nil
}
