// Package reportdto - DTO cho Inventory Intelligence (Tab 5 Dashboard).
package reportdto

// InventoryQueryParams query params cho GET /dashboard/inventory.
// Dùng để tính snapshot tồn kho: KPI, bảng chi tiết, days cover distribution, alerts.
// Phân trang theo chuẩn PaginateResult: page (mặc định 1), limit (mặc định 50).
type InventoryQueryParams struct {
	From               string `query:"from"`               // dd-mm-yyyy
	To                 string `query:"to"`               // dd-mm-yyyy
	Period             string `query:"period"`            // day|week|month|60d|90d|year|custom
	Page               int    `query:"page"`             // Trang hiện tại (mặc định 1) — chuẩn phân trang
	Limit              int    `query:"limit"`            // Số dòng items mỗi trang (mặc định 50)
	Status             string `query:"status"`            // out_of_stock|low_stock|ok|sell_negative|all
	Efficiency         string `query:"efficiency"`        // dead_stock|slow_moving|at_risk|ok|all — lọc theo hiệu quả tồn kho
	WarehouseID        string `query:"warehouseId"`       // Lọc theo warehouse (UUID)
	Sort               string `query:"sort"`             // days_cover_asc|days_cover_desc|remain_asc|remain_desc|sku|product_name
	LowStockThreshold  int    `query:"lowStockThreshold"`  // Ngưỡng số lượng sắp hết
	LowStockDaysCover  int    `query:"lowStockDaysCover"` // Ngưỡng số ngày còn (days cover)
	SlowMovingDays     int    `query:"slowMovingDays"`    // Ngưỡng tồn lâu (days cover > X) — mặc định 90
	AtRiskDays         int    `query:"atRiskDays"`        // Ngưỡng cần theo dõi (60 < days cover ≤ 90) — mặc định 60
}

// InventorySummary 6 KPI cho Tab 5 (4 cũ + 2 hiệu quả tồn kho).
type InventorySummary struct {
	TotalInventoryValue int64 `json:"totalInventoryValue"` // Tổng giá trị tồn kho
	SkuCount            int64 `json:"skuCount"`            // Số SKU
	LowStockCount       int64 `json:"lowStockCount"`       // Sắp hết hàng
	OutOfStockCount     int64 `json:"outOfStockCount"`     // Hết hàng
	DeadStockCount      int64 `json:"deadStockCount"`      // Số SKU hàng chết
	DeadStockValue      int64 `json:"deadStockValue"`      // Tổng giá trị hàng chết (để hiển thị nếu có)
	SlowMovingCount     int64 `json:"slowMovingCount"`     // Số SKU hàng tồn lâu
	SlowMovingValue     int64 `json:"slowMovingValue"`     // Tổng giá trị hàng tồn lâu (để hiển thị nếu có)
}

// InventoryItem 1 dòng trong bảng tồn kho (variation-warehouse).
type InventoryItem struct {
	VariationId     string  `json:"variationId"`
	ProductId       string  `json:"productId"`
	Sku             string  `json:"sku"`
	VariationName   string  `json:"variationName,omitempty"`   // Tên mẫu mã (vd: Size M, Màu đỏ)
	ProductName     string  `json:"productName"`
	WarehouseId     string  `json:"warehouseId"`
	WarehouseName   string  `json:"warehouseName"`
	RemainQuantity  int64   `json:"remainQuantity"`
	DailySalesRate  float64 `json:"dailySalesRate"`
	DaysCover         float64 `json:"daysCover"`      // -1 = infinity (không ước tính)
	DaysSinceLastSale int64   `json:"daysSinceLastSale"` // Số ngày chưa bán được (kể từ lần bán cuối); -1 = chưa từng bán
	Status            string  `json:"status"`         // ok|low_stock|out_of_stock|sell_negative
	EfficiencyStatus string  `json:"efficiencyStatus"` // dead_stock|slow_moving|at_risk|ok — hiệu quả tồn kho
	UnitPrice        float64 `json:"unitPrice"`     // Đơn giá (giá vốn/retail) — dùng cho Tạo phiếu nhập
	InventoryValue   float64 `json:"inventoryValue"`
	IsSellNegative   bool    `json:"isSellNegative"`
	ImageUrl         string  `json:"imageUrl,omitempty"` // URL ảnh mẫu mã (thumbnail)
}

// InventoryAlertItem mục trong alert zone CRITICAL/WARNING.
type InventoryAlertItem struct {
	Sku           string  `json:"sku"`
	ProductName   string  `json:"productName"`
	WarehouseName string  `json:"warehouseName,omitempty"`
	DaysCover     float64 `json:"daysCover,omitempty"`
}

// InventoryAlerts danh sách critical và warning.
type InventoryAlerts struct {
	Critical []InventoryAlertItem `json:"critical"`
	Warning  []InventoryAlertItem `json:"warning"`
}

