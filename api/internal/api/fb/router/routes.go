// Package router đăng ký các route thuộc domain Facebook: Page, Post, Conversation, Message, MessageItem, FbCustomer.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	fbhdl "meta_commerce/internal/api/fb/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route Facebook lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	fbPageHandler, err := fbhdl.NewFbPageHandler()
	if err != nil {
		return fmt.Errorf("create facebook page handler: %w", err)
	}
	fbPageReadMiddleware := middleware.AuthMiddleware("FbPage.Read")
	fbPageUpdateMiddleware := middleware.AuthMiddleware("FbPage.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/facebook/page", "GET", "/find-by-page-id/:id", []fiber.Handler{fbPageReadMiddleware}, fbPageHandler.HandleFindOneByPageID)
	apirouter.RegisterRouteWithMiddleware(v1, "/facebook/page", "PUT", "/update-token", []fiber.Handler{fbPageUpdateMiddleware}, fbPageHandler.HandleUpdateToken)
	r.RegisterCRUDRoutes(v1, "/facebook/page", fbPageHandler, apirouter.ReadWriteConfig, "FbPage")

	fbPostHandler, err := fbhdl.NewFbPostHandler()
	if err != nil {
		return fmt.Errorf("create facebook post handler: %w", err)
	}
	fbPostReadMiddleware := middleware.AuthMiddleware("FbPost.Read")
	apirouter.RegisterRouteWithMiddleware(v1, "/facebook/post", "GET", "/find-by-post-id/:id", []fiber.Handler{fbPostReadMiddleware}, fbPostHandler.HandleFindOneByPostID)
	r.RegisterCRUDRoutes(v1, "/facebook/post", fbPostHandler, apirouter.ReadWriteConfig, "FbPost")

	fbConvHandler, err := fbhdl.NewFbConversationHandler()
	if err != nil {
		return fmt.Errorf("create facebook conversation handler: %w", err)
	}
	fbConvReadMiddleware := middleware.AuthMiddleware("FbConversation.Read")
	apirouter.RegisterRouteWithMiddleware(v1, "/facebook/conversation", "GET", "/sort-by-api-update", []fiber.Handler{fbConvReadMiddleware}, fbConvHandler.HandleFindAllSortByApiUpdate)
	r.RegisterCRUDRoutes(v1, "/facebook/conversation", fbConvHandler, apirouter.ReadWriteConfig, "FbConversation")

	fbMessageHandler, err := fbhdl.NewFbMessageHandler()
	if err != nil {
		return fmt.Errorf("create facebook message handler: %w", err)
	}
	fbMessageUpdateMiddleware := middleware.AuthMiddleware("FbMessage.Update")
	apirouter.RegisterRouteWithMiddleware(v1, "/facebook/message", "POST", "/upsert-messages", []fiber.Handler{fbMessageUpdateMiddleware}, fbMessageHandler.HandleUpsertMessages)
	r.RegisterCRUDRoutes(v1, "/facebook/message", fbMessageHandler, apirouter.ReadWriteConfig, "FbMessage")

	fbMessageItemHandler, err := fbhdl.NewFbMessageItemHandler()
	if err != nil {
		return fmt.Errorf("create facebook message item handler: %w", err)
	}
	fbMessageItemReadMiddleware := middleware.AuthMiddleware("FbMessageItem.Read")
	apirouter.RegisterRouteWithMiddleware(v1, "/facebook/message-item", "GET", "/find-by-conversation/:conversationId", []fiber.Handler{fbMessageItemReadMiddleware}, fbMessageItemHandler.HandleFindByConversationId)
	apirouter.RegisterRouteWithMiddleware(v1, "/facebook/message-item", "GET", "/find-by-message-id/:messageId", []fiber.Handler{fbMessageItemReadMiddleware}, fbMessageItemHandler.HandleFindOneByMessageId)
	r.RegisterCRUDRoutes(v1, "/facebook/message-item", fbMessageItemHandler, apirouter.ReadWriteConfig, "FbMessageItem")

	fbCustomerHandler, err := fbhdl.NewFbCustomerHandler()
	if err != nil {
		return fmt.Errorf("create fb customer handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/fb-customer", fbCustomerHandler, apirouter.ReadWriteConfig, "FbCustomer")

	return nil
}
