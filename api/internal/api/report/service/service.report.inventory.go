// Package reportsvc - Inventory Intelligence (Tab 5): snapshot tồn kho, KPI, bảng, days cover distribution, alerts.
package reportsvc

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	reportdto "meta_commerce/internal/api/report/dto"
	"meta_commerce/internal/common"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Bucket keys cho days cover distribution.
var daysCoverBucketKeys = []string{"0", "1-7", "8-14", "15-30", "31-60", "61-90", "90+", "infinity"}

// orderStatusCancelled: đơn hủy/xóa — loại trừ khỏi aggregate daily sales và days since last sale.
// Chỉ trừ 6 (Đã hủy), 7 (Đã xóa gần đây).
var orderStatusCancelled = []int{6, 7}

// GetInventorySnapshot trả về snapshot tồn kho cho Tab 5 Inventory Intelligence.
// Bao gồm: 4 KPI, bảng items (variation-warehouse), days cover distribution, alerts.
func (s *ReportService) GetInventorySnapshot(ctx context.Context, ownerOrganizationID primitive.ObjectID, params *reportdto.InventoryQueryParams) (*reportdto.InventorySnapshotResult, error) {
	if params == nil {
		params = &reportdto.InventoryQueryParams{}
	}
	applyInventoryDefaults(params)

	fromTime, toTime, daysInPeriod, err := parseInventoryPeriod(params)
	if err != nil {
		return nil, fmt.Errorf("parse period: %w", err)
	}
	if daysInPeriod < 1 {
		daysInPeriod = 1
	}

	// Load variations
	variations, err := s.loadVariationsForInventory(ctx, ownerOrganizationID)
	if err != nil {
		return nil, err
	}

	// Load products, warehouses
	productNames := make(map[string]string)
	warehouseNames := make(map[string]string)
	productIds := make(map[string]bool)
	warehouseIds := make(map[string]bool)
	for _, v := range variations {
		if v.ProductId != "" {
			productIds[v.ProductId] = true
		}
		for _, whID := range v.WarehouseIds {
			warehouseIds[whID] = true
		}
	}
	if err := s.loadProductNames(ctx, ownerOrganizationID, productIds, productNames); err != nil {
		return nil, err
	}
	if err := s.loadWarehouseNames(ctx, ownerOrganizationID, warehouseIds, warehouseNames); err != nil {
		return nil, err
	}

	// Aggregate daily sales từ orders
	dailySalesByVariation, err := s.aggregateDailySales(ctx, ownerOrganizationID, fromTime, toTime, daysInPeriod)
	if err != nil {
		return nil, err
	}
	// Lấy ngày bán cuối cùng theo variation (để tính daysSinceLastSale)
	lastSaleByVariation, _ := s.getLastSaleDateByVariation(ctx, ownerOrganizationID)

	// Build items
	var items []reportdto.InventoryItem
	skuSet := make(map[string]bool)
	var totalValue float64
	var lowStockCount, outOfStockCount, deadStockCount, slowMovingCount int64
	var deadStockValue, slowMovingValue float64

	for _, v := range variations {
		productName := productNames[v.ProductId]
		if productName == "" {
			productName = "Không xác định"
		}
		dailySales := dailySalesByVariation[v.VariationId]
		unitPrice := v.UnitPrice

		if len(v.Warehouses) > 0 {
			for _, wh := range v.Warehouses {
				remain := wh.RemainQuantity
				isSellNegative := remain == -1
				whName := warehouseNames[wh.WarehouseId]
				if whName == "" {
					whName = "Không xác định"
				}
				sellingAvg := wh.SellingAvg
				if sellingAvg <= 0 && dailySales > 0 {
					sellingAvg = dailySales
				}
				daysSinceLastSale := computeDaysSinceLastSale(lastSaleByVariation[v.VariationId])
				item := buildInventoryItem(v, wh, productName, whName, sellingAvg, unitPrice, params.LowStockThreshold, params.LowStockDaysCover, params.SlowMovingDays, params.AtRiskDays, daysSinceLastSale)
				items = append(items, item)
				if !skuSet[v.VariationId] {
					skuSet[v.VariationId] = true
				}
				if !isSellNegative && remain >= 0 {
					totalValue += item.InventoryValue
					if item.Status == "low_stock" {
						lowStockCount++
					}
					if item.Status == "out_of_stock" {
						outOfStockCount++
					}
					if item.EfficiencyStatus == "dead_stock" {
						deadStockCount++
						deadStockValue += item.InventoryValue
					}
					if item.EfficiencyStatus == "slow_moving" {
						slowMovingCount++
						slowMovingValue += item.InventoryValue
					}
				}
			}
		} else {
			remain := v.TotalRemain
			isSellNegative := remain == -1
			daysSinceLastSale := computeDaysSinceLastSale(lastSaleByVariation[v.VariationId])
			item := buildInventoryItemTotal(v, productName, dailySales, unitPrice, params.LowStockThreshold, params.LowStockDaysCover, params.SlowMovingDays, params.AtRiskDays, daysSinceLastSale)
			items = append(items, item)
			if !skuSet[v.VariationId] {
				skuSet[v.VariationId] = true
			}
			if !isSellNegative && remain >= 0 {
				totalValue += item.InventoryValue
				if item.Status == "low_stock" {
					lowStockCount++
				}
				if item.Status == "out_of_stock" {
					outOfStockCount++
				}
				if item.EfficiencyStatus == "dead_stock" {
					deadStockCount++
					deadStockValue += item.InventoryValue
				}
				if item.EfficiencyStatus == "slow_moving" {
					slowMovingCount++
					slowMovingValue += item.InventoryValue
				}
			}
		}
	}

	// Build distribution
	dist := make(map[string]int64)
	for _, k := range daysCoverBucketKeys {
		dist[k] = 0
	}
	for _, it := range items {
		bucket := getDaysCoverBucket(it.DaysCover, it.IsSellNegative)
		dist[bucket]++
	}

	// Build alerts
	var critical, warning []reportdto.InventoryAlertItem
	for _, it := range items {
		if it.Status == "out_of_stock" {
			critical = append(critical, reportdto.InventoryAlertItem{Sku: it.Sku, ProductName: it.ProductName, WarehouseName: it.WarehouseName})
		} else if it.Status == "low_stock" {
			warning = append(warning, reportdto.InventoryAlertItem{Sku: it.Sku, ProductName: it.ProductName, WarehouseName: it.WarehouseName, DaysCover: it.DaysCover})
		}
	}
	sort.Slice(critical, func(i, j int) bool {
		return critical[i].Sku < critical[j].Sku
	})
	sort.Slice(warning, func(i, j int) bool {
		if warning[i].DaysCover >= 0 && warning[j].DaysCover >= 0 {
			return warning[i].DaysCover < warning[j].DaysCover
		}
		return warning[i].Sku < warning[j].Sku
	})
	if len(critical) > 10 {
		critical = critical[:10]
	}
	if len(warning) > 10 {
		warning = warning[:10]
	}

	// Filter
	items = filterInventoryItems(items, params.Status, params.Efficiency, params.WarehouseID)
	// Sort
	sortInventoryItems(items, params.Sort)
	// Phân trang theo chuẩn PaginateResult: page, limit, itemCount, total, totalPage
	total := int64(len(items))
	page := int64(params.Page)
	if page < 1 {
		page = 1
	}
	limit := int64(params.Limit)
	if limit <= 0 {
		limit = 50
	}
	skip := (page - 1) * limit
	var pagedItems []reportdto.InventoryItem
	if skip < total {
		end := skip + limit
		if end > total {
			end = total
		}
		pagedItems = items[int(skip):int(end)]
	} else {
		pagedItems = []reportdto.InventoryItem{}
	}
	totalPage := int64(0)
	if total > 0 {
		totalPage = (total + limit - 1) / limit
	}

	summaryStatuses := computeSummaryStatuses(int64(totalValue), int64(len(skuSet)), lowStockCount, outOfStockCount, deadStockCount, slowMovingCount)

	return &reportdto.InventorySnapshotResult{
		Summary: reportdto.InventorySummary{
			TotalInventoryValue: int64(totalValue),
			SkuCount:            int64(len(skuSet)),
			LowStockCount:       lowStockCount,
			OutOfStockCount:     outOfStockCount,
			DeadStockCount:      deadStockCount,
			DeadStockValue:      int64(deadStockValue),
			SlowMovingCount:     slowMovingCount,
			SlowMovingValue:     int64(slowMovingValue),
		},
		SummaryStatuses:       summaryStatuses,
		Items:                 pagedItems,
		Page:                  page,
		Limit:                 limit,
		ItemCount:             int64(len(pagedItems)),
		Total:                 total,
		TotalPage:             totalPage,
		DaysCoverDistribution: dist,
		Alerts: reportdto.InventoryAlerts{
			Critical: critical,
			Warning:  warning,
		},
	}, nil
}

