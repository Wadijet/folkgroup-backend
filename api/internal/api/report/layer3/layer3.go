// Package layer3 - Logic derive Lớp 3 (First, Repeat, VIP, Inactive) từ metricsSnapshot.
// Dùng cho report aggregation và có thể dùng cho metricsSnapshot khi lưu.
package layer3

import (
	"time"
)

const (
	msPerDay                    = 24 * 60 * 60 * 1000
	firstHighAOV                = 500_000
	firstEntryAOV               = 150_000
	firstReorderTooEarly        = 7
	firstReorderExpectedMax     = 60
	repeatFreqEarlyMax          = 7
	repeatFreqOnTrackMax        = 45
	repeatFreqDelayedMax        = 90
	repeatSkuMulti              = 3
	vipSilverMax                = 12
	vipGoldMax                  = 25
	vipPlatinumMax              = 40
	vipSingleLineMax            = 2
	vipMultiLineMax             = 7
	vipSpendTrendThreshold      = 0.15
)

// Layer3Aggregate kết quả derive Lớp 3 cho 1 khách — dùng để đếm phân bố.
type Layer3Aggregate struct {
	First    *FirstAgg
	Repeat   *RepeatAgg
	Vip      *VipAgg
	Inactive *InactiveAgg
	Engaged  *EngagedAgg
}

// EngagedAgg Lớp 3 cho khách engaged (có hội thoại, chưa có đơn). Theo ENGAGED_INTELLIGENCE_LAYER.
type EngagedAgg struct {
	ConversationTemperature string // hot|warm|cooling|cold — theo daysSinceLastConversation
	EngagementDepth         string // light|medium|deep — theo totalMessages
	SourceType              string // organic|ads — theo conversationFromAds
}

type FirstAgg struct {
	PurchaseQuality, ExperienceQuality, EngagementAfterPurchase, ReorderTiming, RepeatProbability string
}
type RepeatAgg struct {
	RepeatDepth, RepeatFrequency, SpendMomentum, ProductExpansion, EmotionalEngagement, UpgradePotential string
}
type VipAgg struct {
	VipDepth, SpendTrend, ProductDiversity, EngagementLevel, RiskScore string
}
type InactiveAgg struct {
	EngagementDrop, ReactivationPotential string
}

// DeriveFromNested derive Lớp 3 từ metricsSnapshot nested (raw/layer1/layer2/layer3). endMs = thời điểm cuối chu kỳ (Unix ms).
func DeriveFromNested(m map[string]interface{}, endMs int64) *Layer3Aggregate {
	flat := extractFlatFromNested(m)
	return DeriveFromMap(flat, endMs)
}

func extractFlatFromNested(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	if _, has := m["raw"]; !has {
		return nil
	}
	out := make(map[string]interface{})
	if raw, ok := m["raw"].(map[string]interface{}); ok {
		for k, v := range raw {
			out[k] = v
		}
	}
	if l1, ok := m["layer1"].(map[string]interface{}); ok {
		for k, v := range l1 {
			out[k] = v
		}
	}
	if l2, ok := m["layer2"].(map[string]interface{}); ok {
		for k, v := range l2 {
			out[k] = v
		}
	}
	return out
}

// DeriveFromMap derive Lớp 3 từ metricsSnapshot flat. endMs = thời điểm cuối chu kỳ (Unix ms).
func DeriveFromMap(m map[string]interface{}, endMs int64) *Layer3Aggregate {
	if m == nil {
		return nil
	}
	journeyStage := getStr(m, "journeyStage")
	orderCount := getInt(m, "orderCount")
	lifecycleStage := getStr(m, "lifecycleStage")
	valueTier := getStr(m, "valueTier")
	daysSinceLast := computeDaysSinceLast(getInt64(m, "lastOrderAt"), endMs)

	out := &Layer3Aggregate{}
	if journeyStage == "first" && orderCount == 1 {
		out.First = deriveFirst(m, daysSinceLast)
	}
	if journeyStage == "repeat" && orderCount >= 2 {
		out.Repeat = deriveRepeat(m, daysSinceLast)
	}
	if (journeyStage == "vip" || valueTier == "vip") && orderCount >= 8 {
		out.Vip = deriveVip(m, daysSinceLast)
	}
	if (lifecycleStage == "cooling" || lifecycleStage == "inactive" || lifecycleStage == "dead") && orderCount >= 1 {
		out.Inactive = deriveInactive(m)
	}
	if journeyStage == "engaged" && orderCount == 0 {
		out.Engaged = deriveEngaged(m, endMs)
	}
	return out
}

