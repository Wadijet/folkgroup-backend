// Package reporthdl - Handler cho Dashboard Order Processing (funnel, orders) và Inventory Intelligence (Tab 5).
package reporthdl

import (
	"strconv"

	basehdl "meta_commerce/internal/api/base/handler"
	reportdto "meta_commerce/internal/api/report/dto"
	crmvc "meta_commerce/internal/api/crm/service"
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
// Query: from, to, period, filter, limit, offset, sortField, sortOrder, journey, valueTier, lifecycle, loyalty, momentum, ceoGroup, source (crm|legacy).
// Mặc định source=crm (dùng crm_customers). Dùng source=legacy để dùng pc_pos_customers (format cũ).
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
		source := params.Source
		if source == "" {
			source = "crm"
		}

		if source == "legacy" {
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
		}

		// source=crm: dùng CrmCustomerService
		limit := params.Limit
		if limit <= 0 {
			limit = 20
		}
		offset := params.Offset
		if offset < 0 {
			offset = 0
		}
		sortField, sortOrder := reportdto.ParseCustomerSortParams(params.SortField, params.SortOrder)
		filters := &crmvc.CrmDashboardFilters{
			Journey:   crmvc.ParseFilterValues(params.Journey),
			Channel:   crmvc.ParseFilterValues(params.Channel),
			ValueTier: crmvc.ParseFilterValues(params.ValueTier),
			Lifecycle: crmvc.ParseFilterValues(params.Lifecycle),
			Loyalty:   crmvc.ParseFilterValues(params.Loyalty),
			Momentum:  crmvc.ParseFilterValues(params.Momentum),
			CeoGroup:  crmvc.ParseFilterValues(params.CeoGroup),
			Limit:     100000, // Lấy toàn bộ để tính aggregates, paginate ở handler
			Offset:    0,
			SortField: sortField,
			SortOrder: sortOrder,
		}
		allItems, total, err := h.CrmCustomerService.ListCustomersForDashboard(c.Context(), *orgID, filters)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Customer Intelligence: " + err.Error(), "status": "error",
			})
			return nil
		}

		// Build summary và distribution từ toàn bộ danh sách
		var customersWithOrder, customersRepeat int64
		for _, it := range allItems {
			if it.OrderCount >= 1 {
				customersWithOrder++
			}
			if it.OrderCount >= 2 {
				customersRepeat++
			}
		}
		repeatRate := 0.0
		if customersWithOrder > 0 {
			repeatRate = float64(customersRepeat) / float64(customersWithOrder)
		}

		// Value tier distribution
		tierDist := reportdto.TierDistribution{}
		lifecycleDist := reportdto.LifecycleDistribution{}
		for _, it := range allItems {
			switch it.ValueTier {
			case "vip":
				tierDist.Platinum++
			case "high":
				tierDist.Gold++
			case "medium":
				tierDist.Silver++
			case "low", "new":
				tierDist.New++
			}
			switch it.LifecycleStage {
			case "active":
				lifecycleDist.Active++
			case "cooling":
				lifecycleDist.Cooling++
			case "inactive":
				if it.ValueTier == "vip" {
					lifecycleDist.VipInactive++
				} else {
					lifecycleDist.Inactive++
				}
			case "dead":
				if it.ValueTier == "vip" {
					lifecycleDist.VipInactive++
				} else {
					lifecycleDist.Inactive++
				}
			case "never_purchased":
				lifecycleDist.NeverPurchased++
			}
		}

		vipInactiveCount := lifecycleDist.VipInactive
		var reactivationValue int64
		for _, it := range allItems {
			if it.ValueTier == "vip" && (it.LifecycleStage == "inactive" || it.LifecycleStage == "dead") {
				reactivationValue += int64(it.TotalSpend)
			}
		}

		result := &reportdto.CustomersSnapshotResult{
			Summary: reportdto.CustomerSummary{
				TotalCustomers:       int64(total),
				NewCustomersInPeriod: 0,
				RepeatRate:           repeatRate,
				VipInactiveCount:     vipInactiveCount,
				ReactivationValue:    reactivationValue,
				ActiveTodayCount:     0,
			},
			TierDistribution:      tierDist,
			LifecycleDistribution: lifecycleDist,
			Customers:            paginateAndMapCustomers(allItems, offset, limit),
			VipInactiveCustomers: buildVipInactiveFromCrm(allItems, params.VipInactiveLimit),
			TotalCount:           total,
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// paginateAndMapCustomers lấy slice con theo offset/limit và map sang CustomerItem.
func paginateAndMapCustomers(items []crmvc.CrmDashboardCustomerItem, offset, limit int) []reportdto.CustomerItem {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []reportdto.CustomerItem{}
	}
	to := offset + limit
	if to > len(items) {
		to = len(items)
	}
	page := items[offset:to]
	result := make([]reportdto.CustomerItem, len(page))
	for i := range page {
		result[i] = crmDashboardItemToCustomerItem(page[i])
	}
	return result
}