// GetInventoryProducts trả về danh sách sản phẩm có phân trang — level 1 của tree (lazy load).
// Khi user expand 1 sản phẩm, gọi GetInventoryProductVariations(productId) để lấy mẫu mã.
func (s *ReportService) GetInventoryProducts(ctx context.Context, ownerOrganizationID primitive.ObjectID, params *reportdto.InventoryProductsQueryParams) (*reportdto.InventoryProductsResult, error) {
	if params == nil {
		params = &reportdto.InventoryProductsQueryParams{}
	}
	applyInventoryProductsDefaults(params)

	fromTime, toTime, daysInPeriod, err := parseInventoryPeriodFromParams(params.From, params.To, params.Period)
	if err != nil {
		return nil, fmt.Errorf("parse period: %w", err)
	}
	if daysInPeriod < 1 {
		daysInPeriod = 1
	}

	variations, err := s.loadVariationsForInventory(ctx, ownerOrganizationID)
	if err != nil {
		return nil, err
	}
	productIds := make(map[string]bool)
	warehouseIds := make(map[string]bool)
	for _, v := range variations {
		if v.ProductId != "" {
			productIds[v.ProductId] = true
		}
		for _, whID := range v.WarehouseIds {
			warehouseIds[whID] = true
		}
	}
	productNames := make(map[string]string)
	if err := s.loadProductNames(ctx, ownerOrganizationID, productIds, productNames); err != nil {
		return nil, err
	}
	// Map productId -> ảnh variation đầu tiên (fallback khi product không có ảnh)
	firstVariationImage := make(map[string]string)
	for _, v := range variations {
		if v.ImageUrl != "" && firstVariationImage[v.ProductId] == "" {
			firstVariationImage[v.ProductId] = v.ImageUrl
		}
	}
	productDetailsMap, _ := s.loadProductDetails(ctx, ownerOrganizationID, productIds, firstVariationImage)
	warehouseNames := make(map[string]string)
	if err := s.loadWarehouseNames(ctx, ownerOrganizationID, warehouseIds, warehouseNames); err != nil {
		return nil, err
	}
	dailySalesByVariation, err := s.aggregateDailySales(ctx, ownerOrganizationID, fromTime, toTime, daysInPeriod)
	if err != nil {
		return nil, err
	}
	lastSaleByVariation, _ := s.getLastSaleDateByVariation(ctx, ownerOrganizationID)

	// Build items (variation-warehouse) rồi group theo product
	var allItems []reportdto.InventoryItem
	for _, v := range variations {
		productName := productNames[v.ProductId]
		if productName == "" {
			productName = "Không xác định"
		}
		dailySales := dailySalesByVariation[v.VariationId]
		unitPrice := v.UnitPrice
		daysSinceLastSale := computeDaysSinceLastSale(lastSaleByVariation[v.VariationId])
		if len(v.Warehouses) > 0 {
			for _, wh := range v.Warehouses {
				whName := warehouseNames[wh.WarehouseId]
				if whName == "" {
					whName = "Không xác định"
				}
				sellingAvg := wh.SellingAvg
				if sellingAvg <= 0 && dailySales > 0 {
					sellingAvg = dailySales
				}
				item := buildInventoryItem(v, wh, productName, whName, sellingAvg, unitPrice, params.LowStockThreshold, params.LowStockDaysCover, params.SlowMovingDays, params.AtRiskDays, daysSinceLastSale)
				allItems = append(allItems, item)
			}
		} else {
			item := buildInventoryItemTotal(v, productName, dailySales, unitPrice, params.LowStockThreshold, params.LowStockDaysCover, params.SlowMovingDays, params.AtRiskDays, daysSinceLastSale)
			allItems = append(allItems, item)
		}
	}
	allItems = filterInventoryItems(allItems, params.Status, params.Efficiency, params.WarehouseID)

	// Group theo productId
	type productAgg struct {
		productName     string
		variationCount  int
		totalRemain     int64
		lowStockCount   int64
		outOfStockCount int64
		deadStockCount  int64
		slowMovingCount int64
		inventoryValue  float64
	}
	byProduct := make(map[string]*productAgg)
	variationIdsSeen := make(map[string]map[string]bool) // productId -> set of variationId
	for _, it := range allItems {
		if byProduct[it.ProductId] == nil {
			byProduct[it.ProductId] = &productAgg{productName: it.ProductName}
			variationIdsSeen[it.ProductId] = make(map[string]bool)
		}
		agg := byProduct[it.ProductId]
		variationIdsSeen[it.ProductId][it.VariationId] = true
		agg.totalRemain += it.RemainQuantity
		agg.inventoryValue += it.InventoryValue
		if it.Status == "low_stock" {
			agg.lowStockCount++
		}
		if it.Status == "out_of_stock" {
			agg.outOfStockCount++
		}
		if it.EfficiencyStatus == "dead_stock" {
			agg.deadStockCount++
		}
		if it.EfficiencyStatus == "slow_moving" {
			agg.slowMovingCount++
		}
	}
	for pid, vids := range variationIdsSeen {
		byProduct[pid].variationCount = len(vids)
	}

	// Build product list và sort
	var products []reportdto.InventoryProductItem
	for pid, agg := range byProduct {
		productStatus := "ok"
		if agg.outOfStockCount > 0 {
			productStatus = "critical"
		} else if agg.lowStockCount > 0 {
			productStatus = "warning"
		}
		det := productDetailsMap[pid]
		products = append(products, reportdto.InventoryProductItem{
			ProductId:       pid,
			ProductName:     agg.productName,
			VariationCount:  agg.variationCount,
			TotalRemain:     agg.totalRemain,
			LowStockCount:   agg.lowStockCount,
			OutOfStockCount: agg.outOfStockCount,
			DeadStockCount:  agg.deadStockCount,
			SlowMovingCount: agg.slowMovingCount,
			InventoryValue:  agg.inventoryValue,
			ProductStatus:   productStatus,
			ImageUrl:        det.ImageUrl,
			CategoryId:      det.CategoryId,
			CategoryName:    det.CategoryName,
		})
	}
	sortInventoryProducts(products, params.Sort)

	total := int64(len(products))
	page := int64(params.Page)
	if page < 1 {
		page = 1
	}
	limit := int64(params.Limit)
	if limit <= 0 {
		limit = 50
	}
	skip := (page - 1) * limit
	var paged []reportdto.InventoryProductItem
	if skip < total {
		end := skip + limit
		if end > total {
			end = total
		}
		paged = products[int(skip):int(end)]
	} else {
		paged = []reportdto.InventoryProductItem{}
	}
	totalPage := int64(0)
	if total > 0 {
		totalPage = (total + limit - 1) / limit
	}

	return &reportdto.InventoryProductsResult{
		Items:     paged,
		Page:      page,
		Limit:     limit,
		ItemCount: int64(len(paged)),
		Total:     total,
		TotalPage: totalPage,
	}, nil
}

