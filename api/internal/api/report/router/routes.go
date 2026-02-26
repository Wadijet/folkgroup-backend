// Package router đăng ký các route thuộc domain Report: trend, recompute (báo cáo theo chu kỳ).
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	reporthdl "meta_commerce/internal/api/report/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route report lên v1: trend, recompute, dashboard orders và CRUD report-definition, report-snapshot, report-dirty-period.
func Register(v1 fiber.Router, r *apirouter.Router) error {
	reportHandler, err := reporthdl.NewReportHandler()
	if err != nil {
		return fmt.Errorf("create report handler: %w", err)
	}
	reportReadMiddleware := middleware.AuthMiddleware("Report.Read")
	reportRecomputeMiddleware := middleware.AuthMiddleware("Report.Recompute")
	orgContextMiddleware := middleware.OrganizationContextMiddleware()
	apirouter.RegisterRouteWithMiddleware(v1, "/reports", "GET", "/trend", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleTrend)
	apirouter.RegisterRouteWithMiddleware(v1, "/reports", "POST", "/recompute", []fiber.Handler{reportRecomputeMiddleware, orgContextMiddleware}, reportHandler.HandleRecompute)

	// Dashboard Order Processing (TAB 6) — dữ liệu lũy kế, query trực tiếp DB
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/orders/funnel", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetOrderFunnel)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/orders/recent", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetRecentOrders)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/orders/stage-aging", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetStageAging)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/orders/stuck-orders", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetStuckOrders)

	// Dashboard Inventory Intelligence (TAB 5) — snapshot tồn kho
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/inventory", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetInventory)
	// Tree lazy load: danh sách sản phẩm (level 1) + mẫu mã khi expand (level 2)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/inventory/products", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetInventoryProducts)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/inventory/products/:productId/variations", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetInventoryProductVariations)

	// Dashboard Customer Intelligence (TAB 4) — KPI, tier distribution, lifecycle, bảng khách, VIP inactive panel
	// Đăng ký route con trước /customers để tránh conflict
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers/trend", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetCustomersTrend)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers/trend/transition-matrix", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetTransitionMatrix)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers/trend/group-changes", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetGroupChanges)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers/ceo-groups", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetCeoGroups)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers/journey-funnel", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetJourneyFunnel)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers/asset-matrix", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetAssetMatrix)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers/matrix-journey-value", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetMatrixJourneyValue)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers/matrix-value-loyalty", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetMatrixValueLoyalty)
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/customers", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetCustomers)

	// Dashboard Inbox Operations (TAB 7) — KPI, bảng hội thoại, Sale performance, Alert zone
	apirouter.RegisterRouteWithMiddleware(v1, "/dashboard", "GET", "/inbox", []fiber.Handler{reportReadMiddleware, orgContextMiddleware}, reportHandler.HandleGetInbox)

	// CRUD report definition (chỉ đọc)
	reportDefHandler, err := reporthdl.NewReportDefinitionHandler()
	if err != nil {
		return fmt.Errorf("create report definition handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/report-definition", reportDefHandler, apirouter.ReadOnlyConfig, "Report")

	// CRUD report snapshot (chỉ đọc) - dữ liệu do engine tính, filter theo OwnerOrganizationID
	reportSnapshotHandler, err := reporthdl.NewReportSnapshotHandler()
	if err != nil {
		return fmt.Errorf("create report snapshot handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/report-snapshot", reportSnapshotHandler, apirouter.ReadOnlyConfig, "Report")

	// CRUD report dirty period (chỉ đọc) - hàng đợi chu kỳ cần tính, filter theo OwnerOrganizationID
	reportDirtyPeriodHandler, err := reporthdl.NewReportDirtyPeriodHandler()
	if err != nil {
		return fmt.Errorf("create report dirty period handler: %w", err)
	}
	r.RegisterCRUDRoutes(v1, "/report-dirty-period", reportDirtyPeriodHandler, apirouter.ReadOnlyConfig, "Report")

	return nil
}
