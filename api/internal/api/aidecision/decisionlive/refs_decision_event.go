// Package decisionlive — Tham chiếu audit/trace từ envelope decision_events_queue → DecisionLiveEvent.Refs.
package decisionlive

import (
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// RefsFromDecisionEventEnvelope trích mọi khóa liên kết hợp lệ từ bản ghi queue để UI/audit nhóm theo trace, job, entity, chuỗi nhân quả.
func RefsFromDecisionEventEnvelope(evt *aidecisionmodels.DecisionEvent) map[string]string {
	if evt == nil {
		return nil
	}
	m := map[string]string{}
	put := func(k, v string) {
		v = strings.TrimSpace(v)
		if k == "" || v == "" {
			return
		}
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	put("eventId", evt.EventID)
	put("eventType", evt.EventType)
	put("eventSource", evt.EventSource)
	put("entityType", evt.EntityType)
	put("entityId", evt.EntityID)
	put("orgId", evt.OrgID)
	put("lane", evt.Lane)
	put("parentEventId", evt.ParentEventID)
	put("rootEventId", evt.RootEventID)
	put("causationEventId", evt.CausationEventID)
	put("traceId", evt.TraceID)
	put("w3cTraceId", evt.W3CTraceID)
	put("correlationId", evt.CorrelationID)

	if evt.Payload != nil {
		put("decisionCaseId", strFromPayload(evt.Payload, "decisionCaseId", "decisionCaseID"))
		put("payloadTraceId", strFromPayload(evt.Payload, "traceId", "trace_id"))
		put("campaignId", strFromPayload(evt.Payload, "campaignId", "campaign_id"))
		put("adAccountId", strFromPayload(evt.Payload, "adAccountId", "ad_account_id"))
		put("orderId", strFromPayload(evt.Payload, "orderId", "order_id", "orderUid", "order_uid"))
		put("conversationId", strFromPayload(evt.Payload, "conversationId", "conversation_id"))
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func strFromPayload(p map[string]interface{}, keys ...string) string {
	if p == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := p[k].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// MergeRefsFromDecisionEnvelope gộp refs từ envelope queue vào sự kiện live — không ghi đè khóa đã có (giống EnrichLiveEventFromCase).
func MergeRefsFromDecisionEnvelope(ev *DecisionLiveEvent, evt *aidecisionmodels.DecisionEvent) {
	if ev == nil {
		return
	}
	from := RefsFromDecisionEventEnvelope(evt)
	if len(from) == 0 {
		return
	}
	if ev.Refs == nil {
		ev.Refs = make(map[string]string)
	}
	for k, v := range from {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if _, ok := ev.Refs[k]; !ok || ev.Refs[k] == "" {
			ev.Refs[k] = v
		}
	}
}
