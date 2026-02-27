// Package reporthdl - Handler cho Dashboard Order Processing (funnel, orders) và Inventory Intelligence (Tab 5).
package reporthdl

import (
	"strconv"
	"time"

	basehdl "meta_commerce/internal/api/base/handler"
	reportdto "meta_commerce/internal/api/report/dto"
	"meta_commerce/internal/api/report/layer3"
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

// HandleGetCustomers xử lý GET /dashboard/customers — snapshot Tab 4 Customer Intelligence (chỉ dùng CRM).
// Query: from, to, period, filter, limit, offset, sortField, sortOrder, journey, valueTier, lifecycle, loyalty, momentum, ceoGroup.
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
			Limit:     100000,
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

		// Ưu tiên KPI và phân bố từ report_snapshots (thống kê theo chu kỳ)
		snapshotSource := "realtime"
		var snapshotPeriodKey string
		var snapshotComputedAt int64
		var snapData *reportdto.CustomersDashboardSnapshotData
		snapData, snapshotPeriodKey, snapshotComputedAt, err = h.ReportService.GetSnapshotForCustomersDashboard(c.Context(), *orgID, &params)
		if err == nil && snapData != nil {
			snapshotSource = "report_snapshots"
		} else {
			snapData = buildCustomersDashboardDataFromCrm(allItems, total)
		}

		result := &reportdto.CustomersSnapshotResult{
			Summary:                snapData.Summary,
			ValueDistribution:      snapData.ValueDistribution,
			JourneyDistribution:    snapData.JourneyDistribution,
			LifecycleDistribution:  snapData.LifecycleDistribution,
			ChannelDistribution:    snapData.ChannelDistribution,
			LoyaltyDistribution:    snapData.LoyaltyDistribution,
			MomentumDistribution:   snapData.MomentumDistribution,
			CeoGroupDistribution:  snapData.CeoGroupDistribution,
			ValueLTV:               snapData.ValueLTV,
			JourneyLTV:             snapData.JourneyLTV,
			LifecycleLTV:           snapData.LifecycleLTV,
			ChannelLTV:             snapData.ChannelLTV,
			LoyaltyLTV:             snapData.LoyaltyLTV,
			MomentumLTV:            snapData.MomentumLTV,
			CeoGroupLTV:            snapData.CeoGroupLTV,
			FirstLayer3:            snapData.FirstLayer3,
			RepeatLayer3:           snapData.RepeatLayer3,
			VipLayer3:              snapData.VipLayer3,
			InactiveLayer3:         snapData.InactiveLayer3,
			EngagedLayer3:          snapData.EngagedLayer3,
			Customers:              paginateAndMapCustomers(allItems, offset, limit),
			VipInactiveCustomers:   buildVipInactiveFromCrm(allItems, params.VipInactiveLimit),
			TotalCount:             total,
			SnapshotSource:         snapshotSource,
			SnapshotPeriodKey:      snapshotPeriodKey,
			SnapshotComputedAt:     snapshotComputedAt,
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// buildCustomersDashboardDataFromCrm build KPI, phân bố và LTV theo nhóm từ danh sách CRM (fallback khi không có snapshot).
// Gồm cả phân bố Lớp 3 (First, Repeat, VIP, Inactive) để dashboard có đủ số tổng hợp.
func buildCustomersDashboardDataFromCrm(items []crmvc.CrmDashboardCustomerItem, total int) *reportdto.CustomersDashboardSnapshotData {
	var customersWithOrder, customersRepeat int64
	var reactivationValue, totalLTV, vipLTV float64
	valueDist := reportdto.ValueDistribution{}
	journeyDist := reportdto.JourneyDistribution{}
	lifecycleDist := reportdto.LifecycleDistribution{}
	channelDist := reportdto.ChannelDistribution{}
	loyaltyDist := reportdto.LoyaltyDistribution{}
	momentumDist := reportdto.MomentumDistribution{}
	ceoDist := reportdto.CeoGroupDistribution{}
	valueLTV := reportdto.ValueLTV{}
	journeyLTV := reportdto.JourneyLTV{}
	lifecycleLTV := reportdto.LifecycleLTV{}
	channelLTV := reportdto.ChannelLTV{}
	loyaltyLTV := reportdto.LoyaltyLTV{}
	momentumLTV := reportdto.MomentumLTV{}
	ceoGroupLTV := reportdto.CeoGroupLTV{}
	// Phân bố Lớp 3 — map tiêu chí -> số lượng
	firstPQ, firstEQ, firstEng, firstRT, firstRP := make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64)
	repeatRD, repeatRF, repeatSM, repeatPE, repeatEE, repeatUP := make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64)
	vipVD, vipST, vipPD, vipEL, vipRS := make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64)
	inactiveED, inactiveRP := make(map[string]int64), make(map[string]int64)
	engagedTemp, engagedDepth, engagedSource := make(map[string]int64), make(map[string]int64), make(map[string]int64)
	inc := func(m map[string]int64, k string) { m[k]++ }
	endMs := time.Now().UnixMilli()

	for _, it := range items {
		if it.OrderCount >= 1 {
			customersWithOrder++
		}
		if it.OrderCount >= 2 {
			customersRepeat++
		}
		if it.ValueTier == "vip" && (it.LifecycleStage == "inactive" || it.LifecycleStage == "dead") {
			reactivationValue += it.TotalSpend
		}
		totalLTV += it.TotalSpend
		if it.ValueTier == "vip" {
			vipLTV += it.TotalSpend
		}
		incValueDistribution(&valueDist, it.ValueTier)
		incJourneyDistribution(&journeyDist, it.JourneyStage)
		incLifecycleDistribution(&lifecycleDist, it.LifecycleStage, it.ValueTier)
		incChannelDistribution(&channelDist, it.Channel)
		incLoyaltyDistribution(&loyaltyDist, it.LoyaltyStage)
		incMomentumDistribution(&momentumDist, it.MomentumStage)
		incCeoDistribution(&ceoDist, it.ValueTier, it.LifecycleStage, it.JourneyStage, it.LoyaltyStage, it.MomentumStage)
		incValueLTV(&valueLTV, it.ValueTier, it.TotalSpend)
		incJourneyLTV(&journeyLTV, it.JourneyStage, it.TotalSpend)
		incLifecycleLTV(&lifecycleLTV, it.LifecycleStage, it.ValueTier, it.TotalSpend)
		incChannelLTV(&channelLTV, it.Channel, it.TotalSpend)
		incLoyaltyLTV(&loyaltyLTV, it.LoyaltyStage, it.TotalSpend)
		incMomentumLTV(&momentumLTV, it.MomentumStage, it.TotalSpend)
		incCeoGroupLTV(&ceoGroupLTV, it.ValueTier, it.LifecycleStage, it.JourneyStage, it.LoyaltyStage, it.MomentumStage, it.TotalSpend)
		// Lớp 3: derive và aggregate phân bố
		m := crmItemToLayer3Map(it)
		agg := layer3.DeriveFromMap(m, endMs)
		if agg != nil {
			if agg.First != nil {
				inc(firstPQ, agg.First.PurchaseQuality)
				inc(firstEQ, agg.First.ExperienceQuality)
				inc(firstEng, agg.First.EngagementAfterPurchase)
				inc(firstRT, agg.First.ReorderTiming)
				inc(firstRP, agg.First.RepeatProbability)
			}
			if agg.Repeat != nil {
				inc(repeatRD, agg.Repeat.RepeatDepth)
				inc(repeatRF, agg.Repeat.RepeatFrequency)
				inc(repeatSM, agg.Repeat.SpendMomentum)
				inc(repeatPE, agg.Repeat.ProductExpansion)
				inc(repeatEE, agg.Repeat.EmotionalEngagement)
				inc(repeatUP, agg.Repeat.UpgradePotential)
			}
			if agg.Vip != nil {
				inc(vipVD, agg.Vip.VipDepth)
				inc(vipST, agg.Vip.SpendTrend)
				inc(vipPD, agg.Vip.ProductDiversity)
				inc(vipEL, agg.Vip.EngagementLevel)
				inc(vipRS, agg.Vip.RiskScore)
			}
			if agg.Inactive != nil {
				inc(inactiveED, agg.Inactive.EngagementDrop)
				inc(inactiveRP, agg.Inactive.ReactivationPotential)
			}
			if agg.Engaged != nil {
				inc(engagedTemp, agg.Engaged.ConversationTemperature)
				inc(engagedDepth, agg.Engaged.EngagementDepth)
				inc(engagedSource, agg.Engaged.SourceType)
			}
		}
	}
	repeatRate := 0.0
	if customersWithOrder > 0 {
		repeatRate = float64(customersRepeat) / float64(customersWithOrder)
	}
	avgLTV := 0.0
	if total > 0 {
		avgLTV = totalLTV / float64(total)
	}
	return &reportdto.CustomersDashboardSnapshotData{
		Summary: reportdto.CustomerSummary{
			TotalCustomers:       int64(total),
			CustomersWithOrder:   customersWithOrder,
			CustomersRepeat:      customersRepeat,
			NewCustomersInPeriod: 0,
			RepeatRate:           repeatRate,
			VipInactiveCount:     ceoDist.VipInactive,
			ReactivationValue:    int64(reactivationValue),
			ActiveTodayCount:     0,
			TotalLTV:             totalLTV,
			AvgLTV:               avgLTV,
			VipLTV:               vipLTV,
		},
		ValueDistribution:     valueDist,
		JourneyDistribution:   journeyDist,
		LifecycleDistribution: lifecycleDist,
		ChannelDistribution:   channelDist,
		LoyaltyDistribution:   loyaltyDist,
		MomentumDistribution:  momentumDist,
		CeoGroupDistribution:  ceoDist,
		ValueLTV:              valueLTV,
		JourneyLTV:            journeyLTV,
		LifecycleLTV:          lifecycleLTV,
		ChannelLTV:            channelLTV,
		LoyaltyLTV:            loyaltyLTV,
		MomentumLTV:           momentumLTV,
		CeoGroupLTV:           ceoGroupLTV,
		FirstLayer3:           reportdto.FirstLayer3Distribution{PurchaseQuality: firstPQ, ExperienceQuality: firstEQ, EngagementAfterPurchase: firstEng, ReorderTiming: firstRT, RepeatProbability: firstRP},
		RepeatLayer3:          reportdto.RepeatLayer3Distribution{RepeatDepth: repeatRD, RepeatFrequency: repeatRF, SpendMomentum: repeatSM, ProductExpansion: repeatPE, EmotionalEngagement: repeatEE, UpgradePotential: repeatUP},
		VipLayer3:             reportdto.VipLayer3Distribution{VipDepth: vipVD, SpendTrend: vipST, ProductDiversity: vipPD, EngagementLevel: vipEL, RiskScore: vipRS},
		InactiveLayer3:       reportdto.InactiveLayer3Distribution{EngagementDrop: inactiveED, ReactivationPotential: inactiveRP},
		EngagedLayer3:        reportdto.EngagedLayer3Distribution{ConversationTemperature: engagedTemp, EngagementDepth: engagedDepth, SourceType: engagedSource},
	}
}

