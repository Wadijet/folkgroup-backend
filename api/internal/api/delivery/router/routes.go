// Package router đăng ký các route thuộc domain Delivery: Send, History.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	deliveryhdl "meta_commerce/internal/api/delivery/handler"
	notifhdl "meta_commerce/internal/api/notification/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route delivery lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	sendHandler, err := deliveryhdl.NewDeliverySendHandler()
	if err != nil {
		return fmt.Errorf("create delivery send handler: %w", err)
	}
	sendMiddleware := middleware.AuthMiddleware("Delivery.Send")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	apirouter.RegisterRouteWithMiddleware(v1, "/delivery", "POST", "/send", []fiber.Handler{sendMiddleware, orgContextMiddleware}, sendHandler.HandleSend)

	historyHandler, err := notifhdl.NewNotificationHistoryHandler()
	if err != nil {
		return fmt.Errorf("create delivery history handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/delivery/history", historyHandler, apirouter.ReadOnlyConfig, "DeliveryHistory")
	return nil
}
