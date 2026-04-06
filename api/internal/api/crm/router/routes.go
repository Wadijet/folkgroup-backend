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
	pendingMergeHandler, err := crmhdl.NewCrmPendingMergeHandler()
	if err != nil {
		return fmt.Errorf("tạo CrmPendingMergeHandler: %w", err)
	}
	bulkJobHandler, err := crmhdl.NewCrmBulkJobHandler()
	if err != nil {
		return fmt.Errorf("tạo CrmBulkJobHandler: %w", err)
	}

	crmReadMiddleware := middleware.AuthMiddleware("Report.Read")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	middlewares := []fiber.Handler{crmReadMiddleware, orgContextMiddleware}

	// CRUD customers (chỉ đọc) — find, find-one, find-by-id, find-with-pagination, count. Filter theo ownerOrganizationId + classification.
	// Đăng ký trước các route /:unifiedId để tránh conflict.
	r.RegisterCRUDRoutes(v1, "/customers", customerHandler, apirouter.ReadOnlyConfig, "Report")

	// CRUD crm-pending-merge (chỉ đọc) — queue merge L1→L2 CRM (khác CIO ingest).
	r.RegisterCRUDRoutes(v1, "/crm-pending-merge", pendingMergeHandler, apirouter.ReadOnlyConfig, "Report")

	// CRUD crm-bulk-jobs — đọc + update-by-id (retry, isPriority). Queue sync/backfill/recalculate.
	r.RegisterCRUDRoutes(v1, "/crm-bulk-jobs", bulkJobHandler, apirouter.CrmBulkJobConfig, "Report")

	// POST /customers/rebuild — tạo 2 job: sync + backfill. Query/Body: sources=pos,fb,order,conversation,note (rỗng=tất cả)
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/rebuild", middlewares, customerHandler.HandleRebuildCrm)

	// POST /customers/recalculate-all — tạo N job batch. Body: ownerOrganizationId, batchSize (mặc định 200)
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/recalculate-all", middlewares, customerHandler.HandleRecalculateAllCustomers)

	// POST /customers/:unifiedId/recalculate — cập nhật toàn bộ thông tin khách từ tất cả nguồn
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "POST", "/:unifiedId/recalculate", middlewares, customerHandler.HandleRecalculateCustomer)

	// GET /customers/:unifiedId/intel-runs — lịch sử intel (crm_customer_intel_runs), phân trang
	apirouter.RegisterRouteWithMiddleware(v1, "/customers", "GET", "/:unifiedId/intel-runs", middlewares, customerHandler.HandleListIntelRuns)

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
