// Package livecopy — Khung mô tả thống nhất cho DecisionLiveEvent (domain theo eventType queue).
package livecopy

import (
	"strings"

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
		StepTitle:       "Xử lý tác vụ",
		BusinessOneLine: "Một việc đã được xếp hàng chờ; hệ thống sẽ xử lý theo đúng loại việc.",
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
	case "meta_campaign.inserted", "meta_campaign.updated":
		out.StepTitle = "Đồng bộ chiến dịch quảng cáo (Meta)"
		out.BusinessOneLine = "Cập nhật thông tin chiến dịch từ Meta; có thể dùng cho báo cáo và gợi ý tối ưu."
		if campaignID != "" {
			out.EntityBullets = append(out.EntityBullets, "Chiến dịch: "+campaignID)
		}
		if adAccountID != "" {
			out.EntityBullets = append(out.EntityBullets, "Tài khoản quảng cáo: "+adAccountID)
		}
	case "ads.context_requested":
		out.StepTitle = "Chuẩn bị dữ liệu quảng cáo"
		out.BusinessOneLine = "Đang lấy số liệu và trạng thái chiến dịch để đánh giá theo quy tắc."
		if campaignID != "" {
			out.EntityBullets = append(out.EntityBullets, "Chiến dịch: "+campaignID)
		}
	case "ads.context_ready":
		out.StepTitle = "Đánh giá gợi ý quảng cáo"
		out.BusinessOneLine = "Đã đủ dữ liệu; hệ thống áp quy tắc để có thể tạo gợi ý hoặc kết thúc hợp lý."
		out.EntityBullets = append(out.EntityBullets, "Có thể không có gợi ý nếu chưa đạt điều kiện.")
	case "order.inserted", "order.updated":
		out.StepTitle = "Thay đổi đơn hàng"
		out.BusinessOneLine = "Đơn mới hoặc vừa sửa; có thể cập nhật cảnh báo và theo dõi rủi ro."
		if orderID != "" {
			out.EntityBullets = append(out.EntityBullets, "Đơn: "+orderID)
		}
	case "order.flags_emitted":
		out.StepTitle = "Cờ trên đơn đã cập nhật"
		out.BusinessOneLine = "Đã có cờ/cảnh báo trên đơn; có thể bước sau là đánh giá hoặc thông báo."
		if orderID != "" {
			out.EntityBullets = append(out.EntityBullets, "Đơn: "+orderID)
		}
	case "conversation.inserted", "conversation.updated", "message.inserted", "message.updated":
		out.StepTitle = "Đồng bộ hội thoại / tin nhắn"
		out.BusinessOneLine = "Tin nhắn hoặc hội thoại thay đổi; có thể gom nhóm trước khi phân tích."
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
		if custID != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách: "+custID)
		}
	case "conversation.message_inserted", "message.batch_ready":
		out.StepTitle = "Phân tích hội thoại"
		out.BusinessOneLine = "Đang xử lý tin nhắn để phân tích nội dung."
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
	case "cix_analysis_result.inserted", "cix_analysis_result.updated", "cix.analysis_completed":
		out.StepTitle = "Kết quả phân tích hội thoại"
		out.BusinessOneLine = "Đã có kết quả phân tích; có thể cập nhật hồ sơ và bước xử lý tiếp."
		if convID != "" {
			out.EntityBullets = append(out.EntityBullets, "Hội thoại: "+convID)
		}
	case "customer.context_ready":
		out.StepTitle = "Thông tin khách đã đủ"
		out.BusinessOneLine = "Đủ thông tin khách để kết hợp với phân tích và quyết định."
		if custID != "" {
			out.EntityBullets = append(out.EntityBullets, "Khách: "+custID)
		}
	case "conversation.intelligence_requested":
		out.StepTitle = "Tính lại tóm tắt hội thoại"
		out.BusinessOneLine = "Yêu cầu tổng hợp/tính toán theo cấu hình."
	case "executor.propose_requested", "ads.propose_requested":
		out.StepTitle = "Tạo đề xuất chờ duyệt"
		out.BusinessOneLine = "Đưa gợi ý vào bước duyệt hoặc thực hiện."
		out.EntityBullets = append(out.EntityBullets, "Bước sau: duyệt / thực hiện trên hệ thống.")
	case "pos_variation.updated", "pos_product.updated", "pos_customer.updated", "pos_shop.updated", "pos_warehouse.updated":
		out.StepTitle = "Đồng bộ POS / kho / sản phẩm"
		out.BusinessOneLine = "Dữ liệu cửa hàng hoặc kho thay đổi; có thể cập nhật báo cáo liên quan."
	case "meta_ad.updated", "meta_adset.updated", "meta_ad_insight.updated", "meta_ad_account.updated":
		out.StepTitle = "Đồng bộ quảng cáo Meta (chi tiết)"
		out.BusinessOneLine = "Cập nhật quảng cáo hoặc số liệu từ Meta cho báo cáo."
	case "commerce.order_completed":
		out.StepTitle = "Đơn hoàn thành"
		out.BusinessOneLine = "Đơn đã hoàn thành; có thể kích hoạt báo cáo hoặc chăm sóc sau bán."
	case "crm.intelligence.compute_requested":
		out.StepTitle = "Cập nhật chỉ số khách (CRM)"
		out.BusinessOneLine = "Tính lại hoặc cập nhật chỉ số gắn với khách hàng."
	default:
		if strings.HasPrefix(et, "meta_") {
			out.StepTitle = "Đồng bộ Meta"
			out.BusinessOneLine = "Cập nhật thông tin liên quan Meta Ads."
		} else if strings.HasPrefix(et, "pos_") {
			out.StepTitle = "Đồng bộ POS"
			out.BusinessOneLine = "Cập nhật dữ liệu cửa hàng hoặc kho."
		} else {
			out.BusinessOneLine = "Xử lý theo loại việc đã cấu hình."
		}
	}
	if decisionCaseID != "" && !strings.Contains(strings.Join(out.EntityBullets, " "), decisionCaseID) {
		out.EntityBullets = append(out.EntityBullets, "Hồ sơ xử lý: "+decisionCaseID)
	}
	if src != "" {
		srcLabel := src
		srcMap := map[string]string{
			"datachanged":   "sau khi dữ liệu đổi",
			"aidecision":    "từ luồng AI Decision",
		}
		if v, ok := srcMap[src]; ok {
			srcLabel = v
		}
		out.EntityBullets = append(out.EntityBullets, "Nguồn phát: "+srcLabel)
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
