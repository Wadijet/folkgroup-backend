// Package reporthdl - Handler cho Dashboard Order Processing (funnel snapshot, recent orders).
package reporthdl

import (
	"strconv"

	basehdl "meta_commerce/internal/api/base/handler"
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

// HandleGetStuckOrders xử lý GET /dashboard/orders/stuck-orders — danh sách đơn vượt SLA.
// Query: limit (mặc định 50, max 200), stage (lọc theo stage, optional).
func (h *ReportHandler) HandleGetStuckOrders(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		limit := 50
		if s := c.Query("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
				if limit > 200 {
					limit = 200
				}
			}
		}
		stageFilter := c.Query("stage")
		items, err := h.ReportService.GetStuckOrders(c.Context(), *orgID, limit, stageFilter)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn đơn vượt SLA", "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{"items": items}, "status": "success",
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
