package decisionlive

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

// Giá trị feedSourceCategory — ổn định cho chip lọc UI (khác với sourceKind thô: queue / unknown).
const (
	FeedSourceConversation = "conversation" // Hội thoại
	FeedSourceOrder        = "order"        // Đơn hàng
	FeedSourceDecision     = "decision"     // Thực thi / đề xuất quyết định (execute, propose)
	FeedSourceIntel        = "intel"        // Chuẩn bị ngữ cảnh: CIX, order intel, customer context…
	FeedSourceAds          = "ads"          // Ads (context, cấu hình ads)
	FeedSourceMetaSync     = "meta_sync"    // Đồng bộ Meta (campaign, ad, insight…)
	FeedSourcePosSync      = "pos_sync"     // Đồng bộ POS / kho / sản phẩm
	FeedSourceCrm          = "crm"          // CRM: khách, ghi chú, intelligence CRM
	FeedSourceWebhook      = "webhook"      // Webhook thô
	FeedSourceQueue        = "queue"        // Hàng đợi — event chưa gán nhóm
	FeedSourceOther        = "other"        // Khác
)

var feedSourceLabelVi = map[string]string{
	FeedSourceConversation: "Hội thoại",
	FeedSourceOrder:        "Đơn hàng",
	FeedSourceDecision:     "Quyết định",
	FeedSourceIntel:        "Chuẩn bị intel",
	FeedSourceAds:          "Ads",
	FeedSourceMetaSync:     "Đồng bộ Meta",
	FeedSourcePosSync:      "Đồng bộ POS",
	FeedSourceCrm:          "CRM",
	FeedSourceWebhook:      "Webhook",
	FeedSourceQueue:        "Hàng đợi",
	FeedSourceOther:        "Khác",
}

func labelFeedSource(cat string) string {
	if s, ok := feedSourceLabelVi[cat]; ok {
		return s
	}
	return feedSourceLabelVi[FeedSourceOther]
}

// applySourceKindForFeedCategory ghi đè sourceKind bằng nhóm nguồn hiển thị.
// Nhiều client chỉ map conversation | order | else → "Khác"; giữ queue → vẫn "Khác" nên với job queue đã suy ra pos_sync / meta_sync… thì phải gửi đúng giá trị đó trên sourceKind.
func applySourceKindForFeedCategory(ev *DecisionLiveEvent, cat string) {
	if ev == nil {
		return
	}
	switch cat {
	case FeedSourceQueue:
		ev.SourceKind = SourceQueue
	case FeedSourceOther:
		ev.SourceKind = SourceUnknown
	default:
		ev.SourceKind = cat
	}
}

// eventTypeFromRefs lấy eventType từ refs (queue / ingest).
func eventTypeFromRefs(ev *DecisionLiveEvent) string {
	if ev == nil || ev.Refs == nil {
		return ""
	}
	et := strings.TrimSpace(ev.Refs["eventType"])
	if et == "" {
		et = strings.TrimSpace(ev.Refs["event_type"])
	}
	return et
}

// isQueueConsumerFeedPhase — phase milestone consumer queue (không có refs.eventType trên một số frame cũ).
func isQueueConsumerFeedPhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case PhaseQueueProcessing, PhaseQueueDone, PhaseQueueError, PhaseDatachangedEffects:
		return true
	default:
		return false
	}
}

