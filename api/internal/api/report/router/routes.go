// Package router đăng ký các route thuộc domain Report: trend, recompute (báo cáo theo chu kỳ).
package router

import (
	"fmt"

	"github.com/gofiber/fiber/v3"

	reporthdl "meta_commerce/internal/api/report/handler"
	apirouter "meta_commerce/internal/api/router"
	"meta_commerce/internal/api/middleware"
)

// Register đăng ký tất cả route report lên v1: trend, recompute và CRUD report-definition, report-snapshot, report-dirty-period.
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