func computeDaysSinceLast(lastOrderAtMs int64, endMs int64) int64 {
	if lastOrderAtMs <= 0 {
		return -1
	}
	return (endMs - lastOrderAtMs) / msPerDay
}

func deriveFirst(m map[string]interface{}, daysSinceLast int64) *FirstAgg {
	pq := firstPurchaseQuality(getFloat(m, "avgOrderValue"))
	eq := firstExperienceQuality(getInt(m, "cancelledOrderCount"))
	eng := firstEngagement(getInt64(m, "lastConversationAt"), getInt64(m, "lastOrderAt"))
	rt := firstReorderTiming(daysSinceLast)
	rp := firstRepeatProbability(pq, eq, eng, rt)
	return &FirstAgg{PurchaseQuality: pq, ExperienceQuality: eq, EngagementAfterPurchase: eng, ReorderTiming: rt, RepeatProbability: rp}
}

func firstPurchaseQuality(aov float64) string {
	if aov >= firstHighAOV {
		return "high_aov"
	}
	if aov < firstEntryAOV {
		return "entry"
	}
	return "medium"
}
func firstExperienceQuality(cancelled int) string {
	if cancelled > 0 {
		return "risk"
	}
	return "smooth"
}
func firstEngagement(lastConv, lastOrder int64) string {
	if lastConv <= 0 {
		return "silent"
	}
	convMs := lastConv
	if convMs < 1e12 {
		convMs *= 1000
	}
	if convMs > lastOrder && lastOrder > 0 {
		return "post_purchase_engaged"
	}
	return "silent"
}
func firstReorderTiming(days int64) string {
	if days < 0 {
		return "within_expected"
	}
	if days < firstReorderTooEarly {
		return "too_early"
	}
	if days > firstReorderExpectedMax {
		return "overdue"
	}
	return "within_expected"
}
func firstRepeatProbability(pq, eq, eng, rt string) string {
	score := 0
	if pq == "high_aov" {
		score += 2
	} else if pq == "medium" {
		score += 1
	}
	if eq == "smooth" {
		score += 2
	} else if eq == "risk" {
		score -= 1
	}
	if eng == "post_purchase_engaged" {
		score += 2
	}
	if rt == "within_expected" || rt == "too_early" {
		score += 1
	} else if rt == "overdue" {
		score -= 1
	}
	if score >= 5 {
		return "high"
	}
	if score <= 1 {
		return "low"
	}
	return "medium"
}

func deriveRepeat(m map[string]interface{}, daysSinceLast int64) *RepeatAgg {
	oc := getInt(m, "orderCount")
	lastMs := getInt64(m, "lastOrderAt")
	secondMs := getInt64(m, "secondLastOrderAt")
	avgVal := getFloat(m, "avgOrderValue")
	rev30 := getFloat(m, "revenueLast30d")
	ord30 := getInt(m, "ordersLast30d")
	skuCount := ownedSkuCount(m)
	totalSpend := getFloat(m, "totalSpent")
	lastConv := getInt64(m, "lastConversationAt")

	rd := repeatDepth(oc)
	rf := repeatFrequency(daysSinceLast, lastMs, secondMs, oc)
	sm := repeatSpendMomentum(avgVal, rev30, ord30)
	pe := repeatProductExpansion(skuCount)
	ee := repeatEngagement(lastConv, lastMs)
	up := repeatUpgradePotential(rd, rf, sm, ee, totalSpend)
	return &RepeatAgg{RepeatDepth: rd, RepeatFrequency: rf, SpendMomentum: sm, ProductExpansion: pe, EmotionalEngagement: ee, UpgradePotential: up}
}

