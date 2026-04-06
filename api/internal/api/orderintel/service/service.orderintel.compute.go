// Package orderintelsvc — Tính Raw → L1 → L2 → L3 → Flags từ intelOrderView (nguồn: commerce_orders, fallback pc_pos_orders).
package orderintelsvc

import (
	"strings"

	orderintelmodels "meta_commerce/internal/api/orderintel/models"
)

// intelStatusCodeForView — mã status POS thống nhất cho raw và map stage (ưu tiên posData nếu có).
func intelStatusCodeForView(v *intelOrderView) int {
	if v == nil {
		return 0
	}
	status := v.Status
	if pd := mapFromIface(v.PosData); pd != nil {
		if s, ok := intFromMap(pd, "status"); ok {
			status = s
		}
	}
	return status
}

// BuildOrderIntelRaw gom facts đầu vào pipeline (lớp raw) tại thời điểm evaluatedMs.
func BuildOrderIntelRaw(v *intelOrderView, evaluatedMs int64) orderintelmodels.OrderIntelRaw {
	if v == nil {
		return orderintelmodels.OrderIntelRaw{EvaluatedAtMs: evaluatedMs}
	}
	status := intelStatusCodeForView(v)
	pd := mapFromIface(v.PosData)
	return orderintelmodels.OrderIntelRaw{
		Status:                status,
		InsertedAt:            v.InsertedAt,
		PosUpdatedAt:          v.PosUpdatedAt,
		TotalAfterDiscountVND: totalAfterDiscountVNDFromView(v),
		OrderSources:          orderSourcesFromPosData(pd),
		AdID:                  stringFieldFromPosData(pd, "ad_id"),
		ConversationID:        stringFieldFromPosData(pd, "conversation_id"),
		EvaluatedAtMs:         evaluatedMs,
	}
}

// ComputeSnapshot tính snapshot intelligence (không ghi DB).
// orderUid = canonical ord_*; có thể rỗng — persistence dùng khóa orderId + org.
func ComputeSnapshot(v *intelOrderView, nowMs int64) *orderintelmodels.OrderIntelligenceSnapshot {
	if v == nil {
		return nil
	}
	uid := strings.TrimSpace(v.Uid)
	if uid == "" && v.OrderId <= 0 && v.ID.IsZero() {
		return nil
	}
	status := intelStatusCodeForView(v)
	l1 := orderintelmodels.OrderLayer1{Stage: mapStatusToStage(status)}
	total := totalAfterDiscountVNDFromView(v)
	l2 := orderintelmodels.OrderLayer2{
		AOVTier:               tierFromAOV(total),
		ConversionQuality:     deriveConversionQualityFromView(v),
		FulfillmentLatency:    deriveFulfillmentLatency(status, v.InsertedAt, v.PosUpdatedAt, nowMs),
		ReturnRisk:            deriveReturnRisk(status),
		TotalAfterDiscountVND: total,
	}
	l3 := orderintelmodels.OrderLayer3{
		SourceAttribution: deriveSourceAttributionFromView(v),
		DelayPattern:      deriveDelayPattern(l2.FulfillmentLatency),
		HighValueSignal:   l2.AOVTier == "premium" || l2.AOVTier == "high",
		AtRiskReturn:      l3AtRiskReturn(l2.ReturnRisk),
	}
	flags := deriveFlags(l1, l2, l3)

	tr := orderintelmodels.OrderIntelTrace{
		AdID:           stringFieldFromPosData(v.PosData, "ad_id"),
		PostID:         v.PostId,
		ConversationID: stringFieldFromPosData(v.PosData, "conversation_id"),
		CustomerID:     v.CustomerId,
	}
	if tr.CustomerID == "" && v.LinksCustomerUid != "" {
		tr.CustomerID = v.LinksCustomerUid
	}

	return &orderintelmodels.OrderIntelligenceSnapshot{
		OrderUid:            uid,
		OwnerOrganizationID: v.OwnerOrganizationID,
		OrderID:             v.OrderId,
		Layer1:              l1,
		Layer2:              l2,
		Layer3:              l3,
		Flags:               flags,
		Trace:               tr,
		UpdatedAt:           nowMs,
		CreatedAt:           nowMs,
	}
}

