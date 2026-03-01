// Package reportsvc - Tính phát sinh (in/out) cho toàn bộ cấu trúc metrics customer.
// Snapshot chỉ lưu số phát sinh, không lưu số cuối kỳ.
package reportsvc

import (
	"context"

	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/api/report/layer3"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// customerState trạng thái classification của 1 khách tại 1 thời điểm (từ metricsSnapshot).
type customerState struct {
	ValueTier     string
	JourneyStage  string
	LifecycleStage string
	Channel       string
	LoyaltyStage  string
	MomentumStage string
	CeoGroup      string
	TotalSpent    float64
	OrderCount    int
	LastOrderAt   int64
	// Layer 3
	First    *layer3.FirstAgg
	Repeat   *layer3.RepeatAgg
	Vip      *layer3.VipAgg
	Inactive *layer3.InactiveAgg
	Engaged  *layer3.EngagedAgg
}

func stateFromMetricsSnapshot(m map[string]interface{}, endMs int64) customerState {
	s := customerState{}
	if m == nil {
		return s
	}
	s.ValueTier = crmvc.GetStrFromNestedMetrics(m, "valueTier")
	s.JourneyStage = crmvc.GetStrFromNestedMetrics(m, "journeyStage")
	s.LifecycleStage = crmvc.GetStrFromNestedMetrics(m, "lifecycleStage")
	s.Channel = crmvc.GetStrFromNestedMetrics(m, "channel")
	s.LoyaltyStage = crmvc.GetStrFromNestedMetrics(m, "loyaltyStage")
	s.MomentumStage = crmvc.GetStrFromNestedMetrics(m, "momentumStage")
	s.TotalSpent = crmvc.GetFloatFromNestedMetrics(m, "totalSpent")
	s.OrderCount = crmvc.GetIntFromNestedMetrics(m, "orderCount")
	s.LastOrderAt = crmvc.GetInt64FromNestedMetrics(m, "lastOrderAt")

	if s.ValueTier == "" {
		s.ValueTier = "_unspecified"
	}
	if s.JourneyStage == "" {
		s.JourneyStage = "_unspecified"
	}
	if s.LifecycleStage == "" {
		s.LifecycleStage = "_unspecified"
	}
	if s.Channel == "" {
		s.Channel = "_unspecified"
	}
	if s.LoyaltyStage == "" {
		s.LoyaltyStage = "_unspecified"
	}
	if s.MomentumStage == "" {
		s.MomentumStage = "_unspecified"
	}
	s.CeoGroup = computeCeoGroupForLTV(s.ValueTier, s.LifecycleStage, s.JourneyStage, s.LoyaltyStage, s.MomentumStage)

	agg := layer3.DeriveFromNested(m, endMs)
	if agg != nil {
		s.First = agg.First
		s.Repeat = agg.Repeat
		s.Vip = agg.Vip
		s.Inactive = agg.Inactive
		s.Engaged = agg.Engaged
	}
	return s
}

func incMap(m map[string]int64, k string) {
	if k == "" {
		return
	}
	m[k]++
}

func addMapFloat(m map[string]float64, k string, v float64) {
	if k == "" {
		return
	}
	m[k] += v
}

// computeAllPhatSinh tính phát sinh in/out cho toàn bộ cấu trúc metrics (distributions, LTV, layer3).
// Chỉ đếm chuyển đổi RÒNG cuối cùng mỗi khách (start → end kỳ), không đếm mỗi lần chuyển trạng thái trung gian.
// Tránh trùng đếm khi khách có nhiều activity trong kỳ (vd: nhiều cuộc hội thoại → số nhảy vọt).
func computeAllPhatSinh(ctx context.Context, actSvc *crmvc.CrmActivityService, ownerOrgID primitive.ObjectID, startMs, endMs int64) (map[string]interface{}, error) {
	startState, err := actSvc.GetLastSnapshotPerCustomerBeforeEndMs(ctx, ownerOrgID, startMs)
	if err != nil {
		return nil, err
	}

	activities, err := actSvc.GetActivitiesInPeriod(ctx, ownerOrgID, startMs, endMs)
	if err != nil {
		return nil, err
	}

	// Bước 1: Thu thập trạng thái cuối kỳ mỗi khách (activity cuối trong kỳ — đã sort unifiedId, activityAt)
	endState := make(map[string]customerState)
	for _, a := range activities {
		uid := a.UnifiedId
		if uid == "" || a.Metadata == nil {
			continue
		}
		ms, ok := a.Metadata["metricsSnapshot"].(map[string]interface{})
		if !ok || ms == nil {
			continue
		}
		endState[uid] = stateFromMetricsSnapshot(ms, endMs)
	}

	// Bước 2: Chỉ đếm chuyển đổi ròng (start → end) mỗi khách, không đếm từng bước trung gian
	valueIn, valueOut := make(map[string]int64), make(map[string]int64)
	journeyIn, journeyOut := make(map[string]int64), make(map[string]int64)
	lifecycleIn, lifecycleOut := make(map[string]int64), make(map[string]int64)
	channelIn, channelOut := make(map[string]int64), make(map[string]int64)
	loyaltyIn, loyaltyOut := make(map[string]int64), make(map[string]int64)
	momentumIn, momentumOut := make(map[string]int64), make(map[string]int64)
	ceoIn, ceoOut := make(map[string]int64), make(map[string]int64)

	valueLTVIn, valueLTVOut := make(map[string]float64), make(map[string]float64)
	journeyLTVIn, journeyLTVOut := make(map[string]float64), make(map[string]float64)
	lifecycleLTVIn, lifecycleLTVOut := make(map[string]float64), make(map[string]float64)
	channelLTVIn, channelLTVOut := make(map[string]float64), make(map[string]float64)
	loyaltyLTVIn, loyaltyLTVOut := make(map[string]float64), make(map[string]float64)
	momentumLTVIn, momentumLTVOut := make(map[string]float64), make(map[string]float64)
	ceoLTVIn, ceoLTVOut := make(map[string]float64), make(map[string]float64)

	firstPQIn, firstPQOut := make(map[string]int64), make(map[string]int64)
	firstEQIn, firstEQOut := make(map[string]int64), make(map[string]int64)
	firstEngIn, firstEngOut := make(map[string]int64), make(map[string]int64)
	firstRTIn, firstRTOut := make(map[string]int64), make(map[string]int64)
	firstRPIn, firstRPOut := make(map[string]int64), make(map[string]int64)
	repeatRDIn, repeatRDOut := make(map[string]int64), make(map[string]int64)
	repeatRFIn, repeatRFOut := make(map[string]int64), make(map[string]int64)
	repeatSMIn, repeatSMOut := make(map[string]int64), make(map[string]int64)
	repeatPEIn, repeatPEOut := make(map[string]int64), make(map[string]int64)
	repeatEEIn, repeatEEOut := make(map[string]int64), make(map[string]int64)
	repeatUPIn, repeatUPOut := make(map[string]int64), make(map[string]int64)
	vipVDIn, vipVDOut := make(map[string]int64), make(map[string]int64)
	vipSTIn, vipSTOut := make(map[string]int64), make(map[string]int64)
	vipPDIn, vipPDOut := make(map[string]int64), make(map[string]int64)
	vipELIn, vipELOut := make(map[string]int64), make(map[string]int64)
	vipRSIn, vipRSOut := make(map[string]int64), make(map[string]int64)
	inactiveEDIn, inactiveEDOut := make(map[string]int64), make(map[string]int64)
	inactiveRPIn, inactiveRPOut := make(map[string]int64), make(map[string]int64)
	engagedTempIn, engagedTempOut := make(map[string]int64), make(map[string]int64)
	engagedDepthIn, engagedDepthOut := make(map[string]int64), make(map[string]int64)
	engagedSourceIn, engagedSourceOut := make(map[string]int64), make(map[string]int64)

	var totalIn, totalOut, newInPeriod, activeInPeriod int64
	var reactivationValueIn, reactivationValueOut float64

	for uid, to := range endState {
		var from customerState
		hadFrom := false
		if m, ok := startState[uid]; ok && m != nil {
			from = stateFromMetricsSnapshot(m, endMs)
			hadFrom = true
		} else {
			from = customerState{ValueTier: "_new", JourneyStage: "_new", LifecycleStage: "_new", Channel: "_new", LoyaltyStage: "_new", MomentumStage: "_new", CeoGroup: "_new"}
		}

		// Chỉ đếm khi có thay đổi (chuyển đổi ròng)
		changed := from.ValueTier != to.ValueTier || from.JourneyStage != to.JourneyStage || from.LifecycleStage != to.LifecycleStage ||
			from.Channel != to.Channel || from.LoyaltyStage != to.LoyaltyStage || from.MomentumStage != to.MomentumStage

		if !changed && hadFrom {
			// Không đổi classification nhưng vẫn cần đếm activeInPeriod nếu có order trong kỳ
			if to.OrderCount >= 1 && to.LastOrderAt >= startMs && to.LastOrderAt <= endMs {
				activeInPeriod++
			}
			continue
		}

		// Phát sinh distributions
		if from.ValueTier != to.ValueTier {
			if hadFrom && from.ValueTier != "_new" {
				incMap(valueOut, from.ValueTier)
			}
			incMap(valueIn, to.ValueTier)
		}
		if from.JourneyStage != to.JourneyStage {
			if hadFrom && from.JourneyStage != "_new" {
				incMap(journeyOut, from.JourneyStage)
			}
			incMap(journeyIn, to.JourneyStage)
		}
		if from.LifecycleStage != to.LifecycleStage {
			if hadFrom && from.LifecycleStage != "_new" {
				incMap(lifecycleOut, from.LifecycleStage)
			}
			incMap(lifecycleIn, to.LifecycleStage)
		}
		if from.Channel != to.Channel {
			if hadFrom && from.Channel != "_new" {
				incMap(channelOut, from.Channel)
			}
			incMap(channelIn, to.Channel)
		}
		if from.LoyaltyStage != to.LoyaltyStage {
			if hadFrom && from.LoyaltyStage != "_new" {
				incMap(loyaltyOut, from.LoyaltyStage)
			}
			incMap(loyaltyIn, to.LoyaltyStage)
		}
		if from.MomentumStage != to.MomentumStage {
			if hadFrom && from.MomentumStage != "_new" {
				incMap(momentumOut, from.MomentumStage)
			}
			incMap(momentumIn, to.MomentumStage)
		}
		if from.CeoGroup != to.CeoGroup {
			if hadFrom && from.CeoGroup != "_new" {
				incMap(ceoOut, from.CeoGroup)
			}
			incMap(ceoIn, to.CeoGroup)
		}

		// Phát sinh LTV
		spent := to.TotalSpent
		if from.ValueTier != to.ValueTier {
			if hadFrom && from.ValueTier != "_new" {
				addMapFloat(valueLTVOut, from.ValueTier, from.TotalSpent)
			}
			addMapFloat(valueLTVIn, to.ValueTier, spent)
		}
		if from.JourneyStage != to.JourneyStage {
			if hadFrom && from.JourneyStage != "_new" {
				addMapFloat(journeyLTVOut, from.JourneyStage, from.TotalSpent)
			}
			addMapFloat(journeyLTVIn, to.JourneyStage, spent)
		}
		if from.LifecycleStage != to.LifecycleStage {
			if hadFrom && from.LifecycleStage != "_new" {
				addMapFloat(lifecycleLTVOut, from.LifecycleStage, from.TotalSpent)
			}
			addMapFloat(lifecycleLTVIn, to.LifecycleStage, spent)
		}
		if from.Channel != to.Channel {
			if hadFrom && from.Channel != "_new" {
				addMapFloat(channelLTVOut, from.Channel, from.TotalSpent)
			}
			addMapFloat(channelLTVIn, to.Channel, spent)
		}
		if from.LoyaltyStage != to.LoyaltyStage {
			if hadFrom && from.LoyaltyStage != "_new" {
				addMapFloat(loyaltyLTVOut, from.LoyaltyStage, from.TotalSpent)
			}
			addMapFloat(loyaltyLTVIn, to.LoyaltyStage, spent)
		}
		if from.MomentumStage != to.MomentumStage {
			if hadFrom && from.MomentumStage != "_new" {
				addMapFloat(momentumLTVOut, from.MomentumStage, from.TotalSpent)
			}
			addMapFloat(momentumLTVIn, to.MomentumStage, spent)
		}
		if from.CeoGroup != to.CeoGroup {
			if hadFrom && from.CeoGroup != "_new" {
				addMapFloat(ceoLTVOut, from.CeoGroup, from.TotalSpent)
			}
			addMapFloat(ceoLTVIn, to.CeoGroup, spent)
		}

		// Summary phát sinh
		if !hadFrom {
			newInPeriod++
			totalIn++
		} else {
			totalOut++
			totalIn++
		}
		if to.OrderCount >= 1 && to.LastOrderAt >= startMs && to.LastOrderAt <= endMs {
			activeInPeriod++
		}
		if to.ValueTier == "vip" && (to.LifecycleStage == "inactive" || to.LifecycleStage == "dead") {
			reactivationValueIn += spent
		}
		if hadFrom && from.ValueTier == "vip" && (from.LifecycleStage == "inactive" || from.LifecycleStage == "dead") {
			reactivationValueOut += from.TotalSpent
		}

		// Layer 3 phát sinh
		if from.First != nil && to.First != nil {
			if from.First.PurchaseQuality != to.First.PurchaseQuality {
				incMap(firstPQOut, from.First.PurchaseQuality)
				incMap(firstPQIn, to.First.PurchaseQuality)
			}
			if from.First.ExperienceQuality != to.First.ExperienceQuality {
				incMap(firstEQOut, from.First.ExperienceQuality)
				incMap(firstEQIn, to.First.ExperienceQuality)
			}
			if from.First.EngagementAfterPurchase != to.First.EngagementAfterPurchase {
				incMap(firstEngOut, from.First.EngagementAfterPurchase)
				incMap(firstEngIn, to.First.EngagementAfterPurchase)
			}
			if from.First.ReorderTiming != to.First.ReorderTiming {
				incMap(firstRTOut, from.First.ReorderTiming)
				incMap(firstRTIn, to.First.ReorderTiming)
			}
			if from.First.RepeatProbability != to.First.RepeatProbability {
				incMap(firstRPOut, from.First.RepeatProbability)
				incMap(firstRPIn, to.First.RepeatProbability)
			}
		} else if to.First != nil {
			incMap(firstPQIn, to.First.PurchaseQuality)
			incMap(firstEQIn, to.First.ExperienceQuality)
			incMap(firstEngIn, to.First.EngagementAfterPurchase)
			incMap(firstRTIn, to.First.ReorderTiming)
			incMap(firstRPIn, to.First.RepeatProbability)
		} else if from.First != nil {
			incMap(firstPQOut, from.First.PurchaseQuality)
			incMap(firstEQOut, from.First.ExperienceQuality)
			incMap(firstEngOut, from.First.EngagementAfterPurchase)
			incMap(firstRTOut, from.First.ReorderTiming)
			incMap(firstRPOut, from.First.RepeatProbability)
		}

		if from.Repeat != nil && to.Repeat != nil {
			if from.Repeat.RepeatDepth != to.Repeat.RepeatDepth {
				incMap(repeatRDOut, from.Repeat.RepeatDepth)
				incMap(repeatRDIn, to.Repeat.RepeatDepth)
			}
			if from.Repeat.RepeatFrequency != to.Repeat.RepeatFrequency {
				incMap(repeatRFOut, from.Repeat.RepeatFrequency)
				incMap(repeatRFIn, to.Repeat.RepeatFrequency)
			}
			if from.Repeat.SpendMomentum != to.Repeat.SpendMomentum {
				incMap(repeatSMOut, from.Repeat.SpendMomentum)
				incMap(repeatSMIn, to.Repeat.SpendMomentum)
			}
			if from.Repeat.ProductExpansion != to.Repeat.ProductExpansion {
				incMap(repeatPEOut, from.Repeat.ProductExpansion)
				incMap(repeatPEIn, to.Repeat.ProductExpansion)
			}
			if from.Repeat.EmotionalEngagement != to.Repeat.EmotionalEngagement {
				incMap(repeatEEOut, from.Repeat.EmotionalEngagement)
				incMap(repeatEEIn, to.Repeat.EmotionalEngagement)
			}
			if from.Repeat.UpgradePotential != to.Repeat.UpgradePotential {
				incMap(repeatUPOut, from.Repeat.UpgradePotential)
				incMap(repeatUPIn, to.Repeat.UpgradePotential)
			}
		} else if to.Repeat != nil {
			incMap(repeatRDIn, to.Repeat.RepeatDepth)
			incMap(repeatRFIn, to.Repeat.RepeatFrequency)
			incMap(repeatSMIn, to.Repeat.SpendMomentum)
			incMap(repeatPEIn, to.Repeat.ProductExpansion)
			incMap(repeatEEIn, to.Repeat.EmotionalEngagement)
			incMap(repeatUPIn, to.Repeat.UpgradePotential)
		} else if from.Repeat != nil {
			incMap(repeatRDOut, from.Repeat.RepeatDepth)
			incMap(repeatRFOut, from.Repeat.RepeatFrequency)
			incMap(repeatSMOut, from.Repeat.SpendMomentum)
			incMap(repeatPEOut, from.Repeat.ProductExpansion)
			incMap(repeatEEOut, from.Repeat.EmotionalEngagement)
			incMap(repeatUPOut, from.Repeat.UpgradePotential)
		}

		if from.Vip != nil && to.Vip != nil {
			if from.Vip.VipDepth != to.Vip.VipDepth {
				incMap(vipVDOut, from.Vip.VipDepth)
				incMap(vipVDIn, to.Vip.VipDepth)
			}
			if from.Vip.SpendTrend != to.Vip.SpendTrend {
				incMap(vipSTOut, from.Vip.SpendTrend)
				incMap(vipSTIn, to.Vip.SpendTrend)
			}
			if from.Vip.ProductDiversity != to.Vip.ProductDiversity {
				incMap(vipPDOut, from.Vip.ProductDiversity)
				incMap(vipPDIn, to.Vip.ProductDiversity)
			}
			if from.Vip.EngagementLevel != to.Vip.EngagementLevel {
				incMap(vipELOut, from.Vip.EngagementLevel)
				incMap(vipELIn, to.Vip.EngagementLevel)
			}
			if from.Vip.RiskScore != to.Vip.RiskScore {
				incMap(vipRSOut, from.Vip.RiskScore)
				incMap(vipRSIn, to.Vip.RiskScore)
			}
		} else if to.Vip != nil {
			incMap(vipVDIn, to.Vip.VipDepth)
			incMap(vipSTIn, to.Vip.SpendTrend)
			incMap(vipPDIn, to.Vip.ProductDiversity)
			incMap(vipELIn, to.Vip.EngagementLevel)
			incMap(vipRSIn, to.Vip.RiskScore)
		} else if from.Vip != nil {
			incMap(vipVDOut, from.Vip.VipDepth)
			incMap(vipSTOut, from.Vip.SpendTrend)
			incMap(vipPDOut, from.Vip.ProductDiversity)
			incMap(vipELOut, from.Vip.EngagementLevel)
			incMap(vipRSOut, from.Vip.RiskScore)
		}

		if from.Inactive != nil && to.Inactive != nil {
			if from.Inactive.EngagementDrop != to.Inactive.EngagementDrop {
				incMap(inactiveEDOut, from.Inactive.EngagementDrop)
				incMap(inactiveEDIn, to.Inactive.EngagementDrop)
			}
			if from.Inactive.ReactivationPotential != to.Inactive.ReactivationPotential {
				incMap(inactiveRPOut, from.Inactive.ReactivationPotential)
				incMap(inactiveRPIn, to.Inactive.ReactivationPotential)
			}
		} else if to.Inactive != nil {
			incMap(inactiveEDIn, to.Inactive.EngagementDrop)
			incMap(inactiveRPIn, to.Inactive.ReactivationPotential)
		} else if from.Inactive != nil {
			incMap(inactiveEDOut, from.Inactive.EngagementDrop)
			incMap(inactiveRPOut, from.Inactive.ReactivationPotential)
		}

		if from.Engaged != nil && to.Engaged != nil {
			if from.Engaged.ConversationTemperature != to.Engaged.ConversationTemperature {
				incMap(engagedTempOut, from.Engaged.ConversationTemperature)
				incMap(engagedTempIn, to.Engaged.ConversationTemperature)
			}
			if from.Engaged.EngagementDepth != to.Engaged.EngagementDepth {
				incMap(engagedDepthOut, from.Engaged.EngagementDepth)
				incMap(engagedDepthIn, to.Engaged.EngagementDepth)
			}
			if from.Engaged.SourceType != to.Engaged.SourceType {
				incMap(engagedSourceOut, from.Engaged.SourceType)
				incMap(engagedSourceIn, to.Engaged.SourceType)
			}
		} else if to.Engaged != nil {
			incMap(engagedTempIn, to.Engaged.ConversationTemperature)
			incMap(engagedDepthIn, to.Engaged.EngagementDepth)
			incMap(engagedSourceIn, to.Engaged.SourceType)
		} else if from.Engaged != nil {
			incMap(engagedTempOut, from.Engaged.ConversationTemperature)
			incMap(engagedDepthOut, from.Engaged.EngagementDepth)
			incMap(engagedSourceOut, from.Engaged.SourceType)
		}
	}

	return buildPhatSinhMetrics(
		valueIn, valueOut, journeyIn, journeyOut, lifecycleIn, lifecycleOut,
		channelIn, channelOut, loyaltyIn, loyaltyOut, momentumIn, momentumOut,
		ceoIn, ceoOut,
		valueLTVIn, valueLTVOut, journeyLTVIn, journeyLTVOut, lifecycleLTVIn, lifecycleLTVOut,
		channelLTVIn, channelLTVOut, loyaltyLTVIn, loyaltyLTVOut, momentumLTVIn, momentumLTVOut,
		ceoLTVIn, ceoLTVOut,
		totalIn, totalOut, newInPeriod, activeInPeriod, reactivationValueIn, reactivationValueOut,
		firstPQIn, firstPQOut, firstEQIn, firstEQOut, firstEngIn, firstEngOut, firstRTIn, firstRTOut, firstRPIn, firstRPOut,
		repeatRDIn, repeatRDOut, repeatRFIn, repeatRFOut, repeatSMIn, repeatSMOut, repeatPEIn, repeatPEOut, repeatEEIn, repeatEEOut, repeatUPIn, repeatUPOut,
		vipVDIn, vipVDOut, vipSTIn, vipSTOut, vipPDIn, vipPDOut, vipELIn, vipELOut, vipRSIn, vipRSOut,
		inactiveEDIn, inactiveEDOut, inactiveRPIn, inactiveRPOut,
		engagedTempIn, engagedTempOut, engagedDepthIn, engagedDepthOut, engagedSourceIn, engagedSourceOut,
	), nil
}

// buildPhatSinhMetrics xây cấu trúc metrics phát sinh đầy đủ — giống metricsSnapshot trong crm_activity_history (raw, layer1, layer2, layer3).
// Mỗi layer có in/out cho phát sinh; số cuối kỳ lấy từ API GetPeriodEndBalance hoặc realtime.
func buildPhatSinhMetrics(
	valueIn, valueOut, journeyIn, journeyOut, lifecycleIn, lifecycleOut map[string]int64,
	channelIn, channelOut, loyaltyIn, loyaltyOut, momentumIn, momentumOut map[string]int64,
	ceoIn, ceoOut map[string]int64,
	valueLTVIn, valueLTVOut, journeyLTVIn, journeyLTVOut, lifecycleLTVIn, lifecycleLTVOut map[string]float64,
	channelLTVIn, channelLTVOut, loyaltyLTVIn, loyaltyLTVOut, momentumLTVIn, momentumLTVOut map[string]float64,
	ceoLTVIn, ceoLTVOut map[string]float64,
	totalIn, totalOut, newInPeriod, activeInPeriod int64, reactivationValueIn, reactivationValueOut float64,
	firstPQIn, firstPQOut, firstEQIn, firstEQOut, firstEngIn, firstEngOut, firstRTIn, firstRTOut, firstRPIn, firstRPOut map[string]int64,
	repeatRDIn, repeatRDOut, repeatRFIn, repeatRFOut, repeatSMIn, repeatSMOut, repeatPEIn, repeatPEOut, repeatEEIn, repeatEEOut, repeatUPIn, repeatUPOut map[string]int64,
	vipVDIn, vipVDOut, vipSTIn, vipSTOut, vipPDIn, vipPDOut, vipELIn, vipELOut, vipRSIn, vipRSOut map[string]int64,
	inactiveEDIn, inactiveEDOut, inactiveRPIn, inactiveRPOut map[string]int64,
	engagedTempIn, engagedTempOut, engagedDepthIn, engagedDepthOut, engagedSourceIn, engagedSourceOut map[string]int64,
) map[string]interface{} {
	sumFloat64Map := func(m map[string]float64) float64 {
		var s float64
		for _, v := range m {
			s += v
		}
		return s
	}

	totalLTVIn := sumFloat64Map(valueLTVIn)
	totalLTVOut := sumFloat64Map(valueLTVOut)

	// inOutMap tạo { in: x, out: y } từ 2 giá trị — in/out trong cùng nhóm metric.
	inOutInt64 := func(in, out int64) map[string]interface{} {
		m := map[string]interface{}{"in": in}
		if out != 0 {
			m["out"] = out
		}
		return m
	}
	inOutFloat64 := func(in, out float64) map[string]interface{} {
		m := map[string]interface{}{"in": in}
		if out != 0 {
			m["out"] = out
		}
		return m
	}
	inOutMapInt64 := func(inM, outM map[string]int64) map[string]interface{} {
		allKeys := make(map[string]bool)
		for k := range inM {
			allKeys[k] = true
		}
		for k := range outM {
			allKeys[k] = true
		}
		m := make(map[string]interface{})
		for k := range allKeys {
			io := map[string]interface{}{"in": inM[k]}
			if outM[k] != 0 {
				io["out"] = outM[k]
			}
			m[k] = io
		}
		return m
	}
	inOutMapFloat64 := func(inM, outM map[string]float64) map[string]interface{} {
		allKeys := make(map[string]bool)
		for k := range inM {
			allKeys[k] = true
		}
		for k := range outM {
			allKeys[k] = true
		}
		m := make(map[string]interface{})
		for k := range allKeys {
			io := map[string]interface{}{"in": inM[k]}
			if outM[k] != 0 {
				io["out"] = outM[k]
			}
			m[k] = io
		}
		return m
	}
	// raw: mỗi metric có in/out trong cùng nhóm.
	raw := map[string]interface{}{
		"totalCustomers":       inOutInt64(totalIn, totalOut),
		"newCustomersInPeriod": map[string]interface{}{"in": newInPeriod},
		"activeInPeriod":       map[string]interface{}{"in": activeInPeriod},
		"reactivationValue":    inOutFloat64(reactivationValueIn, reactivationValueOut),
		"totalLTV":             inOutFloat64(totalLTVIn, totalLTVOut),
	}

	// layer1: journeyStage — mỗi stage có in/out trong cùng nhóm.
	layer1 := map[string]interface{}{
		"journeyStage": inOutMapInt64(journeyIn, journeyOut),
	}

	// layer2: mỗi dimension (valueTier, ceoGroup, ...) — mỗi nhóm có in/out trong cùng nhóm.
	layer2 := map[string]interface{}{
		"valueTier":         inOutMapInt64(valueIn, valueOut),
		"lifecycleStage":    inOutMapInt64(lifecycleIn, lifecycleOut),
		"channel":           inOutMapInt64(channelIn, channelOut),
		"loyaltyStage":      inOutMapInt64(loyaltyIn, loyaltyOut),
		"momentumStage":     inOutMapInt64(momentumIn, momentumOut),
		"ceoGroup":          inOutMapInt64(ceoIn, ceoOut),
		"valueTierLTV":      inOutMapFloat64(valueLTVIn, valueLTVOut),
		"lifecycleStageLTV": inOutMapFloat64(lifecycleLTVIn, lifecycleLTVOut),
		"channelLTV":        inOutMapFloat64(channelLTVIn, channelLTVOut),
		"loyaltyStageLTV":   inOutMapFloat64(loyaltyLTVIn, loyaltyLTVOut),
		"momentumStageLTV":  inOutMapFloat64(momentumLTVIn, momentumLTVOut),
		"ceoGroupLTV":       inOutMapFloat64(ceoLTVIn, ceoLTVOut),
	}

	// layer3: mỗi nhóm (first, repeat, vip, ...) — mỗi dimension có in/out trong cùng nhóm.
	layer3 := map[string]interface{}{
		"first": map[string]interface{}{
			"purchaseQuality":        inOutMapInt64(firstPQIn, firstPQOut),
			"experienceQuality":      inOutMapInt64(firstEQIn, firstEQOut),
			"engagementAfterPurchase": inOutMapInt64(firstEngIn, firstEngOut),
			"reorderTiming":          inOutMapInt64(firstRTIn, firstRTOut),
			"repeatProbability":      inOutMapInt64(firstRPIn, firstRPOut),
		},
		"repeat": map[string]interface{}{
			"repeatDepth":         inOutMapInt64(repeatRDIn, repeatRDOut),
			"repeatFrequency":     inOutMapInt64(repeatRFIn, repeatRFOut),
			"spendMomentum":       inOutMapInt64(repeatSMIn, repeatSMOut),
			"productExpansion":    inOutMapInt64(repeatPEIn, repeatPEOut),
			"emotionalEngagement": inOutMapInt64(repeatEEIn, repeatEEOut),
			"upgradePotential":    inOutMapInt64(repeatUPIn, repeatUPOut),
		},
		"vip": map[string]interface{}{
			"vipDepth":         inOutMapInt64(vipVDIn, vipVDOut),
			"spendTrend":       inOutMapInt64(vipSTIn, vipSTOut),
			"productDiversity": inOutMapInt64(vipPDIn, vipPDOut),
			"engagementLevel":  inOutMapInt64(vipELIn, vipELOut),
			"riskScore":        inOutMapInt64(vipRSIn, vipRSOut),
		},
		"inactive": map[string]interface{}{
			"engagementDrop":        inOutMapInt64(inactiveEDIn, inactiveEDOut),
			"reactivationPotential": inOutMapInt64(inactiveRPIn, inactiveRPOut),
		},
		"engaged": map[string]interface{}{
			"conversationTemperature": inOutMapInt64(engagedTempIn, engagedTempOut),
			"engagementDepth":         inOutMapInt64(engagedDepthIn, engagedDepthOut),
			"sourceType":              inOutMapInt64(engagedSourceIn, engagedSourceOut),
		},
	}

	return map[string]interface{}{
		"raw":    raw,
		"layer1": layer1,
		"layer2": layer2,
		"layer3": layer3,
	}
}

// buildPeriodEndBalance xây cấu trúc số dư cuối kỳ (raw, layer1, layer2, layer3) — giống metricsSnapshot.
// snapshotMap: metricsSnapshot cuối cùng của mỗi khách trước endMs.
// startMs: đầu kỳ (để tính activeInPeriod); 0 = bỏ qua.
func buildPeriodEndBalance(snapshotMap map[string]map[string]interface{}, endMs, startMs int64) map[string]interface{} {
	valueDist := make(map[string]int64)
	journeyDist := make(map[string]int64)
	lifecycleDist := make(map[string]int64)
	channelDist := make(map[string]int64)
	loyaltyDist := make(map[string]int64)
	momentumDist := make(map[string]int64)
	ceoDist := make(map[string]int64)

	valueLTV := make(map[string]float64)
	journeyLTV := make(map[string]float64)
	lifecycleLTV := make(map[string]float64)
	channelLTV := make(map[string]float64)
	loyaltyLTV := make(map[string]float64)
	momentumLTV := make(map[string]float64)
	ceoLTV := make(map[string]float64)

	firstPQ, firstEQ, firstEng, firstRT, firstRP := make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64)
	repeatRD, repeatRF, repeatSM, repeatPE, repeatEE, repeatUP := make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64)
	vipVD, vipST, vipPD, vipEL, vipRS := make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64), make(map[string]int64)
	inactiveED, inactiveRP := make(map[string]int64), make(map[string]int64)
	engagedTemp, engagedDepth, engagedSource := make(map[string]int64), make(map[string]int64), make(map[string]int64)

	var totalCustomers int64
	var totalLTV float64
	var activeInPeriod int64
	var reactivationValue float64

	for _, m := range snapshotMap {
		s := stateFromMetricsSnapshot(m, endMs)
		totalCustomers++
		totalLTV += s.TotalSpent

		if startMs > 0 && s.LastOrderAt >= startMs && s.LastOrderAt <= endMs {
			activeInPeriod++
		}
		if s.ValueTier == "vip" && (s.LifecycleStage == "inactive" || s.LifecycleStage == "dead") {
			reactivationValue += s.TotalSpent
		}

		incMap(valueDist, s.ValueTier)
		incMap(journeyDist, s.JourneyStage)
		incMap(lifecycleDist, s.LifecycleStage)
		incMap(channelDist, s.Channel)
		incMap(loyaltyDist, s.LoyaltyStage)
		incMap(momentumDist, s.MomentumStage)
		incMap(ceoDist, s.CeoGroup)

		addMapFloat(valueLTV, s.ValueTier, s.TotalSpent)
		addMapFloat(journeyLTV, s.JourneyStage, s.TotalSpent)
		addMapFloat(lifecycleLTV, s.LifecycleStage, s.TotalSpent)
		addMapFloat(channelLTV, s.Channel, s.TotalSpent)
		addMapFloat(loyaltyLTV, s.LoyaltyStage, s.TotalSpent)
		addMapFloat(momentumLTV, s.MomentumStage, s.TotalSpent)
		addMapFloat(ceoLTV, s.CeoGroup, s.TotalSpent)

		if s.First != nil {
			incMap(firstPQ, s.First.PurchaseQuality)
			incMap(firstEQ, s.First.ExperienceQuality)
			incMap(firstEng, s.First.EngagementAfterPurchase)
			incMap(firstRT, s.First.ReorderTiming)
			incMap(firstRP, s.First.RepeatProbability)
		}
		if s.Repeat != nil {
			incMap(repeatRD, s.Repeat.RepeatDepth)
			incMap(repeatRF, s.Repeat.RepeatFrequency)
			incMap(repeatSM, s.Repeat.SpendMomentum)
			incMap(repeatPE, s.Repeat.ProductExpansion)
			incMap(repeatEE, s.Repeat.EmotionalEngagement)
			incMap(repeatUP, s.Repeat.UpgradePotential)
		}
		if s.Vip != nil {
			incMap(vipVD, s.Vip.VipDepth)
			incMap(vipST, s.Vip.SpendTrend)
			incMap(vipPD, s.Vip.ProductDiversity)
			incMap(vipEL, s.Vip.EngagementLevel)
			incMap(vipRS, s.Vip.RiskScore)
		}
		if s.Inactive != nil {
			incMap(inactiveED, s.Inactive.EngagementDrop)
			incMap(inactiveRP, s.Inactive.ReactivationPotential)
		}
		if s.Engaged != nil {
			incMap(engagedTemp, s.Engaged.ConversationTemperature)
			incMap(engagedDepth, s.Engaged.EngagementDepth)
			incMap(engagedSource, s.Engaged.SourceType)
		}
	}

	toInt64Map := func(m map[string]int64) map[string]interface{} {
		out := make(map[string]interface{})
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	toFloat64Map := func(m map[string]float64) map[string]interface{} {
		out := make(map[string]interface{})
		for k, v := range m {
			out[k] = v
		}
		return out
	}

	raw := map[string]interface{}{
		"totalCustomers":       totalCustomers,
		"activeInPeriod":        activeInPeriod,
		"reactivationValue":    reactivationValue,
		"totalLTV":             totalLTV,
	}

	layer1 := map[string]interface{}{
		"journeyStage": toInt64Map(journeyDist),
	}

	layer2 := map[string]interface{}{
		"valueTier":         toInt64Map(valueDist),
		"lifecycleStage":    toInt64Map(lifecycleDist),
		"channel":           toInt64Map(channelDist),
		"loyaltyStage":      toInt64Map(loyaltyDist),
		"momentumStage":     toInt64Map(momentumDist),
		"ceoGroup":          toInt64Map(ceoDist),
		"valueTierLTV":      toFloat64Map(valueLTV),
		"lifecycleStageLTV": toFloat64Map(lifecycleLTV),
		"channelLTV":        toFloat64Map(channelLTV),
		"loyaltyStageLTV":   toFloat64Map(loyaltyLTV),
		"momentumStageLTV":  toFloat64Map(momentumLTV),
		"ceoGroupLTV":       toFloat64Map(ceoLTV),
	}

	layer3 := map[string]interface{}{
		"first": map[string]interface{}{
			"purchaseQuality": firstPQ, "experienceQuality": firstEQ, "engagementAfterPurchase": firstEng,
			"reorderTiming": firstRT, "repeatProbability": firstRP,
		},
		"repeat": map[string]interface{}{
			"repeatDepth": repeatRD, "repeatFrequency": repeatRF, "spendMomentum": repeatSM,
			"productExpansion": repeatPE, "emotionalEngagement": repeatEE, "upgradePotential": repeatUP,
		},
		"vip": map[string]interface{}{
			"vipDepth": vipVD, "spendTrend": vipST, "productDiversity": vipPD,
			"engagementLevel": vipEL, "riskScore": vipRS,
		},
		"inactive": map[string]interface{}{
			"engagementDrop": inactiveED, "reactivationPotential": inactiveRP,
		},
		"engaged": map[string]interface{}{
			"conversationTemperature": engagedTemp, "engagementDepth": engagedDepth, "sourceType": engagedSource,
		},
	}

	return map[string]interface{}{
		"raw":    raw,
		"layer1": layer1,
		"layer2": layer2,
		"layer3": layer3,
	}
}
