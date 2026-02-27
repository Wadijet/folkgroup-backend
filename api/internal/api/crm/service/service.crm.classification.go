// Package crmvc - Logic phân loại khách 2 lớp theo CUSTOMER_CLASSIFICATION_SYSTEM_DESIGN.
//
// Kiến trúc 2 lớp:
//
//	LỚP 1 — CUSTOMER JOURNEY (hành trình trưởng thành, thuần):
//	  VISITOR | ENGAGED | FIRST | REPEAT | VIP | INACTIVE
//
//	LỚP 2 — CUSTOMER SEGMENTATION (5 trục phân loại):
//	  CHANNEL | VALUE | LIFECYCLE | LOYALTY | MOMENTUM
//
// Profile đầy đủ: Journey | Channel | Value | Lifecycle | Loyalty | Momentum
package crmvc

import (
	"time"

	crmmodels "meta_commerce/internal/api/crm/models"
)

const (
	// Ngưỡng Value (VNĐ)
	valueVip    = 50_000_000
	valueHigh   = 20_000_000
	valueMedium = 5_000_000
	valueLow    = 1_000_000

	// Ngưỡng Lifecycle (ngày)
	lifecycleActive   = 30
	lifecycleCooling  = 90
	lifecycleInactive = 180

	// Ngưỡng Loyalty (order_count)
	loyaltyCore   = 5
	loyaltyRepeat = 2

	// Ngưỡng Momentum (tỷ lệ revenue_last_30d / revenue_last_90d)
	momentumRising   = 0.5
	momentumStableLo = 0.2
	momentumStableHi = 0.5
)

// msPerDay milliseconds trong 1 ngày.
const msPerDay = 24 * 60 * 60 * 1000

// daysSinceLastOrder tính số ngày từ lastOrderAt (Unix ms) đến hiện tại.
func daysSinceLastOrder(lastOrderAt int64) int64 {
	if lastOrderAt <= 0 {
		return -1
	}
	return (time.Now().UnixMilli() - lastOrderAt) / msPerDay
}

// ComputeValueTier trả về tier theo total_spent (VNĐ).
// vip | high | medium | low | new
func ComputeValueTier(totalSpent float64) string {
	if totalSpent >= valueVip {
		return "vip"
	}
	if totalSpent >= valueHigh {
		return "high"
	}
	if totalSpent >= valueMedium {
		return "medium"
	}
	if totalSpent >= valueLow {
		return "low"
	}
	return "new"
}

// ComputeLifecycleStage trả về stage theo days_since_last_order.
// active | cooling | inactive | dead | never_purchased
func ComputeLifecycleStage(lastOrderAt int64) string {
	daysSince := daysSinceLastOrder(lastOrderAt)
	if daysSince < 0 {
		return "never_purchased"
	}
	if daysSince <= lifecycleActive {
		return "active"
	}
	if daysSince <= lifecycleCooling {
		return "cooling"
	}
	if daysSince <= lifecycleInactive {
		return "inactive"
	}
	return "dead"
}

// ComputeJourneyStage trả về stage Lớp 1 theo logic ưu tiên (design 4.1.2).
// visitor | engaged | first | repeat | vip | inactive
// Đọc metrics từ currentMetrics hoặc top-level (backward compat).
func ComputeJourneyStage(c *crmmodels.CrmCustomer) string {
	orderCount := getOrderCount(c)
	if orderCount == 0 {
		if getHasConversation(c) {
			return "engaged"
		}
		return "visitor"
	}
	lastOrderAt := getLastOrderAt(c)
	daysSince := daysSinceLastOrder(lastOrderAt)
	if daysSince > lifecycleCooling {
		return "inactive"
	}
	if getTotalSpent(c) >= valueVip {
		return "vip"
	}
	if orderCount >= 2 {
		return "repeat"
	}
	return "first"
}

func getOrderCount(c *crmmodels.CrmCustomer) int {
	if c == nil {
		return 0
	}
	if c.CurrentMetrics != nil {
		return GetIntFromNestedMetrics(c.CurrentMetrics, "orderCount")
	}
	return c.OrderCount
}

func getTotalSpent(c *crmmodels.CrmCustomer) float64 {
	if c == nil {
		return 0
	}
	if c.CurrentMetrics != nil {
		return GetFloatFromNestedMetrics(c.CurrentMetrics, "totalSpent")
	}
	return c.TotalSpent
}

func getLastOrderAt(c *crmmodels.CrmCustomer) int64 {
	if c == nil {
		return 0
	}
	if c.CurrentMetrics != nil {
		return GetInt64FromNestedMetrics(c.CurrentMetrics, "lastOrderAt")
	}
	return c.LastOrderAt
}