// GetInventoryProductVariations trả về danh sách mẫu mã (variation-warehouse) của 1 sản phẩm — level 2 của tree.
// Gọi khi user expand 1 sản phẩm.
func (s *ReportService) GetInventoryProductVariations(ctx context.Context, ownerOrganizationID primitive.ObjectID, productId string, params *reportdto.InventoryVariationsQueryParams) (*reportdto.InventoryVariationsResult, error) {
	if productId == "" {
		return nil, fmt.Errorf("productId không được để trống")
	}
	if params == nil {
		params = &reportdto.InventoryVariationsQueryParams{}
	}
	applyInventoryVariationsDefaults(params)

	fromTime, toTime, daysInPeriod, err := parseInventoryPeriodFromParams(params.From, params.To, params.Period)
	if err != nil {
		return nil, fmt.Errorf("parse period: %w", err)
	}
	if daysInPeriod < 1 {
		daysInPeriod = 1
	}

	variations, err := s.loadVariationsForInventoryFiltered(ctx, ownerOrganizationID, productId)
	if err != nil {
		return nil, err
	}
	if len(variations) == 0 {
		productName := "Không xác định"
		if pn, err := s.getProductName(ctx, ownerOrganizationID, productId); err == nil && pn != "" {
			productName = pn
		}
		return &reportdto.InventoryVariationsResult{
			ProductId:   productId,
			ProductName: productName,
			Items:       []reportdto.InventoryItem{},
		}, nil
	}

	warehouseIds := make(map[string]bool)
	for _, v := range variations {
		for _, whID := range v.WarehouseIds {
			warehouseIds[whID] = true
		}
	}
	productNames := make(map[string]string)
	productNames[productId] = "Không xác định"
	if err := s.loadProductNames(ctx, ownerOrganizationID, map[string]bool{productId: true}, productNames); err == nil {
		if productNames[productId] == "" {
			productNames[productId] = "Không xác định"
		}
	}
	warehouseNames := make(map[string]string)
	if err := s.loadWarehouseNames(ctx, ownerOrganizationID, warehouseIds, warehouseNames); err != nil {
		return nil, err
	}
	dailySalesByVariation, err := s.aggregateDailySales(ctx, ownerOrganizationID, fromTime, toTime, daysInPeriod)
	if err != nil {
		return nil, err
	}
	lastSaleByVariation, _ := s.getLastSaleDateByVariation(ctx, ownerOrganizationID)

	productName := productNames[productId]
	if productName == "" {
		productName = "Không xác định"
	}

	var items []reportdto.InventoryItem
	for _, v := range variations {
		dailySales := dailySalesByVariation[v.VariationId]
		unitPrice := v.UnitPrice
		daysSinceLastSale := computeDaysSinceLastSale(lastSaleByVariation[v.VariationId])
		if len(v.Warehouses) > 0 {
			for _, wh := range v.Warehouses {
				whName := warehouseNames[wh.WarehouseId]
				if whName == "" {
					whName = "Không xác định"
				}
				sellingAvg := wh.SellingAvg
				if sellingAvg <= 0 && dailySales > 0 {
					sellingAvg = dailySales
				}
				item := buildInventoryItem(v, wh, productName, whName, sellingAvg, unitPrice, params.LowStockThreshold, params.LowStockDaysCover, params.SlowMovingDays, params.AtRiskDays, daysSinceLastSale)
				items = append(items, item)
			}
		} else {
			item := buildInventoryItemTotal(v, productName, dailySales, unitPrice, params.LowStockThreshold, params.LowStockDaysCover, params.SlowMovingDays, params.AtRiskDays, daysSinceLastSale)
			items = append(items, item)
		}
	}
	items = filterInventoryItems(items, params.Status, params.Efficiency, params.WarehouseID)
	sortInventoryItems(items, params.Sort)

	return &reportdto.InventoryVariationsResult{
		ProductId:   productId,
		ProductName: productName,
		Items:       items,
	}, nil
}

// getProductName lấy tên sản phẩm theo productId.
func (s *ReportService) getProductName(ctx context.Context, ownerOrgID primitive.ObjectID, productId string) (string, error) {
	out := make(map[string]string)
	if err := s.loadProductNames(ctx, ownerOrgID, map[string]bool{productId: true}, out); err != nil {
		return "", err
	}
	return out[productId], nil
}