// crmItemToLayer3Map chuyển CrmDashboardCustomerItem sang map để layer3.DeriveFromMap có thể đọc.
// Keys theo chuẩn metricsSnapshot: journeyStage, orderCount, valueTier, lifecycleStage, totalSpent, avgOrderValue, lastOrderAt, secondLastOrderAt, lastConversationAt, cancelledOrderCount, revenueLast30d, ordersLast30d, ownedSkuCount, totalMessages, conversationFromAds (cho Engaged).
func crmItemToLayer3Map(it crmvc.CrmDashboardCustomerItem) map[string]interface{} {
	m := map[string]interface{}{
		"journeyStage":        it.JourneyStage,
		"orderCount":          it.OrderCount,
		"valueTier":           it.ValueTier,
		"lifecycleStage":      it.LifecycleStage,
		"totalSpent":          it.TotalSpend,
		"avgOrderValue":       it.AvgOrderValue,
		"lastOrderAt":         it.LastOrderAtMs,
		"secondLastOrderAt":   it.SecondLastOrderAt,
		"lastConversationAt":  it.LastConversationAt,
		"cancelledOrderCount": it.CancelledOrderCount,
		"revenueLast30d":      it.RevenueLast30d,
		"ordersLast30d":       it.OrdersLast30d,
		"ownedSkuCount":       it.OwnedSkuCount,
		"totalMessages":       it.TotalMessages,
		"conversationFromAds": it.ConversationFromAds,
	}
	return m
}