func ownedSkuCount(m map[string]interface{}) int {
	if n := getInt(m, "ownedSkuCount"); n > 0 {
		return n
	}
	v, ok := m["ownedSkuQuantities"]
	if !ok || v == nil {
		return 0
	}
	if om, ok := v.(map[string]interface{}); ok {
		return len(om)
	}
	return 0
}

func repeatDepth(oc int) string {
	switch {
	case oc == 2:
		return "R1"
	case oc >= 3 && oc <= 4:
		return "R2"
	case oc >= 5 && oc <= 7:
		return "R3"
	case oc >= 8:
		return "R4"
	}
	return "R1"
}
func repeatFrequency(days int64, lastMs, secondMs int64, oc int) string {
	if days < 0 {
		return "on_track"
	}
	avgDays := float64(-1)
	if oc >= 2 && lastMs > 0 && secondMs > 0 {
		diff := lastMs - secondMs
		if diff > 0 {
			avgDays = float64(diff) / float64(msPerDay)
		}
	}
	if avgDays > 0 {
		if days < int64(avgDays*0.5) {
			return "early"
		}
		if days <= int64(avgDays*1.5) {
			return "on_track"
		}
		if days <= int64(avgDays*2) {
			return "delayed"
		}
		return "overdue"
	}
	if days < repeatFreqEarlyMax {
		return "early"
	}
	if days <= repeatFreqOnTrackMax {
		return "on_track"
	}
	if days <= repeatFreqDelayedMax {
		return "delayed"
	}
	return "overdue"
}
func repeatSpendMomentum(avgVal, rev30 float64, ord30 int) string {
	if ord30 <= 0 || avgVal <= 0 {
		return "stable"
	}
	lastAOV := rev30 / float64(ord30)
	diff := (lastAOV - avgVal) / avgVal
	if diff >= 0.15 {
		return "upscaling"
	}
	if diff <= -0.15 {
		return "downscaling"
	}
	return "stable"
}
func repeatProductExpansion(skuCount int) string {
	if skuCount >= repeatSkuMulti {
		return "multi_category"
	}
	return "single_category"
}
func repeatEngagement(lastConv, lastMs int64) string {
	if lastConv <= 0 {
		return "silent_repeat"
	}
	convMs := lastConv
	if convMs < 1e12 {
		convMs *= 1000
	}
	if convMs > lastMs && lastMs > 0 {
		return "engaged_repeat"
	}
	return "transactional_repeat"
}
func repeatUpgradePotential(rd, rf, sm, ee string, totalSpend float64) string {
	score := 0
	switch rd {
	case "R4", "R3":
		score += 2
	case "R2":
		score += 1
	}
	switch rf {
	case "on_track":
		score += 2
	case "early":
		score += 1
	case "overdue":
		score -= 1
	}
	switch sm {
	case "upscaling":
		score += 2
	case "stable":
		score += 1
	case "downscaling":
		score -= 1
	}
	switch ee {
	case "engaged_repeat":
		score += 2
	case "transactional_repeat":
		score += 1
	}
	if totalSpend >= 2_000_000 {
		score += 1
	}
	if score >= 6 {
		return "high"
	}
	if score <= 2 {
		return "low"
	}
	return "medium"
}

func deriveVip(m map[string]interface{}, daysSinceLast int64) *VipAgg {
	oc := getInt(m, "orderCount")
	avgVal := getFloat(m, "avgOrderValue")
	rev30 := getFloat(m, "revenueLast30d")
	ord30 := getInt(m, "ordersLast30d")
	skuCount := ownedSkuCount(m)
	lastConv := getInt64(m, "lastConversationAt")
	lastMs := getInt64(m, "lastOrderAt")
	lifecycle := getStr(m, "lifecycleStage")

	vd := vipDepth(oc)
	st := vipSpendTrend(avgVal, rev30, ord30)
	pd := vipProductDiversity(skuCount)
	el := vipEngagement(lastConv, lastMs)
	rs := vipRiskScore(lifecycle, st, el, daysSinceLast)
	return &VipAgg{VipDepth: vd, SpendTrend: st, ProductDiversity: pd, EngagementLevel: el, RiskScore: rs}
}