func sortInventoryProducts(products []reportdto.InventoryProductItem, sortBy string) {
	switch sortBy {
	case "variation_count":
		sort.Slice(products, func(i, j int) bool { return products[i].VariationCount > products[j].VariationCount })
	case "total_remain":
		sort.Slice(products, func(i, j int) bool { return products[i].TotalRemain > products[j].TotalRemain })
	case "inventory_value":
		sort.Slice(products, func(i, j int) bool { return products[i].InventoryValue > products[j].InventoryValue })
	default:
		sort.Slice(products, func(i, j int) bool { return products[i].ProductName < products[j].ProductName })
	}
}

func applyInventoryProductsDefaults(p *reportdto.InventoryProductsQueryParams) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
	if p.LowStockThreshold <= 0 {
		p.LowStockThreshold = 10
	}
	if p.LowStockDaysCover <= 0 {
		p.LowStockDaysCover = 7
	}
	if p.SlowMovingDays <= 0 {
		p.SlowMovingDays = 90
	}
	if p.AtRiskDays <= 0 {
		p.AtRiskDays = 60
	}
	if p.Period == "" {
		p.Period = "90d"
	}
	if p.Status == "" {
		p.Status = "all"
	}
	if p.Efficiency == "" {
		p.Efficiency = "all"
	}
	if p.Sort == "" {
		p.Sort = "product_name"
	}
}

func applyInventoryVariationsDefaults(p *reportdto.InventoryVariationsQueryParams) {
	if p.LowStockThreshold <= 0 {
		p.LowStockThreshold = 10
	}
	if p.LowStockDaysCover <= 0 {
		p.LowStockDaysCover = 7
	}
	if p.SlowMovingDays <= 0 {
		p.SlowMovingDays = 90
	}
	if p.AtRiskDays <= 0 {
		p.AtRiskDays = 60
	}
	if p.Period == "" {
		p.Period = "90d"
	}
	if p.Status == "" {
		p.Status = "all"
	}
	if p.Efficiency == "" {
		p.Efficiency = "all"
	}
	if p.Sort == "" {
		p.Sort = "days_cover_asc"
	}
}

// parseInventoryPeriodFromParams parse period từ string params (dùng cho products/variations).
func parseInventoryPeriodFromParams(from, to, period string) (time.Time, time.Time, int, error) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	if period == "custom" && from != "" && to != "" {
		fromTime, err := time.ParseInLocation(reportdto.ReportDateFormat, from, loc)
		if err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("from không đúng định dạng dd-mm-yyyy: %w", err)
		}
		toTime, err := time.ParseInLocation(reportdto.ReportDateFormat, to, loc)
		if err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("to không đúng định dạng dd-mm-yyyy: %w", err)
		}
		if fromTime.After(toTime) {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("from phải nhỏ hơn hoặc bằng to")
		}
		daysInPeriod := int(toTime.Sub(fromTime).Hours()/24) + 1
		if daysInPeriod < 1 {
			daysInPeriod = 1
		}
		return fromTime, toTime, daysInPeriod, nil
	}

	switch period {
	case "day":
		from := today
		to := today.Add(24*time.Hour - time.Second)
		return from, to, 1, nil
	case "week":
		from := today.AddDate(0, 0, -7)
		to := today
		return from, to, 7, nil
	case "60d":
		from := today.AddDate(0, 0, -60)
		to := today
		return from, to, 60, nil
	case "90d":
		from := today.AddDate(0, 0, -90)
		to := today
		return from, to, 90, nil
	case "year":
		from := today.AddDate(0, 0, -365)
		to := today
		return from, to, 365, nil
	default:
		from := today.AddDate(0, 0, -90)
		to := today
		return from, to, 90, nil
	}
}

// variationInventoryData dữ liệu variation đã parse để build items.
type variationInventoryData struct {
	VariationId   string
	ProductId     string
	Sku           string
	VariationName string // Tên mẫu mã (từ posData.name hoặc fields)
	ImageUrl      string // URL ảnh đầu tiên
	TotalRemain   int64
	UnitPrice     float64
	Warehouses    []warehouseInventoryData
	WarehouseIds  []string
}

type warehouseInventoryData struct {
	WarehouseId   string
	RemainQuantity int64
	SellingAvg    float64
}

func (s *ReportService) loadVariationsForInventory(ctx context.Context, ownerOrgID primitive.ObjectID) ([]variationInventoryData, error) {
	return s.loadVariationsForInventoryFiltered(ctx, ownerOrgID, "")
}