func incValueLTV(d *reportdto.ValueLTV, v string, spend float64) {
	switch v {
	case "vip": d.Vip += spend
	case "high": d.High += spend
	case "medium": d.Medium += spend
	case "low": d.Low += spend
	case "new", "": d.New += spend
	default: d.New += spend
	}
}
func incJourneyLTV(d *reportdto.JourneyLTV, v string, spend float64) {
	switch v {
	case "visitor": d.Visitor += spend
	case "engaged": d.Engaged += spend
	case "first": d.First += spend
	case "repeat": d.Repeat += spend
	case "vip": d.Vip += spend
	case "inactive", "": d.Inactive += spend
	default: d.Inactive += spend
	}
}
func incLifecycleLTV(d *reportdto.LifecycleLTV, lifecycle, valueTier string, spend float64) {
	switch lifecycle {
	case "active": d.Active += spend
	case "cooling": d.Cooling += spend
	case "inactive": d.Inactive += spend
	case "dead": d.Dead += spend
	case "never_purchased", "": d.NeverPurchased += spend
	default: d.NeverPurchased += spend
	}
}
func incChannelLTV(d *reportdto.ChannelLTV, v string, spend float64) {
	switch v {
	case "online": d.Online += spend
	case "offline": d.Offline += spend
	case "omnichannel": d.Omnichannel += spend
	case "", "_unspecified": d.Unspecified += spend
	default: d.Unspecified += spend
	}
}
func incLoyaltyLTV(d *reportdto.LoyaltyLTV, v string, spend float64) {
	switch v {
	case "core": d.Core += spend
	case "repeat": d.Repeat += spend
	case "one_time": d.OneTime += spend
	case "", "_unspecified": d.Unspecified += spend
	default: d.Unspecified += spend
	}
}
func incMomentumLTV(d *reportdto.MomentumLTV, v string, spend float64) {
	switch v {
	case "rising": d.Rising += spend
	case "stable": d.Stable += spend
	case "declining": d.Declining += spend
	case "lost": d.Lost += spend
	case "", "_unspecified": d.Unspecified += spend
	default: d.Unspecified += spend
	}
}
func incCeoGroupLTV(d *reportdto.CeoGroupLTV, valueTier, lifecycleStage, journeyStage, loyaltyStage, momentumStage string, spend float64) {
	ceoGroup := buildCeoGroupForLTV(valueTier, lifecycleStage, journeyStage, loyaltyStage, momentumStage)
	switch ceoGroup {
	case "vip_active": d.VipActive += spend
	case "vip_inactive": d.VipInactive += spend
	case "rising": d.Rising += spend
	case "new": d.New += spend
	case "one_time": d.OneTime += spend
	case "dead": d.Dead += spend
	default: d.Other += spend
	}
}
func buildCeoGroupForLTV(valueTier, lifecycleStage, journeyStage, loyaltyStage, momentumStage string) string {
	if valueTier == "vip" && lifecycleStage == "active" {
		return "vip_active"
	}
	if valueTier == "vip" && (lifecycleStage == "inactive" || lifecycleStage == "dead") {
		return "vip_inactive"
	}
	if momentumStage == "rising" {
		return "rising"
	}
	if journeyStage == "first" || valueTier == "new" {
		return "new"
	}
	if loyaltyStage == "one_time" {
		return "one_time"
	}
	if lifecycleStage == "dead" {
		return "dead"
	}
	return "_other"
}