func vipDepth(oc int) string {
	switch {
	case oc <= vipSilverMax:
		return "silver_vip"
	case oc <= vipGoldMax:
		return "gold_vip"
	case oc <= vipPlatinumMax:
		return "platinum_vip"
	}
	return "core_patron"
}
func vipSpendTrend(avgVal, rev30 float64, ord30 int) string {
	if ord30 <= 0 || avgVal <= 0 {
		return "stable_vip"
	}
	lastAOV := rev30 / float64(ord30)
	diff := (lastAOV - avgVal) / avgVal
	if diff >= vipSpendTrendThreshold {
		return "upscaling_vip"
	}
	if diff <= -vipSpendTrendThreshold {
		return "downscaling_vip"
	}
	return "stable_vip"
}
func vipProductDiversity(skuCount int) string {
	if skuCount <= vipSingleLineMax {
		return "single_line_vip"
	}
	if skuCount <= vipMultiLineMax {
		return "multi_line_vip"
	}
	return "full_portfolio_vip"
}
func vipEngagement(lastConv, lastMs int64) string {
	if lastConv <= 0 {
		return "silent_vip"
	}
	convMs := lastConv
	if convMs < 1e12 {
		convMs *= 1000
	}
	if convMs > lastMs && lastMs > 0 {
		return "engaged_vip"
	}
	return "transactional_vip"
}
func vipRiskScore(lifecycle, spendTrend, engagement string, days int64) string {
	score := 0
	switch lifecycle {
	case "active":
		score += 2
	case "cooling":
		score += 1
	case "inactive":
		score -= 1
	case "dead":
		score -= 2
	default:
		score += 2
	}
	switch spendTrend {
	case "upscaling_vip":
		score += 2
	case "stable_vip":
		score += 1
	case "downscaling_vip":
		score -= 2
	}
	switch engagement {
	case "engaged_vip":
		score += 2
	case "silent_vip":
		score -= 1
	}
	if days > 90 {
		score -= 2
	} else if days > 60 {
		score -= 1
	}
	if score <= -4 {
		return "critical"
	}
	if score <= -1 {
		return "high"
	}
	if score <= 2 {
		return "medium"
	}
	return "low"
}

func deriveInactive(m map[string]interface{}) *InactiveAgg {
	lastConv := getInt64(m, "lastConversationAt")
	lastMs := getInt64(m, "lastOrderAt")
	valueTier := getStr(m, "valueTier")
	lifecycle := getStr(m, "lifecycleStage")
	oc := getInt(m, "orderCount")

	ed := inactiveEngagementDrop(lastConv, lastMs)
	rp := inactiveReactivationPotential(valueTier, lifecycle, oc, ed)
	return &InactiveAgg{EngagementDrop: ed, ReactivationPotential: rp}
}

// deriveEngaged Lớp 3 cho khách engaged (có hội thoại, chưa có đơn). Theo ENGAGED_INTELLIGENCE_LAYER Phase 1.
func deriveEngaged(m map[string]interface{}, endMs int64) *EngagedAgg {
	lastConv := getInt64(m, "lastConversationAt")
	totalMsgs := getInt(m, "totalMessages")
	fromAds := getBool(m, "conversationFromAds")

	temp := engagedConversationTemperature(lastConv, endMs)
	depth := engagedEngagementDepth(totalMsgs)
	source := engagedSourceType(fromAds)
	return &EngagedAgg{ConversationTemperature: temp, EngagementDepth: depth, SourceType: source}
}

func engagedConversationTemperature(lastConvAtMs, endMs int64) string {
	if lastConvAtMs <= 0 {
		return "cold"
	}
	if lastConvAtMs < 1e12 {
		lastConvAtMs *= 1000
	}
	days := (endMs - lastConvAtMs) / msPerDay
	if days <= 1 {
		return "hot"
	}
	if days <= 3 {
		return "warm"
	}
	if days <= 7 {
		return "cooling"
	}
	return "cold"
}

func engagedEngagementDepth(totalMessages int) string {
	if totalMessages <= 0 {
		return "light"
	}
	if totalMessages <= 3 {
		return "light"
	}
	if totalMessages <= 10 {
		return "medium"
	}
	return "deep"
}

