// Package router đăng ký các route thuộc domain Webhook: Pancake/PancakePOS webhook (public), WebhookLog (CRUD).
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	webhookhdl "meta_commerce/internal/api/webhook/handler"
	apirouter "meta_commerce/internal/api/router"
)

// Register đăng ký tất cả route webhook lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	pancakeWebhookHandler, err := webhookhdl.NewPancakeWebhookHandler()
	if err != nil {
		return fmt.Errorf("create pancake webhook handler: %w", err)
	}
	v1.Post("/pancake/webhook", pancakeWebhookHandler.HandlePancakeWebhook)

	pancakePosWebhookHandler, err := webhookhdl.NewPancakePosWebhookHandler()
	if err != nil {
		return fmt.Errorf("create pancake pos webhook handler: %w", err)
	}
	v1.Post("/pancake-pos/webhook", pancakePosWebhookHandler.HandlePancakePosWebhook)

	webhookLogHandler, err := webhookhdl.NewWebhookLogHandler()
	if err != nil {
		return fmt.Errorf("create webhook log handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/webhook-log", webhookLogHandler, apirouter.ReadWriteConfig, "WebhookLog")

	return nil
}