// loadVariationsForInventoryFiltered load variations, có thể filter theo productId (rỗng = tất cả).
func (s *ReportService) loadVariationsForInventoryFiltered(ctx context.Context, ownerOrgID primitive.ObjectID, productId string) ([]variationInventoryData, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosVariations)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosVariations, common.ErrNotFound)
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID}
	if productId != "" {
		filter["productId"] = productId
	}
	opts := options.Find().SetProjection(bson.M{
		"variationId": 1, "productId": 1, "sku": 1, "quantity": 1, "retailPrice": 1, "posData": 1,
	})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	var result []variationInventoryData
	for cursor.Next(ctx) {
		var doc struct {
			VariationId string                 `bson:"variationId"`
			ProductId   string                 `bson:"productId"`
			Sku         string                 `bson:"sku"`
			Quantity    int64                  `bson:"quantity"`
			RetailPrice float64                `bson:"retailPrice"`
			PosData     map[string]interface{} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if isHiddenOrRemoved(doc.PosData) {
			continue
		}
		sku := getStringFromMap(doc.PosData, "display_id", "sku")
		if sku == "" {
			sku = doc.Sku
		}
		variationName := getVariationNameFromPosData(doc.PosData)
		imageUrl := getFirstImageUrlFromPosData(doc.PosData)
		unitPrice := getFloatFromMap(doc.PosData, "average_imported_price", "retail_price")
		if unitPrice <= 0 {
			unitPrice = doc.RetailPrice
		}
		totalRemain := doc.Quantity
		if r := getInt64FromMap(doc.PosData, "remain_quantity"); r != nil {
			totalRemain = *r
		}
		var warehouses []warehouseInventoryData
		var whIds []string
		if arr, ok := doc.PosData["variations_warehouses"].([]interface{}); ok && len(arr) > 0 {
			for _, a := range arr {
				m, ok := a.(map[string]interface{})
				if !ok {
					continue
				}
				whID := getStringFromMapDirect(m, "warehouse_id")
				actual := getInt64FromMapDirect(m, "actual_remain_quantity")
				remain := getInt64FromMapDirect(m, "remain_quantity")
				if actual != nil {
					remain = actual
				}
				if remain == nil {
					remainVal := int64(0)
					remain = &remainVal
				}
				sellingAvg := getFloatFromMapDirect(m, "selling_avg")
				warehouses = append(warehouses, warehouseInventoryData{
					WarehouseId:    whID,
					RemainQuantity: *remain,
					SellingAvg:     sellingAvg,
				})
				if whID != "" {
					whIds = append(whIds, whID)
				}
			}
		}
		result = append(result, variationInventoryData{
			VariationId:   doc.VariationId,
			ProductId:    doc.ProductId,
			Sku:          sku,
			VariationName: variationName,
			ImageUrl:     imageUrl,
			TotalRemain:  totalRemain,
			UnitPrice:    unitPrice,
			Warehouses:   warehouses,
			WarehouseIds: whIds,
		})
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return result, nil
}

// productDetails thông tin chi tiết sản phẩm (category, image).
type productDetails struct {
	CategoryId   int64
	CategoryName string
	ImageUrl     string
}

func (s *ReportService) loadProductNames(ctx context.Context, ownerOrgID primitive.ObjectID, productIds map[string]bool, out map[string]string) error {
	if len(productIds) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosProducts)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosProducts, common.ErrNotFound)
	}
	ids := make([]string, 0, len(productIds))
	for id := range productIds {
		ids = append(ids, id)
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID, "productId": bson.M{"$in": ids}}
	opts := options.Find().SetProjection(bson.M{"productId": 1, "name": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct {
			ProductId string `bson:"productId"`
			Name      string `bson:"name"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		out[doc.ProductId] = doc.Name
	}
	return cursor.Err()
}

// loadProductDetails load category và image cho products (dùng cho InventoryProductItem).
func (s *ReportService) loadProductDetails(ctx context.Context, ownerOrgID primitive.ObjectID, productIds map[string]bool, firstVariationImage map[string]string) (map[string]productDetails, error) {
	out := make(map[string]productDetails)
	if len(productIds) == 0 {
		return out, nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosProducts)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosProducts, common.ErrNotFound)
	}
	ids := make([]string, 0, len(productIds))
	for id := range productIds {
		ids = append(ids, id)
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID, "productId": bson.M{"$in": ids}}
	opts := options.Find().SetProjection(bson.M{"productId": 1, "categoryIds": 1, "posData": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	categoryIdsSet := make(map[int64]bool)
	for cursor.Next(ctx) {
		var doc struct {
			ProductId   string                 `bson:"productId"`
			CategoryIds []int64               `bson:"categoryIds"`
			PosData     map[string]interface{} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		det := productDetails{}
		if len(doc.CategoryIds) > 0 {
			det.CategoryId = doc.CategoryIds[0]
			categoryIdsSet[doc.CategoryIds[0]] = true
		}
		det.ImageUrl = getFirstImageUrlFromPosData(doc.PosData)
		if det.ImageUrl == "" && firstVariationImage != nil {
			det.ImageUrl = firstVariationImage[doc.ProductId]
		}
		out[doc.ProductId] = det
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}

	// Load category names
	if len(categoryIdsSet) > 0 {
		catNameMap := make(map[int64]string)
		if catColl, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosCategories); ok {
			catIds := make([]int64, 0, len(categoryIdsSet))
			for id := range categoryIdsSet {
				catIds = append(catIds, id)
			}
			catFilter := bson.M{"ownerOrganizationId": ownerOrgID, "categoryId": bson.M{"$in": catIds}}
			catOpts := options.Find().SetProjection(bson.M{"categoryId": 1, "name": 1})
			catCursor, err := catColl.Find(ctx, catFilter, catOpts)
			if err == nil {
				defer catCursor.Close(ctx)
				for catCursor.Next(ctx) {
					var c struct {
						CategoryId int64  `bson:"categoryId"`
						Name       string `bson:"name"`
					}
					if catCursor.Decode(&c) == nil {
						catNameMap[c.CategoryId] = c.Name
					}
				}
			}
		}
		for pid, det := range out {
			det.CategoryName = catNameMap[det.CategoryId]
			out[pid] = det
		}
	}

	return out, nil
}

func (s *ReportService) loadWarehouseNames(ctx context.Context, ownerOrgID primitive.ObjectID, warehouseIds map[string]bool, out map[string]string) error {
	if len(warehouseIds) == 0 {
		return nil
	}
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosWarehouses)
	if !ok {
		return fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosWarehouses, common.ErrNotFound)
	}
	ids := make([]string, 0, len(warehouseIds))
	for id := range warehouseIds {
		ids = append(ids, id)
	}
	filter := bson.M{"ownerOrganizationId": ownerOrgID, "warehouseId": bson.M{"$in": ids}}
	opts := options.Find().SetProjection(bson.M{"warehouseId": 1, "name": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc struct {
			WarehouseId string `bson:"warehouseId"`
			Name        string `bson:"name"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		out[doc.WarehouseId] = doc.Name
	}
	return cursor.Err()
}

func (s *ReportService) aggregateDailySales(ctx context.Context, ownerOrgID primitive.ObjectID, fromTime, toTime time.Time, daysInPeriod int) (map[string]float64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosOrders, common.ErrNotFound)
	}
	// Thời gian: insertedAt/posCreatedAt có thể lưu Unix giây hoặc milliseconds (Pancake POS)
	// Sample: insertedAt = 1767866197971 (ms) — phải hỗ trợ cả hai
	fromSec := fromTime.Unix()
	toSec := toTime.Unix()
	fromMs := fromSec * 1000
	toMs := toSec * 1000
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		"$and": []bson.M{
			{
				"$or": []bson.M{
					{"insertedAt": bson.M{"$gte": fromSec, "$lte": toSec}},
					{"posCreatedAt": bson.M{"$gte": fromSec, "$lte": toSec}},
					{"insertedAt": bson.M{"$gte": fromMs, "$lte": toMs}},
					{"posCreatedAt": bson.M{"$gte": fromMs, "$lte": toMs}},
				},
			},
			// Trừ đơn hủy (6), xóa (7)
			{"posData.status": bson.M{"$nin": orderStatusCancelled}},
			{"status": bson.M{"$nin": orderStatusCancelled}},
		},
	}
	opts := options.Find().SetProjection(bson.M{"orderItems": 1, "posData": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	totalByVariation := make(map[string]int64)
	for cursor.Next(ctx) {
		var doc struct {
			OrderItems []interface{}          `bson:"orderItems"`
			PosData    map[string]interface{} `bson:"posData"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		items := extractOrderItemsFromDoc(doc.OrderItems, doc.PosData)
		for _, it := range items {
			vid := getVariationIdFromItem(it)
			if vid == "" {
				continue
			}
			qty := getInt64FromMapDirect(it, "quantity")
			if qty == nil {
				continue
			}
			totalByVariation[vid] += *qty
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}

	result := make(map[string]float64)
	for vid, total := range totalByVariation {
		result[vid] = float64(total) / float64(daysInPeriod)
	}
	return result, nil
}

// getLastSaleDateByVariation trả về map variationId -> Unix timestamp (giây) của lần bán cuối cùng.
// Dùng để tính daysSinceLastSale. Trả về map rỗng nếu lỗi.
func (s *ReportService) getLastSaleDateByVariation(ctx context.Context, ownerOrgID primitive.ObjectID) (map[string]int64, error) {
	coll, ok := global.RegistryCollections.Get(global.MongoDB_ColNames.PcPosOrders)
	if !ok {
		return nil, fmt.Errorf("không tìm thấy collection %s: %w", global.MongoDB_ColNames.PcPosOrders, common.ErrNotFound)
	}
	filter := bson.M{
		"ownerOrganizationId": ownerOrgID,
		// Trừ đơn hủy (6), xóa (7) — $nin: field không tồn tại cũng match
		"$and": []bson.M{
			{"posData.status": bson.M{"$nin": orderStatusCancelled}},
			{"status": bson.M{"$nin": orderStatusCancelled}},
		},
	}
	opts := options.Find().SetProjection(bson.M{"orderItems": 1, "posData": 1, "insertedAt": 1, "posCreatedAt": 1})
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, common.ConvertMongoError(err)
	}
	defer cursor.Close(ctx)

	lastSaleByVariation := make(map[string]int64)
	for cursor.Next(ctx) {
		var doc struct {
			OrderItems   []interface{}          `bson:"orderItems"`
			PosData      map[string]interface{} `bson:"posData"`
			InsertedAt   interface{}            `bson:"insertedAt"`
			PosCreatedAt interface{}            `bson:"posCreatedAt"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		orderTs := getOrderTimestamp(doc.InsertedAt, doc.PosCreatedAt, doc.PosData)
		if orderTs <= 0 {
			continue
		}
		items := extractOrderItemsFromDoc(doc.OrderItems, doc.PosData)
		for _, it := range items {
			vid := getVariationIdFromItem(it)
			if vid == "" {
				continue
			}
			if orderTs > lastSaleByVariation[vid] {
				lastSaleByVariation[vid] = orderTs
			}
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, common.ConvertMongoError(err)
	}
	return lastSaleByVariation, nil
}

// getOrderTimestamp lấy Unix timestamp (giây) từ insertedAt, posCreatedAt hoặc posData.
// Hỗ trợ cả giây, milliseconds và chuỗi ISO (vd: "2025-12-18T12:23:33.022266").
func getOrderTimestamp(insertedAt, posCreatedAt interface{}, posData map[string]interface{}) int64 {
	var ts int64
	if v, ok := toInt64(insertedAt); ok && v > 0 {
		ts = v
	}
	if v, ok := toInt64(posCreatedAt); ok && v > 0 && v > ts {
		ts = v
	}
	if posData != nil {
		for _, key := range []string{"inserted_at", "pos_created_at", "created_at"} {
			if v, ok := posData[key]; ok && v != nil {
				if parsed := parseTimestampFromValue(v); parsed > ts {
					ts = parsed
				}
			}
		}
	}
	if ts <= 0 {
		return 0
	}
	// Chuẩn hóa về giây (nếu là milliseconds, giá trị > 1e12)
	if ts > 1e12 {
		ts = ts / 1000
	}
	return ts
}

// parseTimestampFromValue chuyển giá trị (int64, float64, string ISO) thành Unix giây.
func parseTimestampFromValue(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int32:
		return int64(x)
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		// Thử parse ISO format: 2006-01-02T15:04:05, 2006-01-02T15:04:05.999999
		for _, layout := range []string{
			"2006-01-02T15:04:05.999999",
			"2006-01-02T15:04:05.999",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			time.RFC3339,
		} {
			if t, err := time.Parse(layout, x); err == nil {
				return t.Unix()
			}
		}
		return 0
	default:
		return 0
	}
}

// computeDaysSinceLastSale tính số ngày từ lastSaleUnixSec đến hiện tại.
// lastSaleUnixSec = 0 hoặc -1: chưa từng bán → trả -1.
func computeDaysSinceLastSale(lastSaleUnixSec int64) int64 {
	if lastSaleUnixSec <= 0 {
		return -1
	}
	now := time.Now().Unix()
	diff := now - lastSaleUnixSec
	if diff < 0 {
		return 0
	}
	return diff / 86400
}

func buildInventoryItem(v variationInventoryData, wh warehouseInventoryData, productName, whName string, dailySales float64, unitPrice float64, lowThreshold, lowDays, slowMovingDays, atRiskDays int, daysSinceLastSale int64) reportdto.InventoryItem {
	remain := wh.RemainQuantity
	isSellNegative := remain == -1
	var daysCover float64 = -1
	if !isSellNegative && dailySales > 0 {
		daysCover = float64(remain) / dailySales
	}
	status := computeInventoryStatus(remain, isSellNegative, daysCover, lowThreshold, lowDays)
	effStatus := computeEfficiencyStatus(remain, isSellNegative, dailySales, daysCover, daysSinceLastSale, slowMovingDays, atRiskDays)
	invValue := float64(0)
	if !isSellNegative && remain > 0 {
		invValue = float64(remain) * unitPrice
	}
	return reportdto.InventoryItem{
		VariationId:       v.VariationId,
		ProductId:        v.ProductId,
		Sku:              v.Sku,
		VariationName:    v.VariationName,
		ProductName:      productName,
		WarehouseId:      wh.WarehouseId,
		WarehouseName:    whName,
		RemainQuantity:   remain,
		DailySalesRate:   dailySales,
		DaysCover:        daysCover,
		DaysSinceLastSale: daysSinceLastSale,
		Status:           status,
		EfficiencyStatus: effStatus,
		UnitPrice:        unitPrice,
		InventoryValue:   invValue,
		IsSellNegative:   isSellNegative,
		ImageUrl:         v.ImageUrl,
	}
}

func buildInventoryItemTotal(v variationInventoryData, productName string, dailySales float64, unitPrice float64, lowThreshold, lowDays, slowMovingDays, atRiskDays int, daysSinceLastSale int64) reportdto.InventoryItem {
	remain := v.TotalRemain
	isSellNegative := remain == -1
	var daysCover float64 = -1
	if !isSellNegative && dailySales > 0 {
		daysCover = float64(remain) / dailySales
	}
	status := computeInventoryStatus(remain, isSellNegative, daysCover, lowThreshold, lowDays)
	effStatus := computeEfficiencyStatus(remain, isSellNegative, dailySales, daysCover, daysSinceLastSale, slowMovingDays, atRiskDays)
	invValue := float64(0)
	if !isSellNegative && remain > 0 {
		invValue = float64(remain) * unitPrice
	}
	return reportdto.InventoryItem{
		VariationId:       v.VariationId,
		ProductId:        v.ProductId,
		Sku:              v.Sku,
		VariationName:    v.VariationName,
		ProductName:      productName,
		WarehouseId:      "",
		WarehouseName:    "Tổng",
		RemainQuantity:   remain,
		DailySalesRate:   dailySales,
		DaysCover:        daysCover,
		DaysSinceLastSale: daysSinceLastSale,
		Status:           status,
		EfficiencyStatus: effStatus,
		UnitPrice:        unitPrice,
		InventoryValue:   invValue,
		IsSellNegative:   isSellNegative,
		ImageUrl:         v.ImageUrl,
	}
}

// computeEfficiencyStatus tính trạng thái hiệu quả tồn kho dựa trên daysCover, dailySales và daysSinceLastSale.
// dead_stock: remain > 0 VÀ (daysSinceLastSale >= slowMovingDays HOẶC daysSinceLastSale == -1 chưa từng bán).
// slow_moving: daysCover > slowMovingDays (tồn lâu).
// at_risk: atRiskDays < daysCover <= slowMovingDays (cần theo dõi).
// ok: còn lại.
func computeEfficiencyStatus(remain int64, isSellNegative bool, dailySales, daysCover float64, daysSinceLastSale int64, slowMovingDays, atRiskDays int) string {
	if isSellNegative || remain <= 0 {
		return "ok"
	}
	// Hàng chết = số ngày chưa bán được >= ngưỡng, hoặc chưa từng bán (-1)
	slow := int64(slowMovingDays)
	if slow <= 0 {
		slow = 90
	}
	if daysSinceLastSale == -1 || daysSinceLastSale >= slow {
		return "dead_stock"
	}
	if daysCover < 0 {
		return "ok"
	}
	slowF := float64(slowMovingDays)
	if slowF <= 0 {
		slowF = 90
	}
	atRisk := float64(atRiskDays)
	if atRisk <= 0 {
		atRisk = 60
	}
	if daysCover > slowF {
		return "slow_moving"
	}
	if daysCover > atRisk {
		return "at_risk"
	}
	return "ok"
}

func computeInventoryStatus(remain int64, isSellNegative bool, daysCover float64, lowThreshold, lowDays int) string {
	if isSellNegative {
		return "sell_negative"
	}
	if remain == 0 {
		return "out_of_stock"
	}
	if remain <= int64(lowThreshold) {
		return "low_stock"
	}
	if daysCover >= 0 && daysCover < float64(lowDays) {
		return "low_stock"
	}
	return "ok"
}

func getDaysCoverBucket(daysCover float64, isSellNegative bool) string {
	if isSellNegative || daysCover < 0 {
		return "infinity"
	}
	d := int64(daysCover)
	if d == 0 {
		return "0"
	}
	if d < 8 {
		return "1-7"
	}
	if d < 15 {
		return "8-14"
	}
	if d < 31 {
		return "15-30"
	}
	if d < 61 {
		return "31-60"
	}
	if d < 91 {
		return "61-90"
	}
	return "90+"
}

func filterInventoryItems(items []reportdto.InventoryItem, statusFilter, efficiencyFilter, warehouseID string) []reportdto.InventoryItem {
	var out []reportdto.InventoryItem
	for _, it := range items {
		if warehouseID != "" && it.WarehouseId != warehouseID {
			continue
		}
		if statusFilter != "" && statusFilter != "all" && it.Status != statusFilter {
			continue
		}
		if efficiencyFilter != "" && efficiencyFilter != "all" && it.EfficiencyStatus != efficiencyFilter {
			continue
		}
		out = append(out, it)
	}
	return out
}

func sortInventoryItems(items []reportdto.InventoryItem, sortBy string) {
	switch sortBy {
	case "days_cover_desc":
		sort.Slice(items, func(i, j int) bool {
			if items[i].DaysCover < 0 && items[j].DaysCover < 0 {
				return items[i].Sku < items[j].Sku
			}
			if items[i].DaysCover < 0 {
				return false
			}
			if items[j].DaysCover < 0 {
				return true
			}
			return items[i].DaysCover > items[j].DaysCover
		})
	case "remain_asc":
		sort.Slice(items, func(i, j int) bool { return items[i].RemainQuantity < items[j].RemainQuantity })
	case "remain_desc":
		sort.Slice(items, func(i, j int) bool { return items[i].RemainQuantity > items[j].RemainQuantity })
	case "sku":
		sort.Slice(items, func(i, j int) bool { return items[i].Sku < items[j].Sku })
	case "product_name":
		sort.Slice(items, func(i, j int) bool { return items[i].ProductName < items[j].ProductName })
	default:
		sort.Slice(items, func(i, j int) bool {
			if items[i].DaysCover < 0 && items[j].DaysCover < 0 {
				return items[i].Sku < items[j].Sku
			}
			if items[i].DaysCover < 0 {
				return true
			}
			if items[j].DaysCover < 0 {
				return false
			}
			return items[i].DaysCover < items[j].DaysCover
		})
	}
}

func computeSummaryStatuses(totalValue, skuCount, lowStock, outOfStock, deadStock, slowMoving int64) map[string]string {
	st := make(map[string]string)
	st["totalInventoryValue"] = "green"
	if skuCount == 0 {
		st["skuCount"] = "red"
	} else {
		st["skuCount"] = "green"
	}
	if lowStock == 0 {
		st["lowStockCount"] = "green"
	} else if lowStock <= 20 {
		st["lowStockCount"] = "yellow"
	} else {
		st["lowStockCount"] = "red"
	}
	if outOfStock == 0 {
		st["outOfStockCount"] = "green"
	} else if outOfStock <= 5 {
		st["outOfStockCount"] = "yellow"
	} else {
		st["outOfStockCount"] = "red"
	}
	if deadStock == 0 {
		st["deadStockCount"] = "green"
	} else if deadStock <= 10 {
		st["deadStockCount"] = "yellow"
	} else {
		st["deadStockCount"] = "red"
	}
	if slowMoving == 0 {
		st["slowMovingCount"] = "green"
	} else if slowMoving <= 20 {
		st["slowMovingCount"] = "yellow"
	} else {
		st["slowMovingCount"] = "red"
	}
	return st
}

func applyInventoryDefaults(p *reportdto.InventoryQueryParams) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = 50 // Mặc định 50 dòng/trang — chuẩn PaginateResult
	}
	if p.Limit > 2000 {
		p.Limit = 2000
	}
	if p.LowStockThreshold <= 0 {
		p.LowStockThreshold = 10
	}
	if p.LowStockDaysCover <= 0 {
		p.LowStockDaysCover = 7
	}
	if p.SlowMovingDays <= 0 {
		p.SlowMovingDays = 90
	}
	if p.AtRiskDays <= 0 {
		p.AtRiskDays = 60
	}
	if p.Period == "" {
		p.Period = "90d"
	}
	if p.Status == "" {
		p.Status = "all"
	}
	if p.Efficiency == "" {
		p.Efficiency = "all"
	}
	if p.Sort == "" {
		p.Sort = "days_cover_asc"
	}
}