func engagedSourceType(fromAds bool) string {
	if fromAds {
		return "ads"
	}
	return "organic"
}

func getBool(m map[string]interface{}, k string) bool {
	v, ok := m[k]
	if !ok || v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if i, ok := v.(int); ok {
		return i != 0
	}
	if i, ok := v.(int64); ok {
		return i != 0
	}
	return false
}

func inactiveEngagementDrop(lastConv, lastMs int64) string {
	if lastConv <= 0 {
		return "no_engagement"
	}
	convMs := lastConv
	if convMs < 1e12 {
		convMs *= 1000
	}
	if convMs > lastMs && lastMs > 0 {
		return "had_post_engagement"
	}
	return "dropped_engagement"
}
func inactiveReactivationPotential(valueTier, lifecycle string, oc int, engagement string) string {
	score := 0
	switch valueTier {
	case "vip":
		score += 3
	case "high":
		score += 2
	case "medium":
		score += 1
	}
	switch lifecycle {
	case "cooling":
		score += 3
	case "inactive":
		score += 2
	case "dead":
		score += 0
	}
	if oc >= 8 {
		score += 2
	} else if oc >= 2 {
		score += 1
	}
	switch engagement {
	case "had_post_engagement":
		score += 2
	case "dropped_engagement":
		score += 1
	}
	if score >= 7 {
		return "high"
	}
	if score <= 3 {
		return "low"
	}
	return "medium"
}

func getStr(m map[string]interface{}, k string) string {
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func getInt(m map[string]interface{}, k string) int {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	}
	return 0
}
func getInt64(m map[string]interface{}, k string) int64 {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}
func getFloat(m map[string]interface{}, k string) float64 {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}

// NowMs trả về Unix ms hiện tại — helper cho buildMetricsSnapshot.
func NowMs() int64 {
	return time.Now().UnixMilli()
}

// ToMapForStorage chuyển Layer3Aggregate sang map để lưu vào metricsSnapshot (BSON/JSON).
// Trả về map với keys firstLayer3, repeatLayer3, vipLayer3, inactiveLayer3 (chỉ thêm key khi != nil).
func ToMapForStorage(a *Layer3Aggregate) map[string]interface{} {
	if a == nil {
		return nil
	}
	out := make(map[string]interface{})
	if a.First != nil {
		out["firstLayer3"] = map[string]interface{}{
			"purchaseQuality":        a.First.PurchaseQuality,
			"experienceQuality":     a.First.ExperienceQuality,
			"engagementAfterPurchase": a.First.EngagementAfterPurchase,
			"reorderTiming":          a.First.ReorderTiming,
			"repeatProbability":     a.First.RepeatProbability,
		}
	}
	if a.Repeat != nil {
		out["repeatLayer3"] = map[string]interface{}{
			"repeatDepth":         a.Repeat.RepeatDepth,
			"repeatFrequency":     a.Repeat.RepeatFrequency,
			"spendMomentum":       a.Repeat.SpendMomentum,
			"productExpansion":    a.Repeat.ProductExpansion,
			"emotionalEngagement": a.Repeat.EmotionalEngagement,
			"upgradePotential":    a.Repeat.UpgradePotential,
		}
	}
	if a.Vip != nil {
		out["vipLayer3"] = map[string]interface{}{
			"vipDepth":         a.Vip.VipDepth,
			"spendTrend":       a.Vip.SpendTrend,
			"productDiversity": a.Vip.ProductDiversity,
			"engagementLevel":  a.Vip.EngagementLevel,
			"riskScore":        a.Vip.RiskScore,
		}
	}
	if a.Inactive != nil {
		out["inactiveLayer3"] = map[string]interface{}{
			"engagementDrop":        a.Inactive.EngagementDrop,
			"reactivationPotential": a.Inactive.ReactivationPotential,
		}
	}
	if a.Engaged != nil {
		out["engagedLayer3"] = map[string]interface{}{
			"conversationTemperature": a.Engaged.ConversationTemperature,
			"engagementDepth":         a.Engaged.EngagementDepth,
			"sourceType":              a.Engaged.SourceType,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
