// Package router đăng ký các route thuộc domain Notification: Sender, Channel, Template, Routing, History, Trigger, Tracking.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	deliveryhdl "meta_commerce/internal/api/delivery/handler"
	notifhdl "meta_commerce/internal/api/notification/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route notification lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	senderHandler, err := notifhdl.NewNotificationSenderHandler()
	if err != nil {
		return fmt.Errorf("create notification sender handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/notification/sender", senderHandler, apirouter.ReadWriteConfig, "NotificationSender")

	channelHandler, err := notifhdl.NewNotificationChannelHandler()
	if err != nil {
		return fmt.Errorf("create notification channel handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/notification/channel", channelHandler, apirouter.ReadWriteConfig, "NotificationChannel")

	templateHandler, err := notifhdl.NewNotificationTemplateHandler()
	if err != nil {
		return fmt.Errorf("create notification template handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/notification/template", templateHandler, apirouter.ReadWriteConfig, "NotificationTemplate")

	routingHandler, err := notifhdl.NewNotificationRoutingHandler()
	if err != nil {
		return fmt.Errorf("create notification routing handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/notification/routing", routingHandler, apirouter.ReadWriteConfig, "NotificationRouting")

	historyHandler, err := notifhdl.NewNotificationHistoryHandler()
	if err != nil {
		return fmt.Errorf("create notification history handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/notification/history", historyHandler, apirouter.ReadOnlyConfig, "DeliveryHistory")

	triggerHandler, err := notifhdl.NewNotificationTriggerHandler()
	if err != nil {
		return fmt.Errorf("create notification trigger handler: %w", err)
	}
	notificationTriggerMiddleware := middleware.AuthMiddleware("Notification.Trigger")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	apirouter.RegisterRouteWithMiddleware(v1, "/notification", "POST", "/trigger", []fiber.Handler{notificationTriggerMiddleware, orgContextMiddleware}, triggerHandler.HandleTriggerNotification)

	trackingHandler, err := deliveryhdl.NewTrackingHandler()
	if err != nil {
		return fmt.Errorf("create tracking handler: %w", err)
	}
	v1.Get("/track/:action/:historyId", trackingHandler.HandleAction)
	return nil
}