func incValueDistribution(d *reportdto.ValueDistribution, v string) {
	switch v {
	case "vip": d.Vip++
	case "high": d.High++
	case "medium": d.Medium++
	case "low": d.Low++
	case "new", "": d.New++
	default: d.New++
	}
}
func incJourneyDistribution(d *reportdto.JourneyDistribution, v string) {
	switch v {
	case "visitor": d.Visitor++
	case "engaged": d.Engaged++
	case "first": d.First++
	case "repeat": d.Repeat++
	case "vip": d.Vip++
	case "inactive": d.Inactive++
	default: d.Visitor++
	}
}
func incLifecycleDistribution(d *reportdto.LifecycleDistribution, v, valueTier string) {
	switch v {
	case "active": d.Active++
	case "cooling": d.Cooling++
	case "inactive": d.Inactive++
	case "dead": d.Dead++
	case "never_purchased": d.NeverPurchased++
	default: d.NeverPurchased++
	}
}
func incChannelDistribution(d *reportdto.ChannelDistribution, v string) {
	switch v {
	case "online": d.Online++
	case "offline": d.Offline++
	case "omnichannel": d.Omnichannel++
	default: d.Unspecified++
	}
}
func incLoyaltyDistribution(d *reportdto.LoyaltyDistribution, v string) {
	switch v {
	case "core": d.Core++
	case "repeat": d.Repeat++
	case "one_time": d.OneTime++
	default: d.Unspecified++
	}
}
func incMomentumDistribution(d *reportdto.MomentumDistribution, v string) {
	switch v {
	case "rising": d.Rising++
	case "stable": d.Stable++
	case "declining": d.Declining++
	case "lost": d.Lost++
	default: d.Unspecified++
	}
}
func incCeoDistribution(d *reportdto.CeoGroupDistribution, valueTier, lifecycleStage, journeyStage, loyaltyStage, momentumStage string) {
	if valueTier == "vip" && lifecycleStage == "active" {
		d.VipActive++
	}
	if valueTier == "vip" && (lifecycleStage == "inactive" || lifecycleStage == "dead") {
		d.VipInactive++
	}
	if momentumStage == "rising" {
		d.Rising++
	}
	if journeyStage == "first" || valueTier == "new" {
		d.New++
	}
	if loyaltyStage == "one_time" {
		d.OneTime++
	}
	if lifecycleStage == "dead" {
		d.Dead++
	}
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
// Dùng layer3.DeriveFromMap thống nhất logic với snapshot aggregation, tránh duplicate.
func crmDashboardItemToCustomerItem(it crmvc.CrmDashboardCustomerItem) reportdto.CustomerItem {
	item := reportdto.CustomerItem{
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
	agg := layer3.DeriveFromMap(crmItemToLayer3Map(it), time.Now().UnixMilli())
	if agg != nil {
		if agg.First != nil {
			item.First = &reportdto.FirstMetrics{
				PurchaseQuality:        agg.First.PurchaseQuality,
				ExperienceQuality:      agg.First.ExperienceQuality,
				EngagementAfterPurchase: agg.First.EngagementAfterPurchase,
				ReorderTiming:          agg.First.ReorderTiming,
				RepeatProbability:      agg.First.RepeatProbability,
			}
		}
		if agg.Repeat != nil {
			item.Repeat = &reportdto.RepeatMetrics{
				RepeatDepth:         agg.Repeat.RepeatDepth,
				RepeatFrequency:     agg.Repeat.RepeatFrequency,
				SpendMomentum:       agg.Repeat.SpendMomentum,
				ProductExpansion:    agg.Repeat.ProductExpansion,
				EmotionalEngagement: agg.Repeat.EmotionalEngagement,
				UpgradePotential:    agg.Repeat.UpgradePotential,
			}
		}
		if agg.Vip != nil {
			item.Vip = &reportdto.VipMetrics{
				VipDepth:         agg.Vip.VipDepth,
				SpendTrend:       agg.Vip.SpendTrend,
				ProductDiversity: agg.Vip.ProductDiversity,
				EngagementLevel:  agg.Vip.EngagementLevel,
				RiskScore:        agg.Vip.RiskScore,
			}
		}
		if agg.Inactive != nil {
			item.Inactive = &reportdto.InactiveMetrics{
				EngagementDrop:        agg.Inactive.EngagementDrop,
				ReactivationPotential: agg.Inactive.ReactivationPotential,
			}
		}
	}
	return item
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

// HandleGetCustomersTrend xử lý GET /dashboard/customers/trend — snapshot hiện tại + trend data + comparison (% change vs kỳ trước).
// Bổ sung Customers và VipInactiveCustomers từ CrmCustomerService.
func (h *ReportHandler) HandleGetCustomersTrend(c fiber.Ctx) error {
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
		result, err := h.ReportService.GetCustomersTrendWithComparison(c.Context(), *orgID, &params)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Customer Trend: " + err.Error(), "status": "error",
			})
			return nil
		}
		// Bổ sung danh sách khách từ CRM nếu còn trống
		if result != nil && result.CurrentSnapshot != nil && len(result.CurrentSnapshot.Customers) == 0 {
			if params.Limit <= 0 { params.Limit = 20 }
			if params.Offset < 0 { params.Offset = 0 }
			if params.VipInactiveLimit <= 0 { params.VipInactiveLimit = 15 }
			limit := params.Limit
			if limit <= 0 { limit = 20 }
			sortField, sortOrder := reportdto.ParseCustomerSortParams(params.SortField, params.SortOrder)
			filters := &crmvc.CrmDashboardFilters{
				Journey:   crmvc.ParseFilterValues(params.Journey),
				Channel:   crmvc.ParseFilterValues(params.Channel),
				ValueTier: crmvc.ParseFilterValues(params.ValueTier),
				Lifecycle: crmvc.ParseFilterValues(params.Lifecycle),
				Loyalty:   crmvc.ParseFilterValues(params.Loyalty),
				Momentum:  crmvc.ParseFilterValues(params.Momentum),
				CeoGroup:  crmvc.ParseFilterValues(params.CeoGroup),
				Limit:     100000, Offset: 0, SortField: sortField, SortOrder: sortOrder,
			}
			allItems, total, err := h.CrmCustomerService.ListCustomersForDashboard(c.Context(), *orgID, filters)
			if err == nil {
				result.CurrentSnapshot.Customers = paginateAndMapCustomers(allItems, params.Offset, limit)
				result.CurrentSnapshot.VipInactiveCustomers = buildVipInactiveFromCrm(allItems, params.VipInactiveLimit)
				result.CurrentSnapshot.TotalCount = total
				if result.CurrentSnapshot.SnapshotSource == "realtime" {
					snapData := buildCustomersDashboardDataFromCrm(allItems, total)
					result.CurrentSnapshot.Summary = snapData.Summary
					result.CurrentSnapshot.ValueDistribution = snapData.ValueDistribution
					result.CurrentSnapshot.JourneyDistribution = snapData.JourneyDistribution
					result.CurrentSnapshot.LifecycleDistribution = snapData.LifecycleDistribution
					result.CurrentSnapshot.ChannelDistribution = snapData.ChannelDistribution
					result.CurrentSnapshot.LoyaltyDistribution = snapData.LoyaltyDistribution
					result.CurrentSnapshot.MomentumDistribution = snapData.MomentumDistribution
					result.CurrentSnapshot.CeoGroupDistribution = snapData.CeoGroupDistribution
					result.CurrentSnapshot.ValueLTV = snapData.ValueLTV
					result.CurrentSnapshot.JourneyLTV = snapData.JourneyLTV
					result.CurrentSnapshot.LifecycleLTV = snapData.LifecycleLTV
					result.CurrentSnapshot.ChannelLTV = snapData.ChannelLTV
					result.CurrentSnapshot.LoyaltyLTV = snapData.LoyaltyLTV
					result.CurrentSnapshot.MomentumLTV = snapData.MomentumLTV
					result.CurrentSnapshot.CeoGroupLTV = snapData.CeoGroupLTV
					result.CurrentSnapshot.FirstLayer3 = snapData.FirstLayer3
					result.CurrentSnapshot.RepeatLayer3 = snapData.RepeatLayer3
					result.CurrentSnapshot.VipLayer3 = snapData.VipLayer3
					result.CurrentSnapshot.InactiveLayer3 = snapData.InactiveLayer3
				}
			}
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetTransitionMatrix xử lý GET /dashboard/customers/trend/transition-matrix — ma trận chuyển đổi giữa 2 chu kỳ.
// Query: fromPeriod, toPeriod, dimension (journey|channel|value|lifecycle|loyalty|momentum|ceoGroup), sankey (true|false).
func (h *ReportHandler) HandleGetTransitionMatrix(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		fromPeriod := c.Query("fromPeriod")
		toPeriod := c.Query("toPeriod")
		if fromPeriod == "" || toPeriod == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu fromPeriod hoặc toPeriod (vd: 2025-01, 2025-02)", "status": "error",
			})
			return nil
		}
		dimension := c.Query("dimension", "value")
		allowedDim := map[string]bool{"journey": true, "channel": true, "value": true, "lifecycle": true, "loyalty": true, "momentum": true, "ceoGroup": true}
		if !allowedDim[dimension] { dimension = "value" }
		includeSankey := c.Query("sankey") == "true"
		result, err := h.ReportService.GetTransitionMatrix(c.Context(), *orgID, fromPeriod, toPeriod, dimension, includeSankey)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Transition matrix: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
}