func mapStatusToStage(status int) string {
	switch status {
	case 0, 17:
		return "new"
	case 1, 8, 9, 12, 13, 20:
		return "processing"
	case 2:
		return "fulfilled"
	case 3, 16:
		return "completed"
	case 6, 7:
		return "canceled"
	case 4, 5, 15:
		return "returned"
	default:
		return "unknown"
	}
}

func tierFromAOV(total float64) string {
	switch {
	case total >= 10_000_000:
		return "premium"
	case total >= 2_000_000:
		return "high"
	case total >= 500_000:
		return "medium"
	case total > 0:
		return "low"
	default:
		return "low"
	}
}

func deriveConversionQualityFromView(v *intelOrderView) string {
	ad := stringFieldFromPosData(v.PosData, "ad_id")
	src := orderSourcesFromPosData(v.PosData)
	hasAdsSource := false
	for _, x := range src {
		if x == "-1" {
			hasAdsSource = true
			break
		}
	}
	if ad != "" || hasAdsSource {
		return "strong"
	}
	if len(src) == 0 && ad == "" {
		return "weak"
	}
	return "normal"
}

func deriveFulfillmentLatency(status int, insertedMs, _ /* updatedMs */, nowMs int64) string {
	st := mapStatusToStage(status)
	if st == "completed" || st == "fulfilled" {
		return "on_time"
	}
	if st == "canceled" || st == "returned" {
		return "unknown"
	}
	if insertedMs <= 0 {
		return "unknown"
	}
	days := (nowMs - insertedMs) / (86400 * 1000)
	if days >= 14 {
		return "critical"
	}
	if days >= 5 {
		return "delayed"
	}
	return "on_time"
}

func deriveReturnRisk(status int) string {
	switch mapStatusToStage(status) {
	case "returned":
		return "high"
	case "canceled":
		return "medium"
	default:
		return "low"
	}
}

func deriveSourceAttributionFromView(v *intelOrderView) string {
	if stringFieldFromPosData(v.PosData, "ad_id") != "" {
		return "ads"
	}
	for _, x := range orderSourcesFromPosData(v.PosData) {
		if x == "-1" {
			return "ads"
		}
	}
	return "organic"
}

func deriveDelayPattern(lat string) string {
	switch lat {
	case "critical":
		return "severe"
	case "delayed":
		return "mild"
	default:
		return "none"
	}
}

func l3AtRiskReturn(rr string) string {
	if rr == "high" {
		return "high"
	}
	if rr == "medium" {
		return "medium"
	}
	return "low"
}

func deriveFlags(l1 orderintelmodels.OrderLayer1, l2 orderintelmodels.OrderLayer2, l3 orderintelmodels.OrderLayer3) []string {
	var out []string
	if l2.AOVTier == "premium" && l1.Stage == "completed" {
		out = append(out, "high_value_order")
	}
	if l2.FulfillmentLatency == "delayed" || l2.FulfillmentLatency == "critical" {
		out = append(out, "delayed_fulfillment")
	}
	if l3.AtRiskReturn == "high" {
		out = append(out, "at_risk_return")
	}
	if l2.ConversionQuality == "weak" && l1.Stage == "completed" {
		out = append(out, "conversion_quality_drop")
	}
	return out
}

func totalAfterDiscountVNDFromView(v *intelOrderView) float64 {
	pd := mapFromIface(v.PosData)
	if pd == nil {
		return 0
	}
	if val, ok := floatFromMap(pd, "total_price_after_sub_discount"); ok {
		return val
	}
	if val, ok := floatFromMap(pd, "total_price"); ok {
		return val
	}
	return 0
}

func stringFieldFromPosData(pd map[string]interface{}, key string) string {
	if pd == nil {
		return ""
	}
	if s, ok := pd[key].(string); ok && s != "" {
		return s
	}
	return ""
}

func orderSourcesFromPosData(pd map[string]interface{}) []string {
	if pd == nil {
		return nil
	}
	raw, ok := pd["order_sources"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func mapFromIface(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	return m
}

func intFromMap(m map[string]interface{}, key string) (int, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case int:
		return t, true
	case int32:
		return int(t), true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	default:
		return 0, false
	}
}

func floatFromMap(m map[string]interface{}, key string) (float64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}