// enrichLiveEventFeedSource — Publish bước 2: điền feedSourceCategory / feedSourceLabelVi (chip “Nguồn” trên UI).
func enrichLiveEventFeedSource(ev *DecisionLiveEvent) {
	if ev == nil || strings.TrimSpace(ev.FeedSourceCategory) != "" {
		return
	}
	sk := strings.TrimSpace(ev.SourceKind)
	// JSON omitempty hoặc payload cũ — coi như unknown để suy từ phase / refs.
	if sk == "" {
		sk = SourceUnknown
	}
	switch sk {
	case SourceConversation:
		ev.FeedSourceCategory = FeedSourceConversation
		ev.FeedSourceLabelVi = labelFeedSource(FeedSourceConversation)
		// sourceKind đã là conversation
		return
	case SourceOrder:
		ev.FeedSourceCategory = FeedSourceOrder
		ev.FeedSourceLabelVi = labelFeedSource(FeedSourceOrder)
		return
	case SourceQueue:
		et := eventTypeFromRefs(ev)
		if et == "" && strings.TrimSpace(ev.SourceTitle) != "" {
			et = strings.TrimSpace(ev.SourceTitle)
		}
		cat := classifyEventTypeFeedSource(et)
		ev.FeedSourceCategory = cat
		ev.FeedSourceLabelVi = labelFeedSource(cat)
		applySourceKindForFeedCategory(ev, cat)
		return
	default:
		if isOrchestrationPipelinePhase(ev.Phase) {
			ev.FeedSourceCategory = FeedSourceIntel
			ev.FeedSourceLabelVi = labelFeedSource(FeedSourceIntel)
			applySourceKindForFeedCategory(ev, FeedSourceIntel)
			return
		}
		if isIntelDomainComputePhase(ev.Phase) {
			ev.FeedSourceCategory = FeedSourceIntel
			ev.FeedSourceLabelVi = labelFeedSource(FeedSourceIntel)
			applySourceKindForFeedCategory(ev, FeedSourceIntel)
			return
		}
		et := eventTypeFromRefs(ev)
		if et == "aidecision.execute_requested" || et == "executor.propose_requested" || et == "ads.propose_requested" {
			ev.FeedSourceCategory = FeedSourceDecision
			ev.FeedSourceLabelVi = labelFeedSource(FeedSourceDecision)
			applySourceKindForFeedCategory(ev, FeedSourceDecision)
			return
		}
		if et != "" {
			cat := classifyEventTypeFeedSource(et)
			ev.FeedSourceCategory = cat
			ev.FeedSourceLabelVi = labelFeedSource(cat)
			applySourceKindForFeedCategory(ev, cat)
			return
		}
		// ExecuteWithCase: hầu hết bước không gắn refs.eventType — dùng phase (đồng bộ logic opsTier).
		if isExecutePipelinePhase(ev.Phase) {
			ev.FeedSourceCategory = FeedSourceDecision
			ev.FeedSourceLabelVi = labelFeedSource(FeedSourceDecision)
			applySourceKindForFeedCategory(ev, FeedSourceDecision)
			return
		}
		if isQueueConsumerFeedPhase(ev.Phase) {
			ev.FeedSourceCategory = FeedSourceQueue
			ev.FeedSourceLabelVi = labelFeedSource(FeedSourceQueue)
			applySourceKindForFeedCategory(ev, FeedSourceQueue)
			return
		}
		ev.FeedSourceCategory = FeedSourceOther
		ev.FeedSourceLabelVi = labelFeedSource(FeedSourceOther)
		applySourceKindForFeedCategory(ev, FeedSourceOther)
	}
}

// classifyEventTypeFeedSource map event_type queue → nhóm nguồn feed (registry datachanged + consumer).
func classifyEventTypeFeedSource(et string) string {
	s := strings.TrimSpace(et)
	if s == "" {
		return FeedSourceQueue
	}
	switch s {
	case eventtypes.AIDecisionExecuteRequested, eventtypes.ExecutorProposeRequested, eventtypes.AdsProposeRequested:
		return FeedSourceDecision
	}
	// CRM & khách (crm.* API + crm_* datachanged + fb_customer.*)
	if strings.HasPrefix(s, eventtypes.PrefixCrmDot) || strings.HasPrefix(s, eventtypes.PrefixCrmUnderscore) || strings.HasPrefix(s, "fb_customer.") {
		return FeedSourceCrm
	}
	if strings.HasPrefix(s, "webhook_log.") {
		return FeedSourceWebhook
	}
	if strings.HasPrefix(s, "meta_") {
		return FeedSourceMetaSync
	}
	if strings.HasPrefix(s, "pos_") {
		return FeedSourcePosSync
	}
	if strings.HasPrefix(s, "fb_page.") || strings.HasPrefix(s, "fb_post.") || strings.HasPrefix(s, "fb_message_item.") {
		return FeedSourceMetaSync
	}
	// Ads intelligence (batch) → intel; campaign_intel_* / *_{domain}_intel_recomputed sau worker → intel
	if strings.HasPrefix(s, eventtypes.PrefixAdsIntelligence) || strings.HasPrefix(s, eventtypes.PrefixCampaignIntel) ||
		strings.HasPrefix(s, eventtypes.PrefixCrmIntelUnderscore) || strings.HasPrefix(s, eventtypes.PrefixOrderIntelUnderscore) || strings.HasPrefix(s, eventtypes.PrefixCixIntelUnderscore) {
		return FeedSourceIntel
	}
	if strings.HasPrefix(s, eventtypes.PrefixAdsContext) || s == eventtypes.AdsUpdated {
		return FeedSourceAds
	}
	if strings.HasPrefix(s, "ads.") {
		return FeedSourceAds
	}
	// Intel pipeline (CIX, order intel, customer context, commerce…)
	if strings.HasPrefix(s, eventtypes.PrefixCixDot) ||
		strings.HasPrefix(s, eventtypes.PrefixOrderIntelligenceLegacy) || strings.HasPrefix(s, eventtypes.PrefixOrderRecompute) ||
		strings.HasPrefix(s, eventtypes.PrefixCustomerContext) ||
		s == eventtypes.OrderIntelRecomputed ||
		s == eventtypes.ConversationMessageInserted || s == eventtypes.MessageBatchReady {
		return FeedSourceIntel
	}
	// Hội thoại / tin nhắn datachanged (inserted/updated)
	if strings.HasPrefix(s, eventtypes.PrefixConversation) || strings.HasPrefix(s, eventtypes.PrefixMessage) {
		return FeedSourceConversation
	}
	// Đơn datachanged
	if strings.HasPrefix(s, eventtypes.PrefixOrder) {
		return FeedSourceOrder
	}
	return FeedSourceQueue
}
