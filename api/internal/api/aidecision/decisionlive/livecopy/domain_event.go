// Package livecopy — Khung mô tả thống nhất cho DecisionLiveEvent (domain theo eventType queue).
package livecopy

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// DomainNarrative mô tả nghiệp vụ cố định theo loại sự kiện queue (tiếng Việt).
type DomainNarrative struct {
	StepTitle       string   // Tiêu đề bước (TraceStep.Title) — khung từ catalog §5.3 (descriptionUserVi) theo bước E2E đã resolve
	BusinessOneLine string   // Một dòng bối cảnh (ReasoningSummary) — cùng khung catalog; chi tiết tình huống ở EntityBullets
	EntityBullets   []string // Gạch đầu dòng tham chiếu entity / bối cảnh cụ thể (không trùng eventId)
}

// DomainNarrativeFromQueueEvent trích narrative từ envelope decision_events_queue.
// Khung chữ (StepTitle, BusinessOneLine) lấy từ luồng chuẩn §5.3 / E2EStepCatalog (descriptionUserVi) qua resolver; switch chỉ bổ sung phần cụ thể (entity, gạch đầu dòng).
func DomainNarrativeFromQueueEvent(evt *aidecisionmodels.DecisionEvent) DomainNarrative {
	var out DomainNarrative
	if evt == nil {
		out.StepTitle = "Thiếu thông tin sự kiện"
		out.BusinessOneLine = "Không có bản ghi job hàng đợi để mô tả."
		return out
	}
	et := strings.TrimSpace(evt.EventType)
	src := strings.TrimSpace(evt.EventSource)
	ps := strings.TrimSpace(evt.PipelineStage)
	applyQueueNarrativeCatalogFramework(&out, et, src, ps)

	p := evt.Payload
	campaignID := strFromPayload(p, "campaignId", "campaign_id")
	adAccountID := strFromPayload(p, "adAccountId", "ad_account_id")
	orderID := strFromPayload(p, "orderId", "order_id", "orderUid", "order_uid")
	convID := strFromPayload(p, "conversationId", "conversation_id")
	custID := strFromPayload(p, "customerId", "customer_id", "customerUid", "customer_uid")
	decisionCaseID := strFromPayload(p, "decisionCaseId", "decisionCaseID")

	switch et {
	case eventtypes.CampaignIntelRecomputed:
		if campaignID != "" {
			out.EntityBullets = append(out.EntityBullets, "Chiến dịch: "+campaignID)
		}
		if adAccountID != "" {
			out.EntityBullets = append(out.EntityBullets, "Tài khoản quảng cáo: "+adAccountID)
		}
	case eventtypes.CrmIntelRecomputed:
		if u := strFromPayload(p, "unifiedId", "unified_id"); u != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách (unified): "+u)
		}
	case eventtypes.OrderIntelRecomputed:
		if orderID != "" {
			out.EntityBullets = append(out.EntityBullets, "Đơn: "+orderID)
		}
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
	case eventtypes.CixIntelRecomputed:
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
	case eventtypes.MetaCampaignChanged, eventtypes.MetaCampaignInserted, eventtypes.MetaCampaignUpdated:
		if campaignID != "" {
			out.EntityBullets = append(out.EntityBullets, "Chiến dịch: "+campaignID)
		}
		if adAccountID != "" {
			out.EntityBullets = append(out.EntityBullets, "Tài khoản quảng cáo: "+adAccountID)
		}
	case eventtypes.AdsContextRequested:
		if campaignID != "" {
			out.EntityBullets = append(out.EntityBullets, "Chiến dịch: "+campaignID)
		}
	case eventtypes.AdsContextReady:
		out.EntityBullets = append(out.EntityBullets, "Có thể không có gợi ý nếu chưa đạt điều kiện.")
	case eventtypes.OrderChanged, eventtypes.OrderInserted, eventtypes.OrderUpdated:
		if orderID != "" {
			out.EntityBullets = append(out.EntityBullets, "Đơn: "+orderID)
		}
	case eventtypes.ConversationChanged, eventtypes.MessageChanged,
		eventtypes.ConversationInserted, eventtypes.ConversationUpdated, eventtypes.MessageInserted, eventtypes.MessageUpdated:
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
		if custID != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách: "+custID)
		}
	case eventtypes.ConversationMessageInserted, eventtypes.MessageBatchReady:
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
	case eventtypes.CustomerContextReady:
		if custID != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách: "+custID)
		}
	case eventtypes.ExecutorProposeRequested, eventtypes.AdsProposeRequested:
		out.EntityBullets = append(out.EntityBullets, "Bước sau: duyệt / thực hiện trên hệ thống.")
	case eventtypes.CrmIntelligenceRecomputeRequested:
		if u := strFromPayload(p, "unifiedId", "unified_id"); u != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách (unified): "+u)
		}
	}
	if decisionCaseID != "" && !strings.Contains(strings.Join(out.EntityBullets, " "), decisionCaseID) {
		out.EntityBullets = append(out.EntityBullets, "Hồ sơ xử lý: "+decisionCaseID)
	}
	if src != "" {
		srcLabel := src
		srcMap := map[string]string{
			eventtypes.EventSourceL1Datachanged: "sau khi dữ liệu đổi (L1)",
			eventtypes.EventSourceDatachanged:   "sau khi dữ liệu đổi",
			eventtypes.EventSourceAIDecision:    "từ luồng AI Decision",
		}
		if v, ok := srcMap[src]; ok {
			srcLabel = v
		}
		out.EntityBullets = append(out.EntityBullets, "Nguồn sự kiện: "+srcLabel)
	}
	return out
}

// applyQueueNarrativeCatalogFramework — một khung chữ từ §5.3: descriptionUserVi của bước E2E envelope (ResolveE2EForQueueEnvelope).
func applyQueueNarrativeCatalogFramework(out *DomainNarrative, eventType, eventSource, pipelineStage string) {
	if out == nil {
		return
	}
	ref := eventtypes.ResolveE2EForQueueEnvelope(eventType, eventSource, pipelineStage)
	frame := ""
	if ref.StepID != "" {
		frame = eventtypes.E2ECatalogDescriptionUserViForStep(ref.StepID)
	}
	if frame == "" && strings.TrimSpace(ref.LabelVi) != "" && !strings.HasPrefix(strings.TrimSpace(ref.LabelVi), "Chưa map E2E") {
		frame = strings.TrimSpace(ref.LabelVi)
	}
	if frame == "" && strings.TrimSpace(eventType) != "" {
		frame = "Cập nhật trên hàng đợi — " + queueEventTypeDisplayVi(eventType)
	}
	if frame == "" {
		frame = "Việc trên hàng đợi AI Decision"
	}
	out.StepTitle = frame
	out.BusinessOneLine = frame
}

// queueEventTypeDisplayVi — nhãn ngắn từ eventType khi chưa map catalog (chỉ dễ đọc hơn mã máy).
func queueEventTypeDisplayVi(eventType string) string {
	s := strings.TrimSpace(eventType)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, ".", " · ")
	if len(s) > 72 {
		return s[:69] + "…"
	}
	return s
}

func strFromPayload(p map[string]interface{}, keys ...string) string {
	if p == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := p[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
