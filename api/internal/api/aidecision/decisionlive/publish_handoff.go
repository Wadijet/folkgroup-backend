// publish_handoff — Bổ sung dòng «Bước chuyển» trong DetailBullets khi có handoff giữa AI Decision và miền nghiệp vụ.
package decisionlive

import (
	"strings"
)

// enrichPublishHandoffNarrative chèn (sau dòng E2E nếu có) một gạch đầu dòng mô tả bước chuyển miền — không thay thế refs/e2e.
func enrichPublishHandoffNarrative(ev *DecisionLiveEvent) {
	if ev == nil {
		return
	}
	line := handoffPublishLineVi(ev)
	if line == "" {
		return
	}
	if detailBulletsContainHandoffNarrative(ev.DetailBullets) {
		return
	}
	ev.DetailBullets = insertHandoffDetailBullet(ev.DetailBullets, line)
}

func detailBulletsContainHandoffNarrative(bullets []string) bool {
	for _, b := range bullets {
		s := strings.TrimSpace(b)
		if strings.Contains(s, "Bước chuyển:") || strings.Contains(s, "Miền chuyển giao:") {
			return true
		}
	}
	return false
}

// insertHandoffDetailBullet — ưu tiên chèn ngay sau dòng «Trong quy trình:» (prependE2EPublishNarrative).
func insertHandoffDetailBullet(bullets []string, line string) []string {
	if len(bullets) == 0 {
		return []string{line}
	}
	first := strings.TrimSpace(bullets[0])
	if strings.HasPrefix(first, "Trong quy trình:") {
		out := make([]string, 0, len(bullets)+1)
		out = append(out, bullets[0])
		out = append(out, line)
		out = append(out, bullets[1:]...)
		return out
	}
	return append([]string{line}, bullets...)
}

func handoffPublishLineVi(ev *DecisionLiveEvent) string {
	if ev.Refs != nil {
		if v := strings.TrimSpace(ev.Refs["handoffNoteVi"]); v != "" {
			return v
		}
	}
	if ev.Step != nil && ev.Step.OutputRef != nil {
		if v, ok := ev.Step.OutputRef["handoffNoteVi"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		if v, ok := ev.Step.OutputRef["handoffDomainVi"].(string); ok && strings.TrimSpace(v) != "" {
			return "Bước chuyển: " + strings.TrimSpace(v) + "."
		}
		if jt, ok := ev.Step.OutputRef["jobType"].(string); ok {
			if line := handoffLineFromJobType(jt); line != "" {
				return line
			}
		}
	}
	return handoffLineFromAIDEventRefs(ev.Refs)
}

func handoffLineFromAIDEventRefs(refs map[string]string) string {
	if refs == nil {
		return ""
	}
	es := strings.TrimSpace(refs["eventSource"])
	et := strings.TrimSpace(refs["eventType"])
	if et == "" || es != "aidecision" {
		return ""
	}
	label := domainLabelViFromEventType(et)
	if label == "" {
		return ""
	}
	return "Bước chuyển: AI Decision đã tạo việc cho miền «" + label + "» — loại sự kiện: " + et + "."
}

func handoffLineFromJobType(jt string) string {
	jt = strings.ToLower(strings.TrimSpace(jt))
	if jt == "" {
		return ""
	}
	switch {
	case strings.Contains(jt, "crm_intel"):
		return "Bước chuyển: đã xếp hàng việc cho worker miền CRM (intel)."
	case strings.Contains(jt, "cix_intel"):
		return "Bước chuyển: đã xếp hàng việc cho worker miền CIX (intel hội thoại)."
	case strings.Contains(jt, "order_intel"):
		return "Bước chuyển: đã xếp hàng việc cho worker miền Đơn hàng (intel)."
	case strings.Contains(jt, "ads_intel"):
		return "Bước chuyển: đã xếp hàng việc cho worker miền Quảng cáo (intel)."
	default:
		return ""
	}
}

// domainLabelViFromEventType — nhãn ngắn cho lưu đồ nghiệp vụ (docs/flows/bang-pha-buoc-event-e2e §1.3).
func domainLabelViFromEventType(et string) string {
	et = strings.TrimSpace(et)
	if et == "" {
		return ""
	}
	low := strings.ToLower(et)
	switch {
	case strings.HasPrefix(low, "cix.") || strings.HasPrefix(low, "cix_intel"):
		return "CIX (hội thoại)"
	case strings.HasPrefix(low, "crm.") || strings.HasPrefix(low, "crm_"):
		return "CRM / khách hàng"
	case strings.HasPrefix(low, "customer."):
		return "Khách hàng (CRM)"
	case strings.HasPrefix(low, "order.") || strings.HasPrefix(low, "order_intel"):
		return "Đơn hàng"
	case strings.HasPrefix(low, "ads.") || strings.HasPrefix(low, "campaign_intel") || strings.HasPrefix(low, "meta_ad") || strings.HasPrefix(low, "meta_campaign"):
		return "Quảng cáo / Meta"
	case strings.HasPrefix(low, "aidecision.execute_requested"):
		return "Thực thi (Executor)"
	case strings.HasPrefix(low, "executor."):
		return "Executor"
	case strings.HasPrefix(low, "conversation.") || strings.HasPrefix(low, "message."):
		return "Hội thoại / tin nhắn"
	default:
		return ""
	}
}