func parseInventoryPeriod(p *reportdto.InventoryQueryParams) (from, to time.Time, daysInPeriod int, err error) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	if p.Period == "custom" && p.From != "" && p.To != "" {
		from, err = time.ParseInLocation(reportdto.ReportDateFormat, p.From, loc)
		if err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("from không đúng định dạng dd-mm-yyyy: %w", err)
		}
		to, err = time.ParseInLocation(reportdto.ReportDateFormat, p.To, loc)
		if err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("to không đúng định dạng dd-mm-yyyy: %w", err)
		}
		if from.After(to) {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("from phải nhỏ hơn hoặc bằng to")
		}
		daysInPeriod = int(to.Sub(from).Hours()/24) + 1
		if daysInPeriod < 1 {
			daysInPeriod = 1
		}
		return from, to, daysInPeriod, nil
	}

	switch p.Period {
	case "day":
		from = today
		to = today.Add(24*time.Hour - time.Second)
		daysInPeriod = 1
	case "week":
		from = today.AddDate(0, 0, -7)
		to = today
		daysInPeriod = 7
	case "60d":
		from = today.AddDate(0, 0, -60)
		to = today
		daysInPeriod = 60
	case "90d":
		from = today.AddDate(0, 0, -90)
		to = today
		daysInPeriod = 90
	case "year":
		from = today.AddDate(0, 0, -365)
		to = today
		daysInPeriod = 365
	default:
		from = today.AddDate(0, 0, -90)
		to = today
		daysInPeriod = 90
	}
	return from, to, daysInPeriod, nil
}