// HandleGetGroupChanges xử lý GET /dashboard/customers/trend/group-changes — chi tiết khách chuyển nhóm (up/down/unchanged).
// Query: fromPeriod, toPeriod, dimension (journey|channel|value|lifecycle|loyalty|momentum|ceoGroup).
func (h *ReportHandler) HandleGetGroupChanges(c fiber.Ctx) error {
	return basehdl.SafeHandlerWrapper(c, func() error {
		orgID := getActiveOrganizationID(c)
		if orgID == nil || orgID.IsZero() {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Vui lòng chọn tổ chức (active organization)", "status": "error",
			})
			return nil
		}
		fromPeriod := c.Query("fromPeriod")
		toPeriod := c.Query("toPeriod")
		if fromPeriod == "" || toPeriod == "" {
			c.Status(common.StatusBadRequest).JSON(fiber.Map{
				"code": common.ErrCodeValidationInput.Code, "message": "Thiếu fromPeriod hoặc toPeriod", "status": "error",
			})
			return nil
		}
		dimension := c.Query("dimension", "value")
		allowedDim := map[string]bool{"journey": true, "channel": true, "value": true, "lifecycle": true, "loyalty": true, "momentum": true, "ceoGroup": true}
		if !allowedDim[dimension] { dimension = "value" }
		result, err := h.ReportService.GetGroupChanges(c.Context(), *orgID, fromPeriod, toPeriod, dimension)
		if err != nil {
			c.Status(common.StatusInternalServerError).JSON(fiber.Map{
				"code": common.ErrCodeDatabase.Code, "message": "Lỗi truy vấn Group changes: " + err.Error(), "status": "error",
			})
			return nil
		}
		c.Status(common.StatusOK).JSON(fiber.Map{
			"code": common.StatusOK, "message": "Thành công", "data": result, "status": "success",
		})
		return nil
	})
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
// Query: pageId, filter (backlog|unassigned|all|engaged), limit, offset, sort (waiting_desc|updated_desc|updated_asc|care_priority), period, engaged (bool).
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