func getHasConversation(c *crmmodels.CrmCustomer) bool {
	if c == nil {
		return false
	}
	if c.CurrentMetrics != nil {
		if v := getFromNestedMetrics(c.CurrentMetrics, "hasConversation"); v != nil {
			if b, ok := v.(bool); ok {
				return b
			}
		}
		return false
	}
	return c.HasConversation
}

// ComputeChannel trả về kênh mua hàng — trục Lớp 2 (Customer Segmentation).
// online | offline | omnichannel — rỗng nếu order_count = 0 (chưa mua).
func ComputeChannel(c *crmmodels.CrmCustomer) string {
	if getOrderCount(c) == 0 {
		return ""
	}
	ocOnline := getIntFromCustomer(c, "orderCountOnline")
	ocOffline := getIntFromCustomer(c, "orderCountOffline")
	if ocOnline > 0 && ocOffline > 0 {
		return "omnichannel"
	}
	if ocOnline > 0 {
		return "online"
	}
	if ocOffline > 0 {
		return "offline"
	}
	return ""
}

func getIntFromCustomer(c *crmmodels.CrmCustomer, key string) int {
	if c == nil {
		return 0
	}
	if c.CurrentMetrics != nil {
		return GetIntFromNestedMetrics(c.CurrentMetrics, key)
	}
	switch key {
	case "orderCountOnline":
		return c.OrderCountOnline
	case "orderCountOffline":
		return c.OrderCountOffline
	}
	return 0
}

// ComputeLoyaltyStage trả về stage theo order_count.
// core | repeat | one_time
func ComputeLoyaltyStage(orderCount int) string {
	if orderCount >= loyaltyCore {
		return "core"
	}
	if orderCount >= loyaltyRepeat {
		return "repeat"
	}
	if orderCount >= 1 {
		return "one_time"
	}
	return ""
}

// ComputeMomentumStage trả về stage theo revenue_last_30d vs revenue_last_90d.
// rising | stable | declining | lost
func ComputeMomentumStage(c *crmmodels.CrmCustomer) string {
	lastOrderAt := getLastOrderAt(c)
	daysSince := daysSinceLastOrder(lastOrderAt)
	rev30 := getFloatFromCustomer(c, "revenueLast30d")
	rev90 := getFloatFromCustomer(c, "revenueLast90d")
	totalSpent := getTotalSpent(c)

	if daysSince > lifecycleCooling {
		return "lost"
	}
	if rev90 <= 0 && totalSpent > 0 {
		return "lost"
	}
	if rev90 > 0 && rev30 <= 0 && daysSince <= lifecycleCooling {
		return "declining"
	}
	if rev30 <= 0 {
		return "lost"
	}
	denom := rev90
	if denom < 1 {
		denom = 1
	}
	ratio := rev30 / denom
	if ratio > momentumRising {
		return "rising"
	}
	if ratio >= momentumStableLo && ratio <= momentumStableHi {
		return "stable"
	}
	if ratio < momentumStableLo {
		return "stable"
	}
	return "stable"
}

func getFloatFromCustomer(c *crmmodels.CrmCustomer, key string) float64 {
	if c == nil {
		return 0
	}
	if c.CurrentMetrics != nil {
		return GetFloatFromNestedMetrics(c.CurrentMetrics, key)
	}
	switch key {
	case "revenueLast30d":
		return c.RevenueLast30d
	case "revenueLast90d":
		return c.RevenueLast90d
	}
	return 0
}

// ComputeClassificationFromMetrics trả về map các field phân loại để $set vào crm_customers.
// Dùng khi đã có metrics từ aggregate (RefreshMetrics, Merge) — lưu classification hiện tại.
// metricsSnapshot trong activity history giữ lịch sử theo từng sự kiện.
func ComputeClassificationFromMetrics(totalSpent float64, orderCount int, lastOrderAt int64, revenueLast30d, revenueLast90d float64, orderCountOnline, orderCountOffline int, hasConversation bool) map[string]interface{} {
	c := &crmmodels.CrmCustomer{
		TotalSpent:        totalSpent,
		OrderCount:        orderCount,
		LastOrderAt:       lastOrderAt,
		RevenueLast30d:    revenueLast30d,
		RevenueLast90d:    revenueLast90d,
		OrderCountOnline:  orderCountOnline,
		OrderCountOffline: orderCountOffline,
		HasConversation:   hasConversation,
	}
	return map[string]interface{}{
		"valueTier":      ComputeValueTier(totalSpent),
		"lifecycleStage": ComputeLifecycleStage(lastOrderAt),
		"journeyStage":   ComputeJourneyStage(c),
		"channel":        ComputeChannel(c),
		"loyaltyStage":   ComputeLoyaltyStage(orderCount),
		"momentumStage":  ComputeMomentumStage(c),
	}
}