func isHiddenOrRemoved(posData map[string]interface{}) bool {
	if posData == nil {
		return false
	}
	if v, ok := posData["is_hidden"].(bool); ok && v {
		return true
	}
	if posData["is_removed"] != nil {
		return true
	}
	return false
}

// getVariationNameFromPosData lấy tên mẫu mã từ posData (name hoặc format từ fields).
func getVariationNameFromPosData(posData map[string]interface{}) string {
	if posData == nil {
		return ""
	}
	if name := getStringFromMap(posData, "name", "variation_name"); name != "" {
		return name
	}
	// Build từ fields: [{attribute_name, attribute_value}, ...]
	if arr, ok := posData["fields"].([]interface{}); ok && len(arr) > 0 {
		var parts []string
		for _, a := range arr {
			if m, ok := a.(map[string]interface{}); ok {
				attrName := getStringFromMapDirect(m, "attribute_name", "name")
				attrVal := getStringFromMapDirect(m, "attribute_value", "value")
				if attrName != "" || attrVal != "" {
					parts = append(parts, attrName+": "+attrVal)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, ", ")
		}
	}
	return ""
}

// getFirstImageUrlFromPosData lấy URL ảnh đầu tiên từ posData.images.
func getFirstImageUrlFromPosData(posData map[string]interface{}) string {
	if posData == nil {
		return ""
	}
	arr, ok := posData["images"].([]interface{})
	if !ok || len(arr) == 0 {
		return ""
	}
	if s, ok := arr[0].(string); ok && s != "" {
		return s
	}
	if m, ok := arr[0].(map[string]interface{}); ok {
		if url := getStringFromMapDirect(m, "url", "src", "image"); url != "" {
			return url
		}
	}
	return ""
}

func getStringFromMap(m map[string]interface{}, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			if s, ok := v.(string); ok {
				return s
			}
			if s, ok := v.(fmt.Stringer); ok {
				return s.String()
			}
		}
	}
	return ""
}

