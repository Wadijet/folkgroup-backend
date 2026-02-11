// Package router đăng ký các route thuộc domain Content: Nodes, Videos, Publications, Drafts.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	contenthdl "meta_commerce/internal/api/content/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route content storage lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	contentNodeHandler, err := contenthdl.NewContentNodeHandler()
	if err != nil {
		return fmt.Errorf("create content node handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/content/nodes", contentNodeHandler, apirouter.ReadWriteConfig, "ContentNodes")
	contentNodeReadMiddleware := middleware.AuthMiddleware("ContentNodes.Read")
	apirouter.RegisterRouteWithMiddleware(v1, "/content/nodes", "GET", "/tree/:id", []fiber.Handler{contentNodeReadMiddleware}, contentNodeHandler.GetTree)

	videoHandler, err := contenthdl.NewVideoHandler()
	if err != nil {
		return fmt.Errorf("create video handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/content/videos", videoHandler, apirouter.ReadWriteConfig, "ContentVideos")

	publicationHandler, err := contenthdl.NewPublicationHandler()
	if err != nil {
		return fmt.Errorf("create publication handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/content/publications", publicationHandler, apirouter.ReadWriteConfig, "ContentPublications")

	draftContentNodeHandler, err := contenthdl.NewDraftContentNodeHandler()
	if err != nil {
		return fmt.Errorf("create draft content node handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/content/drafts/nodes", draftContentNodeHandler, apirouter.ReadWriteConfig, "ContentDraftNodes")
	draftContentNodeCommitMiddleware := middleware.AuthMiddleware("ContentDraftNodes.Commit")
	apirouter.RegisterRouteWithMiddleware(v1, "/content/drafts/nodes", "POST", "/:id/commit", []fiber.Handler{draftContentNodeCommitMiddleware}, draftContentNodeHandler.CommitDraftNode)
	draftApproveMiddleware := middleware.AuthMiddleware("ContentDraftNodes.Approve")
	draftRejectMiddleware := middleware.AuthMiddleware("ContentDraftNodes.Reject")
	apirouter.RegisterRouteWithMiddleware(v1, "/content/drafts/nodes", "POST", "/:id/approve", []fiber.Handler{draftApproveMiddleware}, draftContentNodeHandler.ApproveDraft)
	apirouter.RegisterRouteWithMiddleware(v1, "/content/drafts/nodes", "POST", "/:id/reject", []fiber.Handler{draftRejectMiddleware}, draftContentNodeHandler.RejectDraft)

	draftVideoHandler, err := contenthdl.NewDraftVideoHandler()
	if err != nil {
		return fmt.Errorf("create draft video handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/content/drafts/videos", draftVideoHandler, apirouter.ReadWriteConfig, "ContentDraftVideos")

	draftPublicationHandler, err := contenthdl.NewDraftPublicationHandler()
	if err != nil {
		return fmt.Errorf("create draft publication handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/content/drafts/publications", draftPublicationHandler, apirouter.ReadWriteConfig, "ContentDraftPublications")

	return nil
}
