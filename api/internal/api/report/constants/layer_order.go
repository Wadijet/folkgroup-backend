// Package constants - Thứ tự chuẩn cho các mức trong từng layer.
// Dùng khi build/serialize distribution, LTV, matrix — đảm bảo output luôn theo thứ tự.
package constants

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/bson"
)

// SortedInt64Map map với thứ tự key cố định khi marshal JSON.
type SortedInt64Map struct {
	Order []string
	Data  map[string]int64
}

// MarshalJSON xuất JSON với keys theo Order. Nếu Order nil → dùng alphabetical.
func (s SortedInt64Map) MarshalJSON() ([]byte, error) {
	if s.Data == nil {
		return []byte("{}"), nil
	}
	keys := getKeysInOrder(s.Data, s.Order)
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(fmt.Sprintf(`"%s":%d`, k, s.Data[k]))
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// MarshalBSON lưu MongoDB dưới dạng map (không lưu Order) — client đọc lại đúng format.
func (s SortedInt64Map) MarshalBSON() ([]byte, error) {
	if s.Data == nil {
		return bson.Marshal(map[string]int64{})
	}
	return bson.Marshal(s.Data)
}

// getKeysInOrder trả về keys theo Order; nếu Order nil → alphabetical.
func getKeysInOrder(data map[string]int64, order []string) []string {
	seen := make(map[string]bool)
	var out []string
	if len(order) > 0 {
		for _, k := range order {
			if _, ok := data[k]; ok {
				out = append(out, k)
				seen[k] = true
			}
		}
	}
	var rest []string
	for k := range data {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	if len(rest) > 0 {
		sortStrings(rest)
		out = append(out, rest...)
	}
	return out
}

func sortStrings(s []string) {
	sort.Strings(s)
}

// SortedFloat64Map map với thứ tự key cố định khi marshal JSON.
type SortedFloat64Map struct {
	Order []string
	Data  map[string]float64
}

// MarshalJSON xuất JSON với keys theo Order.
func (s SortedFloat64Map) MarshalJSON() ([]byte, error) {
	if s.Data == nil {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for _, k := range s.Order {
		if v, ok := s.Data[k]; ok {
			if !first {
				buf.WriteByte(',')
			}
			buf.WriteString(fmt.Sprintf(`"%s":`, k))
			b, _ := json.Marshal(v)
			buf.Write(b)
			first = false
		}
	}
	for k, v := range s.Data {
		if !contains(s.Order, k) {
			if !first {
				buf.WriteByte(',')
			}
			buf.WriteString(fmt.Sprintf(`"%s":`, k))
			b, _ := json.Marshal(v)
			buf.Write(b)
			first = false
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// MarshalBSON lưu MongoDB dưới dạng map (không lưu Order).
func (s SortedFloat64Map) MarshalBSON() ([]byte, error) {
	if s.Data == nil {
		return bson.Marshal(map[string]float64{})
	}
	return bson.Marshal(s.Data)
}

// SortedMapStringInterface map[string]interface{} với thứ tự key cố định.
type SortedMapStringInterface struct {
	Order []string
	Data  map[string]interface{}
}

// MarshalJSON xuất JSON với keys theo Order.
func (s SortedMapStringInterface) MarshalJSON() ([]byte, error) {
	if s.Data == nil {
		return []byte("{}"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	for _, k := range s.Order {
		if v, ok := s.Data[k]; ok {
			if !first {
				buf.WriteByte(',')
			}
			b, _ := json.Marshal(k)
			buf.Write(b)
			buf.WriteByte(':')
			buf.Write(mustMarshal(v))
			first = false
		}
	}
	for k, v := range s.Data {
		if !contains(s.Order, k) {
			if !first {
				buf.WriteByte(',')
			}
			b, _ := json.Marshal(k)
			buf.Write(b)
			buf.WriteByte(':')
			buf.Write(mustMarshal(v))
			first = false
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// JourneyOrder thứ tự Lớp 1 (hành trình trưởng thành).
// blocked_spam là nhóm đặc biệt — đặt sau promoter.
var JourneyOrder = []string{"visitor", "engaged", "first", "repeat", "promoter", "blocked_spam", "_unspecified"}

// ValueOrder thứ tự Value (thấp → cao: new → low → medium → high → top).
var ValueOrder = []string{"new", "low", "medium", "high", "top", "_unspecified"}

// RecencyOrder thứ tự Recency (tích cực → chết). Đổi tên từ Lifecycle — chỉ khách đã mua.
var RecencyOrder = []string{"active", "cooling", "inactive", "dead", "_unspecified"}

// ChannelOrder thứ tự Channel ("" = chưa mua).
var ChannelOrder = []string{"online", "offline", "omnichannel", "", "_unspecified"}

// LoyaltyOrder thứ tự Loyalty (ít → nhiều: one_time → repeat → core).
var LoyaltyOrder = []string{"one_time", "repeat", "core", "_unspecified"}

// MomentumOrder thứ tự Momentum.
var MomentumOrder = []string{"rising", "stable", "declining", "lost", "_unspecified"}

// CeoGroupOrder thứ tự CEO groups — nhóm phái sinh (không thuộc 3 layer gốc).
var CeoGroupOrder = []string{"vip_active", "vip_inactive", "rising", "new", "one_time", "dead", "_other", "_unspecified"}

// Layer3GroupOrder thứ tự nhóm Lớp 3.
var Layer3GroupOrder = []string{"first", "repeat", "top", "inactive", "engaged"}

// --- Lớp 3: Thứ tự tiêu chí chi tiết từng nhóm ---

// FirstLayer3CriteriaOrder thứ tự tiêu chí trong nhóm First.
var FirstLayer3CriteriaOrder = []string{"purchaseQuality", "experienceQuality", "engagementAfterPurchase", "reorderTiming", "repeatProbability"}

// FirstPurchaseQualityOrder giá trị purchaseQuality (entry → medium → high_aov).
var FirstPurchaseQualityOrder = []string{"entry", "medium", "high_aov"}

// FirstExperienceQualityOrder giá trị experienceQuality.
var FirstExperienceQualityOrder = []string{"risk", "smooth"}

// FirstEngagementAfterPurchaseOrder giá trị engagementAfterPurchase.
var FirstEngagementAfterPurchaseOrder = []string{"silent", "post_purchase_engaged"}

// FirstReorderTimingOrder giá trị reorderTiming.
var FirstReorderTimingOrder = []string{"too_early", "within_expected", "overdue"}

// FirstRepeatProbabilityOrder giá trị repeatProbability.
var FirstRepeatProbabilityOrder = []string{"low", "medium", "high"}

// RepeatLayer3CriteriaOrder thứ tự tiêu chí trong nhóm Repeat.
var RepeatLayer3CriteriaOrder = []string{"repeatDepth", "repeatFrequency", "spendMomentum", "productExpansion", "emotionalEngagement", "upgradePotential"}

// RepeatDepthOrder giá trị repeatDepth (R1 → R4).
var RepeatDepthOrder = []string{"R1", "R2", "R3", "R4"}

// RepeatFrequencyOrder giá trị repeatFrequency.
var RepeatFrequencyOrder = []string{"early", "on_track", "delayed", "overdue"}

// RepeatSpendMomentumOrder giá trị spendMomentum.
var RepeatSpendMomentumOrder = []string{"downscaling", "stable", "upscaling"}

// RepeatProductExpansionOrder giá trị productExpansion.
var RepeatProductExpansionOrder = []string{"single_category", "multi_category"}

// RepeatEmotionalEngagementOrder giá trị emotionalEngagement.
var RepeatEmotionalEngagementOrder = []string{"silent_repeat", "transactional_repeat", "engaged_repeat"}

// RepeatUpgradePotentialOrder giá trị upgradePotential.
var RepeatUpgradePotentialOrder = []string{"low", "medium", "high"}

// VipLayer3CriteriaOrder thứ tự tiêu chí trong nhóm VIP.
var VipLayer3CriteriaOrder = []string{"vipDepth", "spendTrend", "productDiversity", "engagementLevel", "riskScore"}

// VipDepthOrder giá trị vipDepth.
var VipDepthOrder = []string{"silver_vip", "gold_vip", "platinum_vip", "core_patron"}

// VipSpendTrendOrder giá trị spendTrend.
var VipSpendTrendOrder = []string{"downscaling_vip", "stable_vip", "upscaling_vip"}

// VipProductDiversityOrder giá trị productDiversity.
var VipProductDiversityOrder = []string{"single_line_vip", "multi_line_vip", "full_portfolio_vip"}

// VipEngagementLevelOrder giá trị engagementLevel.
var VipEngagementLevelOrder = []string{"silent_vip", "transactional_vip", "engaged_vip"}

// VipRiskScoreOrder giá trị riskScore.
var VipRiskScoreOrder = []string{"critical", "high", "medium", "low"}

// InactiveLayer3CriteriaOrder thứ tự tiêu chí trong nhóm Inactive.
var InactiveLayer3CriteriaOrder = []string{"engagementDrop", "reactivationPotential"}

// InactiveEngagementDropOrder giá trị engagementDrop.
var InactiveEngagementDropOrder = []string{"no_engagement", "dropped_engagement", "had_post_engagement"}

// InactiveReactivationPotentialOrder giá trị reactivationPotential.
var InactiveReactivationPotentialOrder = []string{"low", "medium", "high"}

// EngagedLayer3CriteriaOrder thứ tự tiêu chí trong nhóm Engaged.
var EngagedLayer3CriteriaOrder = []string{"conversationTemperature", "engagementDepth", "sourceType"}

// EngagedConversationTemperatureOrder giá trị conversationTemperature.
var EngagedConversationTemperatureOrder = []string{"cold", "cooling", "warm", "hot"}

// EngagedEngagementDepthOrder giá trị engagementDepth.
var EngagedEngagementDepthOrder = []string{"light", "medium", "deep"}

// EngagedSourceTypeOrder giá trị sourceType.
var EngagedSourceTypeOrder = []string{"organic", "ads"}

// ValueOrderForMatrix thứ tự Value cho ma trận (thấp → cao).
var ValueOrderForMatrix = []string{"new", "low", "medium", "high", "top"}

// RecencyOrderForMatrix thứ tự Recency cho ma trận.
var RecencyOrderForMatrix = []string{"active", "cooling", "inactive", "dead"}

// LifecycleOrder alias cho RecencyOrder — backward compat (lifecycleStage trong DB/API).
var LifecycleOrder = RecencyOrder

// LifecycleOrderForMatrix alias.
var LifecycleOrderForMatrix = RecencyOrderForMatrix

// JourneyOrderForMatrix thứ tự Journey cho ma trận/funnel.
// blocked_spam và _unspecified luôn tách riêng nếu có.
var JourneyOrderForMatrix = []string{"visitor", "engaged", "first", "repeat", "promoter", "blocked_spam", "_unspecified"}

// L2ChannelOrderForMatrix thứ tự Channel cho ma trận (có "" cho chưa mua).
var L2ChannelOrderForMatrix = []string{"online", "offline", "omnichannel", ""}

// L2LoyaltyOrderForMatrix thứ tự Loyalty cho ma trận (one_time → repeat → core).
var L2LoyaltyOrderForMatrix = []string{"one_time", "repeat", "core", ""}

// L2MomentumOrderForMatrix thứ tự Momentum cho ma trận.
var L2MomentumOrderForMatrix = []string{"rising", "stable", "declining", "lost", ""}

// OrderForDimension trả về slice thứ tự theo dimension (layer2).
func OrderForDimension(dim string) []string {
	switch dim {
	case "valueTier", "valueTierLTV":
		return ValueOrder
	case "lifecycleStage", "lifecycleStageLTV":
		return LifecycleOrder
	case "channel", "channelLTV":
		return ChannelOrder
	case "loyaltyStage", "loyaltyStageLTV":
		return LoyaltyOrder
	case "momentumStage", "momentumStageLTV":
		return MomentumOrder
	case "ceoGroup", "ceoGroupLTV":
		return CeoGroupOrder
	default:
		return nil
	}
}
