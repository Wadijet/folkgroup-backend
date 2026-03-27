// Package orderintelsvc — Tính Raw → L1 → L2 → L3 → Flags từ bản ghi pc_pos_orders.
package orderintelsvc

import (
	pcmodels "meta_commerce/internal/api/pc/models"
	orderintelmodels "meta_commerce/internal/api/orderintel/models"
)

// ComputeSnapshot tính snapshot intelligence từ đơn POS (không ghi DB).
func ComputeSnapshot(o *pcmodels.PcPosOrder, nowMs int64) *orderintelmodels.OrderIntelligenceSnapshot {
	if o == nil {
		return nil
	}
	status := o.Status
	if pd := mapFromIface(o.PosData); pd != nil {
		if s, ok := intFromMap(pd, "status"); ok {
			status = s
		}
	}
	l1 := orderintelmodels.OrderLayer1{Stage: mapStatusToStage(status)}
	total := totalAfterDiscountVND(o)
	l2 := orderintelmodels.OrderLayer2{
		AOVTier:               tierFromAOV(total),
		ConversionQuality:     deriveConversionQuality(o),
		FulfillmentLatency:    deriveFulfillmentLatency(status, o.InsertedAt, o.PosUpdatedAt, nowMs),
		ReturnRisk:            deriveReturnRisk(status),
		TotalAfterDiscountVND: total,
	}
	l3 := orderintelmodels.OrderLayer3{
		SourceAttribution: deriveSourceAttribution(o),
		DelayPattern:      deriveDelayPattern(l2.FulfillmentLatency),
		HighValueSignal:   l2.AOVTier == "premium" || l2.AOVTier == "high",
		AtRiskReturn:      l3AtRiskReturn(l2.ReturnRisk),
	}
	flags := deriveFlags(l1, l2, l3)

	tr := orderintelmodels.OrderIntelTrace{
		AdID:           stringFieldFromPos(o, "ad_id"),
		PostID:         o.PostId,
		ConversationID: stringFieldFromPos(o, "conversation_id"),
		CustomerID:     o.CustomerId,
	}
	if tr.CustomerID == "" && o.LinksCustomerUid != "" {
		tr.CustomerID = o.LinksCustomerUid
	}

	return &orderintelmodels.OrderIntelligenceSnapshot{
		OrderUid:            o.Uid,
		OwnerOrganizationID: o.OwnerOrganizationID,
		OrderID:             o.OrderId,
		Layer1:              l1,
		Layer2:              l2,
		Layer3:              l3,
		Flags:               flags,
		Trace:               tr,
		UpdatedAt:           nowMs,
		CreatedAt:             nowMs,
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

func deriveConversionQuality(o *pcmodels.PcPosOrder) string {
	ad := stringFieldFromPos(o, "ad_id")
	src := orderSources(o)
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

func deriveSourceAttribution(o *pcmodels.PcPosOrder) string {
	if stringFieldFromPos(o, "ad_id") != "" {
		return "ads"
	}
	for _, x := range orderSources(o) {
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

func totalAfterDiscountVND(o *pcmodels.PcPosOrder) float64 {
	pd := mapFromIface(o.PosData)
	if pd == nil {
		return 0
	}
	if v, ok := floatFromMap(pd, "total_price_after_sub_discount"); ok {
		return v
	}
	if v, ok := floatFromMap(pd, "total_price"); ok {
		return v
	}
	return 0
}

func stringFieldFromPos(o *pcmodels.PcPosOrder, key string) string {
	pd := mapFromIface(o.PosData)
	if pd == nil {
		return ""
	}
	if s, ok := pd[key].(string); ok && s != "" {
		return s
	}
	return ""
}

func orderSources(o *pcmodels.PcPosOrder) []string {
	pd := mapFromIface(o.PosData)
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

