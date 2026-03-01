// Package router đăng ký các route thuộc domain CRM: customers profile, notes.
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	crmhdl "meta_commerce/internal/api/crm/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route CRM lên v1.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	customerHandler, err := crmhdl.NewCrmCustomerHandler()
	if err != nil {
		return fmt.Errorf("tạo CrmCustomerHandler: %w", err)
	}
	noteHandler, err := crmhdl.NewCrmNoteHandler()
	if err != nil {
		return fmt.Errorf("tạo CrmNoteHandler: %w", err)
	}

	crmReadMiddleware := middleware.AuthMiddleware("Report.Read")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	middlewares := []fiber.Handler{crmReadMiddleware, orgContextMiddleware}

	// POST /customers/sync — đồng bộ crm_customers từ POS + FB. Query: sources=pos,fb
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/sync", middlewares, customerHandler.HandleSyncCustomers)

	// POST /customers/backfill-activity — backfill activity. Query: types=order,conversation,note
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/backfill-activity", middlewares, customerHandler.HandleBackfillActivity)

	// POST /customers/rebuild — sync rồi backfill. Query: sources=pos,fb types=order,conversation,note
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/rebuild", middlewares, customerHandler.HandleRebuildCrm)

	// POST /customers/recalculate-all — tính toán lại tất cả khách hàng hiện có. Body: ownerOrganizationId, limit (0=tất cả)
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/recalculate-all", middlewares, customerHandler.HandleRecalculateAllCustomers)

	// POST /customers/:unifiedId/recalculate — cập nhật toàn bộ thông tin khách từ tất cả nguồn
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/:unifiedId/recalculate", middlewares, customerHandler.HandleRecalculateCustomer)

	// GET /customers/:unifiedId/profile
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "GET", "/:unifiedId/profile", middlewares, customerHandler.HandleGetProfile)

	// POST /customers/:unifiedId/notes
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/:unifiedId/notes", middlewares, noteHandler.HandleCreateNote)
	// GET /customers/:unifiedId/notes
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "GET", "/:unifiedId/notes", middlewares, noteHandler.HandleListNotes)
	// DELETE /customers/:unifiedId/notes/:noteId
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "DELETE", "/:unifiedId/notes/:noteId", middlewares, noteHandler.HandleDeleteNote)

	return nil
}