// InventoryProductItem 1 sản phẩm trong danh sách tree (lazy load) — level 1.
// Dùng cho GET /dashboard/inventory/products.
type InventoryProductItem struct {
	ProductId       string  `json:"productId"`
	ProductName     string  `json:"productName"`
	VariationCount  int     `json:"variationCount"`  // Số mẫu mã
	TotalRemain     int64   `json:"totalRemain"`    // Tổng tồn (tất cả variation-warehouse)
	LowStockCount   int64   `json:"lowStockCount"`  // Số dòng variation-warehouse sắp hết
	OutOfStockCount int64   `json:"outOfStockCount"` // Số dòng variation-warehouse hết hàng
	DeadStockCount  int64   `json:"deadStockCount"`  // Số dòng hàng chết (không bán)
	SlowMovingCount int64   `json:"slowMovingCount"`  // Số dòng hàng tồn lâu
	InventoryValue  float64 `json:"inventoryValue"`  // Tổng giá trị tồn kho
	ProductStatus   string  `json:"productStatus"`   // ok|warning|critical — để frontend highlight
	ImageUrl        string  `json:"imageUrl,omitempty"`        // URL ảnh đại diện (từ variation đầu tiên hoặc product)
	CategoryId      int64   `json:"categoryId,omitempty"`     // ID danh mục (lấy category đầu tiên)
	CategoryName    string  `json:"categoryName,omitempty"`  // Tên danh mục
}

// InventoryProductsQueryParams query params cho GET /dashboard/inventory/products.
type InventoryProductsQueryParams struct {
	From              string `query:"from"`
	To                string `query:"to"`
	Period            string `query:"period"`
	Page              int    `query:"page"`
	Limit             int    `query:"limit"`
	Status            string `query:"status"`     // Lọc sản phẩm có ít nhất 1 variation trong status
	Efficiency        string `query:"efficiency"` // dead_stock|slow_moving|at_risk|ok|all
	WarehouseID       string `query:"warehouseId"`
	Sort              string `query:"sort"`       // product_name|variation_count|total_remain|inventory_value
	LowStockThreshold int    `query:"lowStockThreshold"`
	LowStockDaysCover int    `query:"lowStockDaysCover"`
	SlowMovingDays    int    `query:"slowMovingDays"`
	AtRiskDays        int    `query:"atRiskDays"`
}

// InventoryProductsResult kết quả GET /dashboard/inventory/products — format PaginateResult.
type InventoryProductsResult struct {
	Items     []InventoryProductItem `json:"items"`
	Page      int64                  `json:"page"`
	Limit     int64                  `json:"limit"`
	ItemCount int64                  `json:"itemCount"`
	Total     int64                  `json:"total"`
	TotalPage int64                  `json:"totalPage"`
}

// InventoryVariationsQueryParams query params cho GET /dashboard/inventory/products/:productId/variations.
type InventoryVariationsQueryParams struct {
	From              string `query:"from"`
	To                string `query:"to"`
	Period            string `query:"period"`
	Status            string `query:"status"`
	Efficiency        string `query:"efficiency"`
	WarehouseID       string `query:"warehouseId"`
	Sort              string `query:"sort"`
	LowStockThreshold int    `query:"lowStockThreshold"`
	LowStockDaysCover int    `query:"lowStockDaysCover"`
	SlowMovingDays    int    `query:"slowMovingDays"`
	AtRiskDays        int    `query:"atRiskDays"`
}

// InventoryVariationsResult kết quả GET /dashboard/inventory/products/:productId/variations.
type InventoryVariationsResult struct {
	ProductId   string          `json:"productId"`
	ProductName string          `json:"productName"`
	Items       []InventoryItem `json:"items"` // Các dòng variation-warehouse
}

// InventorySnapshotResult kết quả trả về cho GET /dashboard/inventory.
// Phần items tuân format chuẩn PaginateResult: page, limit, itemCount, items, total, totalPage.
type InventorySnapshotResult struct {
	Summary               InventorySummary         `json:"summary"`
	SummaryStatuses       map[string]string        `json:"summaryStatuses,omitempty"` // green|yellow|red từng KPI
	Items                 []InventoryItem          `json:"items"`
	Page                  int64                    `json:"page"`      // Trang hiện tại — chuẩn PaginateResult
	Limit                 int64                    `json:"limit"`     // Số dòng mỗi trang
	ItemCount             int64                    `json:"itemCount"` // Số dòng trong trang hiện tại
	Total                 int64                    `json:"total"`     // Tổng số dòng (sau filter)
	TotalPage             int64                    `json:"totalPage"` // Tổng số trang
	DaysCoverDistribution map[string]int64         `json:"daysCoverDistribution"`
	Alerts                InventoryAlerts          `json:"alerts"`
}
