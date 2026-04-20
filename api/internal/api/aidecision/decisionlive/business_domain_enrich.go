// business_domain_enrich — Gắn mã module đang xử lý mốc timeline (businessDomain) cho frontend.
//
// Nguyên tắc: businessDomain = **module sở hữu queue/worker đang chạy bước đó** — không phải chủ thể nghiệp vụ trong payload (vd. pos_product.updated vẫn do consumer AID trên decision_events_queue xử lý → aidecision).
// Worker intel/merge **riêng** của miền (PublishIntelDomainMilestone, phase intel_domain_*) → crm | order | cix | ads theo refs.intelDomain.
//
// Tham chiếu: docs/flows/bang-pha-buoc-event-e2e.md §1.2 (mã miền / module).
package decisionlive

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
)

// Mã businessDomain — module backend (package/router) gắn với queue/worker xử lý mốc đó.
const (
	BusinessDomainCIO          = "cio"
	BusinessDomainPC           = "pc"
	BusinessDomainFB           = "fb"
	BusinessDomainWebhook      = "webhook"
	BusinessDomainMeta         = "meta"
	BusinessDomainAds          = "ads"
	BusinessDomainCRM          = "crm"
	BusinessDomainOrder        = "order"
	BusinessDomainConversation = "conversation"
	BusinessDomainCIX          = "cix"
	BusinessDomainReport       = "report"
	BusinessDomainNotification = "notification"
	BusinessDomainAIDecision   = "aidecision"
	BusinessDomainExecutor     = "executor"
	BusinessDomainLearning     = "learning"
	BusinessDomainUnknown      = "unknown"
)

// enrichLiveBusinessDomain — Publish: điền businessDomain nếu trống (sau enrichLiveEventFeedSource), luôn gắn businessDomainLabelVi cho cột swimlane module.
func enrichLiveBusinessDomain(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	if strings.TrimSpace(ev.BusinessDomain) == "" {
		if ev.Refs != nil {
			if c := normalizeBusinessDomainCode(ev.Refs["businessDomain"]); c != "" {
				ev.BusinessDomain = c
			}
		}
		if strings.TrimSpace(ev.BusinessDomain) == "" {
			ev.BusinessDomain = resolveBusinessDomain(ev)
		}
	}
	applyBusinessDomainLabelVi(ev)
}

// applyBusinessDomainLabelVi — Nhãn hiển thị cột «module xử lý»; tách hẳn chip «Nguồn» (feedSourceLabelVi có thể là «Khác»).
func applyBusinessDomainLabelVi(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	code := strings.TrimSpace(ev.BusinessDomain)
	if code == "" {
		ev.BusinessDomainLabelVi = eventtypes.ResolveLiveBusinessDomainLabelVi("")
		return
	}
	ev.BusinessDomainLabelVi = businessDomainDisplayLabelVi(code)
}

// businessDomainDisplayLabelVi — «Tên đọc được (mã kỹ thuật)» để UI không phụ thuộc map cứng phía client.
func businessDomainDisplayLabelVi(code string) string {
	return eventtypes.ResolveLiveBusinessDomainLabelVi(code)
}

func normalizeBusinessDomainCode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case BusinessDomainCIO, BusinessDomainPC, BusinessDomainFB, BusinessDomainWebhook,
		BusinessDomainMeta, BusinessDomainAds, BusinessDomainCRM, BusinessDomainOrder,
		BusinessDomainConversation, BusinessDomainCIX, BusinessDomainReport, BusinessDomainNotification,
		BusinessDomainAIDecision, BusinessDomainExecutor, BusinessDomainLearning, BusinessDomainUnknown:
		return s
	default:
		return ""
	}
}

func resolveBusinessDomain(ev *DecisionLiveEvent) string {
	ph := strings.TrimSpace(ev.Phase)

	// Worker miền: queue/job nặng ngoài consumer AID (intel_compute, merge pending, …).
	if isIntelDomainComputePhase(ph) {
		if ev.Refs != nil {
			if d := businessDomainFromIntelDomainRef(ev.Refs["intelDomain"]); d != "" {
				return d
			}
		}
		return BusinessDomainUnknown
	}

	// Module AI Decision: consumer decision_events_queue + pipeline case/orchestrate/execute + tương đương.
	if d := businessDomainFromAIDModule(ph); d != "" {
		return d
	}

	return businessDomainFallback(ev)
}

