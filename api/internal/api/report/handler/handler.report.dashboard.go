// Package reporthdl - Handler cho Dashboard Order Processing (funnel, orders) và Inventory Intelligence (Tab 5).
package reporthdl

import (
	"strconv"

	basehdl "meta_commerce/internal/api/base/handler"
	reportdto "meta_commerce/internal/api/report/dto"
	"meta_commerce/internal/common"

	"github.com/gofiber/fiber/v3"
)

// HandleGetOrderFunnel xử lý GET /dashboard/orders/funnel — funnel đơn hàng lũy kế (snapshot).
// Query: by=stage (mặc định, 6 stage) hoặc by=status (17 status chi tiết).
func (h *ReportHandler) HandleGetOrderFunnel(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		by := c.Query("by", "stage")
		if by != "stage" && by != "status" {
			by = "stage"
		}
		statusItems, stageItems, err := h.ReportService.GetOrderFunnelSnapshot(c.Context(), *orgID, by)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn funnel đơn hàng", "status": "error",
			})
			return nil
		}
		data := fiber.Map{"by": by}
		if by == "stage" {
			data["funnel"] = stageItems
		} else {
			data["funnel"] = statusItems
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": data, "status": "success",
		})
		return nil
	})
}

// HandleGetStageAging xử lý GET /dashboard/orders/stage-aging — Stage Aging Distribution.
// Trả về buckets, stuck rate, percentiles (P50/P90/P95/P99) cho từng stage có SLA.
func (h *ReportHandler) HandleGetStageAging(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		items, err := h.ReportService.GetStageAgingSnapshot(c.Context(), *orgID)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Stage Aging", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{"stages": items}, "status": "success",
		})
		return nil
	})
}

// HandleGetStuckOrders xử lý GET /dashboard/orders/stuck-orders — danh sách đơn vượt SLA có phân trang.
// Query: page (mặc định 1), limit (mặc định 50, max 200), stage (lọc theo stage, optional).
// Response format chuẩn phân trang: { page, limit, itemCount, items, total, totalPage }.
func (h *ReportHandler) HandleGetStuckOrders(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		page := int64(1)
		if s := c.Query("page"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n >= 1 {
				page = n
			}
		}
		limit := int64(50)
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
				limit = n
				if limit > 200 {
					limit = 200
				}
			}
		}
		stageFilter := c.Query("stage")
		result, err := h.ReportService.GetStuckOrders(c.Context(), *orgID, page, limit, stageFilter)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn đơn vượt SLA", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetRecentOrders xử lý GET /dashboard/orders/recent — 5 đơn hàng gần nhất.
// Query: limit (mặc định 5, max 100).
func (h *ReportHandler) HandleGetRecentOrders(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		limit := 5
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
				if limit > 100 {
					limit = 100
				}
			}
		}
		items, err := h.ReportService.GetRecentOrders(c.Context(), *orgID, limit)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn đơn hàng gần nhất", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{"items": items}, "status": "success",
		})
		return nil
	})
}

// HandleGetInventory xử lý GET /dashboard/inventory — snapshot tồn kho Tab 5 Inventory Intelligence.
// Query: from, to, period, page (mặc định 1), limit (mặc định 50), status, warehouseId, sort, lowStockThreshold, lowStockDaysCover.
// Response format chuẩn phân trang: page, limit, itemCount, items, total, totalPage (như PaginateResult).
func (h *ReportHandler) HandleGetInventory(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		var params reportdto.InventoryQueryParams
		_ = c.Bind().Query(&params)
		if params.Page <= 0 {
			params.Page = 1
		}
		if params.Limit <= 0 {
			params.Limit = 50 // Mặc định 50 dòng/trang — chuẩn phân trang
		}
		if params.Limit > 2000 {
			params.Limit = 2000
		}
		if params.Period == "" {
			params.Period = "month"
		}
		if params.Status == "" {
			params.Status = "all"
		}
		if params.Sort == "" {
			params.Sort = "days_cover_asc"
		}
		if params.LowStockThreshold <= 0 {
			params.LowStockThreshold = 10
		}
		if params.LowStockDaysCover <= 0 {
			params.LowStockDaysCover = 7
		}
		result, err := h.ReportService.GetInventorySnapshot(c.Context(), *orgID, &params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn tồn kho", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetInventoryProducts xử lý GET /dashboard/inventory/products — danh sách sản phẩm (level 1 tree, lazy load).
// Query: from, to, period, page, limit, status, warehouseId, sort, lowStockThreshold, lowStockDaysCover.
func (h *ReportHandler) HandleGetInventoryProducts(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		var params reportdto.InventoryProductsQueryParams
		_ = c.Bind().Query(&params)
		result, err := h.ReportService.GetInventoryProducts(c.Context(), *orgID, &params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn danh sách sản phẩm tồn kho", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetInventoryProductVariations xử lý GET /dashboard/inventory/products/:productId/variations — mẫu mã của 1 sản phẩm (level 2 tree).
// Gọi khi user expand 1 sản phẩm.
func (h *ReportHandler) HandleGetInventoryProductVariations(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		productId := c.Params("productId")
		if productId == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "productId không được để trống", "status": "error",
			})
			return nil
		}
		var params reportdto.InventoryVariationsQueryParams
		_ = c.Bind().Query(&params)
		result, err := h.ReportService.GetInventoryProductVariations(c.Context(), *orgID, productId, &params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn mẫu mã tồn kho", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetCustomers xử lý GET /dashboard/customers — snapshot Tab 4 Customer Intelligence.
// Query: from, to, period, filter, limit, offset, sort, vipInactiveLimit, activeDays, coolingDays, inactiveDays.
func (h *ReportHandler) HandleGetCustomers(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		var params reportdto.CustomersQueryParams
		_ = c.Bind().Query(&params)
		result, err := h.ReportService.GetCustomersSnapshot(c.Context(), *orgID, &params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Customer Intelligence", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetInbox xử lý GET /dashboard/inbox — snapshot Tab 7 Inbox Operations.
// Query: pageId, filter (backlog|unassigned|all), limit, offset, sort, period.
func (h *ReportHandler) HandleGetInbox(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		var params reportdto.InboxQueryParams
		_ = c.Bind().Query(&params)
		result, err := h.ReportService.GetInboxSnapshot(c.Context(), *orgID, &params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Inbox", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}
