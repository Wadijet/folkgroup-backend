// Package livecopy — Khung mô tả thống nhất cho DecisionLiveEvent (domain theo eventType queue).
package livecopy

import (
	"strings"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// DomainNarrative mô tả nghiệp vụ cố định theo loại sự kiện queue (tiếng Việt).
type DomainNarrative struct {
	StepTitle       string   // Tiêu đề bước (TraceStep.Title)
	BusinessOneLine string   // Một dòng bối cảnh nghiệp vụ (ReasoningSummary / Step.Reasoning)
	EntityBullets   []string // Gạch đầu dòng tham chiếu entity (không trùng eventId)
}

// DomainNarrativeFromQueueEvent trích narrative từ envelope decision_events_queue.
func DomainNarrativeFromQueueEvent(evt *aidecisionmodels.DecisionEvent) DomainNarrative {
	out := DomainNarrative{
		StepTitle:       "Đang xử lý tự động",
		BusinessOneLine: "Hệ thống đã nhận một cập nhật và sẽ xử lý theo đúng loại thông tin.",
	}
	if evt == nil {
		return out
	}
	et := strings.TrimSpace(evt.EventType)
	src := strings.TrimSpace(evt.EventSource)
	p := evt.Payload
	campaignID := strFromPayload(p, "campaignId", "campaign_id")
	adAccountID := strFromPayload(p, "adAccountId", "ad_account_id")
	orderID := strFromPayload(p, "orderId", "order_id", "orderUid", "order_uid")
	convID := strFromPayload(p, "conversationId", "conversation_id")
	custID := strFromPayload(p, "customerId", "customer_id", "customerUid", "customer_uid")
	decisionCaseID := strFromPayload(p, "decisionCaseId", "decisionCaseID")

	switch et {
	case eventtypes.CampaignIntelRecomputed:
		out.StepTitle = "Số liệu quảng cáo vừa được làm mới"
		out.BusinessOneLine = "Bức tranh chiến dịch vừa được cập nhật; tiếp theo có thể có gợi ý tối ưu nếu phù hợp."
		if campaignID != "" {
			out.EntityBullets = append(out.EntityBullets, "Chiến dịch: "+campaignID)
		}
		if adAccountID != "" {
			out.EntityBullets = append(out.EntityBullets, "Tài khoản quảng cáo: "+adAccountID)
		}
	case eventtypes.CrmIntelRecomputed:
		out.StepTitle = "Thông tin khách hàng vừa được làm mới"
		out.BusinessOneLine = "Hệ thống vừa cập nhật bức tranh khách; có thể có gợi ý tiếp theo khi đủ dữ liệu."
		if u := strFromPayload(p, "unifiedId", "unified_id"); u != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách (unified): "+u)
		}
	case eventtypes.OrderIntelRecomputed:
		out.StepTitle = "Thông tin đơn hàng vừa được làm mới"
		out.BusinessOneLine = "Tóm tắt đơn (trạng thái, rủi ro nếu có) đã cập nhật để các bước sau dùng số mới nhất."
		if orderID != "" {
			out.EntityBullets = append(out.EntityBullets, "Đơn: "+orderID)
		}
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
	case eventtypes.CixIntelRecomputed:
		out.StepTitle = "Phân tích hội thoại đã xong"
		out.BusinessOneLine = "Hệ thống đã đọc ý nghĩa tin nhắn; có thể cập nhật hồ sơ và gợi ý việc tiếp theo."
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
	case eventtypes.MetaCampaignChanged, eventtypes.MetaCampaignInserted, eventtypes.MetaCampaignUpdated:
		out.StepTitle = "Đồng bộ chiến dịch quảng cáo (Meta)"
		out.BusinessOneLine = "Cập nhật thông tin chiến dịch từ Meta; có thể dùng cho báo cáo và gợi ý tối ưu."
		if campaignID != "" {
			out.EntityBullets = append(out.EntityBullets, "Chiến dịch: "+campaignID)
		}
		if adAccountID != "" {
			out.EntityBullets = append(out.EntityBullets, "Tài khoản quảng cáo: "+adAccountID)
		}
	case eventtypes.AdsContextRequested:
		out.StepTitle = "Đang thu thập số liệu quảng cáo"
		out.BusinessOneLine = "Hệ thống đang gom số liệu chiến dịch để đánh giá và gợi ý nếu cần."
		if campaignID != "" {
			out.EntityBullets = append(out.EntityBullets, "Chiến dịch: "+campaignID)
		}
	case eventtypes.AdsContextReady:
		out.StepTitle = "Đã đủ thông tin để gợi ý quảng cáo"
		out.BusinessOneLine = "Số liệu đã sẵn sàng; hệ thống sẽ đánh giá và tạo gợi ý nếu thấy phù hợp."
		out.EntityBullets = append(out.EntityBullets, "Có thể không có gợi ý nếu chưa đạt điều kiện.")
	case eventtypes.OrderChanged, eventtypes.OrderInserted, eventtypes.OrderUpdated:
		out.StepTitle = "Thay đổi đơn hàng"
		out.BusinessOneLine = "Đơn mới hoặc vừa sửa; có thể cập nhật cảnh báo và theo dõi rủi ro."
		if orderID != "" {
			out.EntityBullets = append(out.EntityBullets, "Đơn: "+orderID)
		}
	case eventtypes.ConversationChanged, eventtypes.MessageChanged,
		eventtypes.ConversationInserted, eventtypes.ConversationUpdated, eventtypes.MessageInserted, eventtypes.MessageUpdated:
		out.StepTitle = "Có tin nhắn hoặc hội thoại mới"
		out.BusinessOneLine = "Sau khi bạn lưu tin nhắn, hệ thống sẽ đồng bộ và có thể phân tích, gợi ý trả lời hoặc việc cần làm."
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
		if custID != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách: "+custID)
		}
	case eventtypes.ConversationMessageInserted, eventtypes.MessageBatchReady:
		out.StepTitle = "Đang gom tin nhắn để phân tích"
		out.BusinessOneLine = "Hệ thống đang chuẩn bị nhóm tin nhắn trước khi phân tích sâu."
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
	case eventtypes.CustomerContextReady:
		out.StepTitle = "Thông tin khách đã đủ"
		out.BusinessOneLine = "Đủ thông tin khách để kết hợp với phân tích và quyết định."
		if custID != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách: "+custID)
		}
	case eventtypes.ExecutorProposeRequested, eventtypes.AdsProposeRequested:
		out.StepTitle = "Tạo đề xuất chờ duyệt"
		out.BusinessOneLine = "Đưa gợi ý vào bước duyệt hoặc thực hiện."
		out.EntityBullets = append(out.EntityBullets, "Bước sau: duyệt / thực hiện trên hệ thống.")
	case eventtypes.PosVariationUpdated, eventtypes.PosProductUpdated, eventtypes.PosCustomerUpdated, eventtypes.PosShopUpdated, eventtypes.PosWarehouseUpdated:
		out.StepTitle = "Đồng bộ POS / kho / sản phẩm"
		out.BusinessOneLine = "Dữ liệu cửa hàng hoặc kho thay đổi; có thể cập nhật báo cáo liên quan."
	case eventtypes.MetaAdUpdated, eventtypes.MetaAdsetUpdated, eventtypes.MetaAdInsightUpdated, eventtypes.MetaAdAccountUpdated:
		out.StepTitle = "Đồng bộ quảng cáo Meta (chi tiết)"
		out.BusinessOneLine = "Cập nhật quảng cáo hoặc số liệu từ Meta cho báo cáo."
	case eventtypes.CrmIntelligenceComputeRequested:
		out.StepTitle = "Yêu cầu cập nhật chỉ số khách"
		out.BusinessOneLine = "Đã xếp hàng tính lại chỉ số / intelligence gắn với khách hàng."
	case eventtypes.CrmIntelligenceRecomputeRequested:
		out.StepTitle = "Yêu cầu làm mới thông tin khách"
		out.BusinessOneLine = "Hệ thống sẽ tính lại chỉ số khách sau khi dữ liệu thay đổi (có thể gom nhiều lần cập nhật gần nhau)."
		if u := strFromPayload(p, "unifiedId", "unified_id"); u != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách (unified): "+u)
		}
	default:
		if strings.HasPrefix(et, "meta_") {
			out.StepTitle = "Đồng bộ Meta"
			out.BusinessOneLine = "Cập nhật thông tin liên quan Meta Ads."
		} else if strings.HasPrefix(et, "pos_") {
			out.StepTitle = "Đồng bộ POS"
			out.BusinessOneLine = "Cập nhật dữ liệu cửa hàng hoặc kho."
		} else {
			out.BusinessOneLine = "Xử lý theo loại sự kiện đã cấu hình trên hàng đợi."
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