// getVariationIdFromItem lấy variation ID từ order item. Hỗ trợ variation_id, variationId, variation_info.id, variation_info.variation_id.
func getVariationIdFromItem(it map[string]interface{}) string {
	vid := getStringFromMapDirect(it, "variation_id", "variationId")
	if vid != "" {
		return vid
	}
	if infoRaw, ok := it["variation_info"]; ok && infoRaw != nil {
		info := toMap(infoRaw)
		if info != nil {
			vid = getStringFromMapDirect(info, "variation_id", "id", "variationId")
			if vid != "" {
				return vid
			}
		}
	}
	return ""
}

func getStringFromMapDirect(m map[string]interface{}, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch x := v.(type) {
			case string:
				return x
			default:
				return fmt.Sprintf("%v", v)
			}
		}
	}
	return ""
}

func getFloatFromMap(m map[string]interface{}, keys ...string) float64 {
	if m == nil {
		return 0
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch x := v.(type) {
			case float64:
				return x
			case int:
				return float64(x)
			case int64:
				return float64(x)
			}
		}
	}
	return 0
}

func getFloatFromMapDirect(m map[string]interface{}, keys ...string) float64 {
	if m == nil {
		return 0
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch x := v.(type) {
			case float64:
				return x
			case int:
				return float64(x)
			case int64:
				return float64(x)
			}
		}
	}
	return 0
}

// extractOrderItemsFromDoc lấy danh sách item từ orderItems hoặc posData.items.
// Hỗ trợ cả map và bson.D (MongoDB có thể decode nested doc thành primitive.D).
func extractOrderItemsFromDoc(orderItems []interface{}, posData map[string]interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	appendItem := func(v interface{}) {
		if m := toMap(v); m != nil {
			out = append(out, m)
		}
	}
	for _, v := range orderItems {
		appendItem(v)
	}
	if len(out) > 0 {
		return out
	}
	if posData != nil {
		if arr, ok := posData["items"].([]interface{}); ok {
			for _, v := range arr {
				appendItem(v)
			}
		}
		if arr, ok := posData["order_items"].([]interface{}); ok && len(out) == 0 {
			for _, v := range arr {
				appendItem(v)
			}
		}
	}
	return out
}

// toMap chuyển interface{} thành map[string]interface{}. Hỗ trợ bson.D/primitive.D (MongoDB decode nested doc).
func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	// BSON decode có thể trả primitive.D ([]primitive.E) cho nested document
	if d, ok := v.(primitive.D); ok {
		m := make(map[string]interface{}, len(d))
		for _, e := range d {
			m[e.Key] = e.Value
		}
		return m
	}
	return nil
}

func getInt64FromMap(m map[string]interface{}, keys ...string) *int64 {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			var n int64
			switch x := v.(type) {
			case int64:
				n = x
			case int32:
				n = int64(x)
			case int:
				n = int64(x)
			case float64:
				n = int64(x)
			default:
				continue
			}
			return &n
		}
	}
	return nil
}

func getInt64FromMapDirect(m map[string]interface{}, keys ...string) *int64 {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			var n int64
			switch x := v.(type) {
			case int64:
				n = x
			case int32:
				n = int64(x)
			case int:
				n = int64(x)
			case float64:
				n = int64(x)
			default:
				continue
			}
			return &n
		}
	}
	return nil
}