// crmDashboardItemToCustomerItem map CrmDashboardCustomerItem sang CustomerItem.
func crmDashboardItemToCustomerItem(it crmvc.CrmDashboardCustomerItem) reportdto.CustomerItem {
	return reportdto.CustomerItem{
		CustomerID:       it.CustomerID,
		Name:             it.Name,
		Phone:            it.Phone,
		TotalSpend:       it.TotalSpend,
		OrderCount:       int64(it.OrderCount),
		LastOrderAt:      it.LastOrderAt,
		DaysSinceLast:    it.DaysSinceLast,
		Lifecycle:        it.LifecycleStage,
		JourneyStage:     it.JourneyStage,
		Channel:          it.Channel,
		ValueTier:        it.ValueTier,
		LifecycleStage:   it.LifecycleStage,
		LoyaltyStage:     it.LoyaltyStage,
		MomentumStage:    it.MomentumStage,
		RevenueLast30d:   it.RevenueLast30d,
		RevenueLast90d:   it.RevenueLast90d,
		AvgOrderValue:    it.AvgOrderValue,
		Sources:          it.Sources,
	}
}

// buildVipInactiveFromCrm lấy top VIP inactive từ danh sách CRM.
func buildVipInactiveFromCrm(items []crmvc.CrmDashboardCustomerItem, limit int) []reportdto.VipInactiveItem {
	if limit <= 0 {
		limit = 15
	}
	var vipInactive []crmvc.CrmDashboardCustomerItem
	for _, it := range items {
		if it.ValueTier == "vip" && (it.LifecycleStage == "inactive" || it.LifecycleStage == "dead") {
			vipInactive = append(vipInactive, it)
		}
	}
	// Sort by totalSpend desc
	for i := 0; i < len(vipInactive)-1; i++ {
		for j := i + 1; j < len(vipInactive); j++ {
			if vipInactive[j].TotalSpend > vipInactive[i].TotalSpend {
				vipInactive[i], vipInactive[j] = vipInactive[j], vipInactive[i]
			}
		}
	}
	result := make([]reportdto.VipInactiveItem, 0, limit)
	for i := 0; i < len(vipInactive) && i < limit; i++ {
		result = append(result, reportdto.VipInactiveItem{
			CustomerID:    vipInactive[i].CustomerID,
			Name:          vipInactive[i].Name,
			TotalSpend:    vipInactive[i].TotalSpend,
			DaysSinceLast: vipInactive[i].DaysSinceLast,
		})
	}
	return result
}

// HandleGetCeoGroups xử lý GET /dashboard/customers/ceo-groups — 6 nhóm CEO: count + top items.
func (h *ReportHandler) HandleGetCeoGroups(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		topLimit := 5
		if s := c.Query("topLimit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				topLimit = n
				if topLimit > 20 {
					topLimit = 20
				}
			}
		}
		result, err := h.CrmCustomerService.GetCeoGroups(c.Context(), *orgID, topLimit)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn CEO groups: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetJourneyFunnel xử lý GET /dashboard/customers/journey-funnel — số lượng từng stage Journey.
func (h *ReportHandler) HandleGetJourneyFunnel(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		result, err := h.CrmCustomerService.GetJourneyFunnel(c.Context(), *orgID)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Journey funnel: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetAssetMatrix xử lý GET /dashboard/customers/asset-matrix — ma trận Value × Lifecycle.
func (h *ReportHandler) HandleGetAssetMatrix(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		result, err := h.CrmCustomerService.GetAssetMatrix(c.Context(), *orgID)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Asset matrix: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetMatrixJourneyValue xử lý GET /dashboard/customers/matrix-journey-value — ma trận Journey × L2.
// Query: cols=channel|value|lifecycle|loyalty|momentum (L2 trục cột).
func (h *ReportHandler) HandleGetMatrixJourneyValue(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		cols := c.Query("cols", "value")
		result, err := h.CrmCustomerService.GetMatrixJourneyValue(c.Context(), *orgID, cols)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn matrix: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{"matrix": result.Matrix, "rows": result.Rows, "cols": result.Cols, "total": result.Total}, "status": "success",
		})
		return nil
	})
}

// HandleGetMatrixValueLoyalty xử lý GET /dashboard/customers/matrix-value-loyalty — ma trận L2 × L2.
// Query: rows=channel|value|lifecycle|loyalty|momentum, cols=...
func (h *ReportHandler) HandleGetMatrixValueLoyalty(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		rows := c.Query("rows", "value")
		cols := c.Query("cols", "loyalty")
		result, err := h.CrmCustomerService.GetMatrixValueLoyalty(c.Context(), *orgID, rows, cols)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn matrix: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": fiber.Map{"matrix": result.Matrix, "rows": result.Rows, "cols": result.Cols, "total": result.Total}, "status": "success",
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