// businessDomainFromAIDModule — các phase do package aidecision (consumer, engine, orchestrate…) phát mốc.
func businessDomainFromAIDModule(ph string) string {
	if isQueueConsumerFeedPhase(ph) {
		return BusinessDomainAIDecision
	}
	if ph == PhaseSkipped {
		// Consumer AID (routing noop / no handler) và engine AID đều dùng skipped.
		return BusinessDomainAIDecision
	}
	if ph == PhaseAdsEvaluate {
		return BusinessDomainAIDecision
	}
	if isExecutePipelinePhase(ph) || isOrchestrationPipelinePhase(ph) {
		return BusinessDomainAIDecision
	}
	return ""
}

func businessDomainFromIntelDomainRef(id string) string {
	switch strings.TrimSpace(id) {
	case IntelDomainCIX:
		return BusinessDomainCIX
	case IntelDomainCRMIntel, IntelDomainCrmContext, IntelDomainCrmPendingMerge:
		return BusinessDomainCRM
	case IntelDomainOrderIntel:
		return BusinessDomainOrder
	case IntelDomainAdsIntel:
		return BusinessDomainAds
	default:
		return ""
	}
}

// businessDomainFallback — Mốc thiếu phase chuẩn: map thô từ feed/refs (webhook, CIO tùy emitter…).
func businessDomainFallback(ev *DecisionLiveEvent) string {
	cat := strings.TrimSpace(ev.FeedSourceCategory)
	switch cat {
	case FeedSourceWebhook:
		return BusinessDomainWebhook
	case FeedSourceDecision, FeedSourceIntel:
		return BusinessDomainAIDecision
	case FeedSourceQueue:
		// Bản ghi cũ / thiếu phase nhưng vẫn gắn nhóm hàng đợi consumer AID.
		return BusinessDomainAIDecision
	}
	if ev.Refs != nil {
		et := strings.TrimSpace(ev.Refs["eventType"])
		if et == "" {
			et = strings.TrimSpace(ev.Refs["event_type"])
		}
		if strings.HasPrefix(strings.ToLower(et), "webhook_log.") {
			return BusinessDomainWebhook
		}
		if d := businessDomainFromEventType(et); d != "" {
			return d
		}
	}
	return BusinessDomainUnknown
}

// businessDomainFromEventType — chỉ dùng khi cần mở rộng fallback; không áp cho job trên decision_events_queue (đã là aidecision theo phase).
func businessDomainFromEventType(et string) string {
	s := strings.TrimSpace(et)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	switch low {
	case eventtypes.AIDecisionExecuteRequested, eventtypes.ExecutorProposeRequested, eventtypes.AdsProposeRequested:
		return BusinessDomainAIDecision
	}
	if strings.HasPrefix(low, "webhook_log.") {
		return BusinessDomainWebhook
	}
	if strings.HasPrefix(low, eventtypes.PrefixCrmDot) || strings.HasPrefix(low, eventtypes.PrefixCrmUnderscore) ||
		strings.HasPrefix(low, "fb_customer.") || strings.HasPrefix(low, eventtypes.PrefixCustomerContext) {
		return BusinessDomainCRM
	}
	if strings.HasPrefix(low, "pos_") {
		return BusinessDomainPC
	}
	if strings.HasPrefix(low, "meta_") {
		return BusinessDomainMeta
	}
	if strings.HasPrefix(low, "fb_page.") || strings.HasPrefix(low, "fb_post.") || strings.HasPrefix(low, "fb_message_item.") {
		return BusinessDomainFB
	}
	if strings.HasPrefix(low, eventtypes.PrefixAdsContext) || low == eventtypes.AdsUpdated || strings.HasPrefix(low, "ads.") {
		return BusinessDomainAds
	}
	if strings.HasPrefix(low, eventtypes.PrefixCixDot) {
		return BusinessDomainCIX
	}
	if strings.HasPrefix(low, eventtypes.PrefixOrder) || strings.HasPrefix(low, eventtypes.PrefixOrderRecompute) ||
		strings.HasPrefix(low, eventtypes.PrefixOrderIntelligenceLegacy) || low == eventtypes.OrderIntelRecomputed {
		return BusinessDomainOrder
	}
	if strings.HasPrefix(low, eventtypes.PrefixConversation) || strings.HasPrefix(low, eventtypes.PrefixMessage) ||
		low == eventtypes.ConversationMessageInserted || low == eventtypes.MessageBatchReady {
		return BusinessDomainConversation
	}
	return ""
}
