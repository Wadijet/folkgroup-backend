// queue_trace_step — Phương án B: điền TraceStep (inputRef / reasoning / outputRef) cho mốc consumer queue.
package livecopy

import (
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

const (
	maxTraceRefStrRunes = 320
	maxTraceRefKeys     = 28
)

// traceRefPair — cặp khóa–giá trị ref (thứ tự chèn = ưu tiên audit).
type traceRefPair struct {
	k string
	v interface{}
}

// queueMilestoneTraceKey — Mã ổn định cho audit / ETL (khớp tên mốc trong doc).
func queueMilestoneTraceKey(ms QueueMilestone) string {
	switch ms {
	case QueueMilestoneProcessingStart:
		return "processing_start"
	case QueueMilestoneDatachangedDone:
		return "datachanged_effects_done"
	case QueueMilestoneHandlerDone:
		return "handler_done"
	case QueueMilestoneHandlerError:
		return "handler_error"
	case QueueMilestoneRoutingSkipped:
		return "routing_skipped"
	case QueueMilestoneNoHandler:
		return "no_handler"
	default:
		return "unknown"
	}
}

// buildQueueConsumerTraceStep — TraceStep có cấu trúc: đầu vào (ref), tại sao (reasoning), đầu ra (outputRef).
func buildQueueConsumerTraceStep(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, dn DomainNarrative, processErr error) *decisionlive.TraceStep {
	in := queueConsumerTraceStepInputRef(evt, ms)
	out := queueConsumerTraceStepOutputRef(evt, ms, processErr)
	frame := PublishCatalogUserViForQueueConsumerMilestone(evt, queueMilestoneToE2EKey(ms))
	return &decisionlive.TraceStep{
		Index:     0,
		Kind:      "queue",
		Title:     frame,
		Reasoning: buildQueueConsumerTraceStepNarrativeVi(evt, ms, dn, processErr),
		InputRef:  in,
		OutputRef: out,
	}
}

func queueConsumerTraceStepInputRef(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone) map[string]interface{} {
	var pairs []traceRefPair
	add := func(k string, v interface{}) {
		k = strings.TrimSpace(k)
		if k == "" {
			return
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				return
			}
			pairs = append(pairs, traceRefPair{k, strings.TrimSpace(t)})
		case bool, int, int64:
			pairs = append(pairs, traceRefPair{k, t})
		}
	}

	add("traceStepSchema", "1")
	add("queueMilestone", queueMilestoneTraceKey(ms))
	if evt == nil {
		return refPairsToMap(pairs)
	}
	add("eventId", evt.EventID)
	add("eventType", evt.EventType)
	add("eventSource", evt.EventSource)
	add("pipelineStage", evt.PipelineStage)
	if !evt.OwnerOrganizationID.IsZero() {
		add("ownerOrgIdHex", evt.OwnerOrganizationID.Hex())
	}
	add("traceId", evt.TraceID)
	add("correlationId", evt.CorrelationID)
	add("entityId", evt.EntityID)
	add("entityType", evt.EntityType)
	if evt.Payload != nil {
		if sc, ok := evt.Payload["sourceCollection"].(string); ok {
			add("sourceCollection", sc)
		}
		if uid, ok := evt.Payload["normalizedRecordUid"].(string); ok {
			add("normalizedRecordUid", uid)
		}
		if op, ok := evt.Payload["dataChangeOperation"].(string); ok {
			add("dataChangeOperation", op)
		}
		for _, key := range []string{"decisionCaseId", "decisionCaseID", "conversationId", "customerId", "unifiedId", "campaignId", "adAccountId", "objectId", "objectType"} {
			if v, ok := evt.Payload[key].(string); ok {
				add(key, v)
			}
		}
	}
	if dc := decisionCaseIDFromQueuePayload(evt); dc != "" {
		add("decisionCaseId", dc)
	}
	return refPairsToMap(pairs)
}

func queueConsumerTraceStepOutputRef(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, processErr error) map[string]interface{} {
	var pairs []traceRefPair
	add := func(k string, v interface{}) {
		k = strings.TrimSpace(k)
		if k == "" {
			return
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				return
			}
			pairs = append(pairs, traceRefPair{k, strings.TrimSpace(t)})
		case bool, int, int64:
			pairs = append(pairs, traceRefPair{k, t})
		}
	}

	add("traceStepSchema", "1")
	add("queueMilestone", queueMilestoneTraceKey(ms))

	switch ms {
	case QueueMilestoneProcessingStart:
		add("consumerPhase", "processing_start")
		add("resultVi", "Đã nhận job (lease); bắt đầu xử lý theo luồng G2.")
	case QueueMilestoneDatachangedDone:
		add("consumerPhase", "datachanged_effects_done")
		add("resultVi", "Đã chạy đồng bộ sau khi lưu và các side-effect đã đăng ký; chi tiết trong processTrace.")
		if evt != nil && isDatachangedCustomerMirrorOnly(evt) {
			add("noteVi", "Mirror khách: thường không còn handler nghiệp vụ trên consumer.")
		}
	case QueueMilestoneHandlerDone:
		add("consumerPhase", "handler_completed")
		add("resultVi", "Handler nghiệp vụ chạy xong, không lỗi.")
	case QueueMilestoneHandlerError:
		add("consumerPhase", "handler_error")
		add("resultVi", "Handler báo lỗi; job có thể được thử lại theo cấu hình hàng đợi.")
		if processErr != nil {
			add("errorMessage", truncateRunes(processErr.Error(), maxQueueErrRunes))
		}
	case QueueMilestoneRoutingSkipped:
		add("consumerPhase", "routing_skipped")
		add("policyVi", "Quy tắc routing: bỏ qua gọi handler cho lần cập nhật này.")
	case QueueMilestoneNoHandler:
		add("consumerPhase", "no_registered_handler")
		add("resultVi", "Chưa đăng ký handler cho loại sự kiện này.")
		if evt != nil && isDatachangedCustomerMirrorOnly(evt) {
			add("noteVi", "Luồng mirror khách chỉ đồng bộ — coi là bình thường (nominal).")
		}
	default:
		add("consumerPhase", "unknown")
	}
	return refPairsToMap(pairs)
}

// buildQueueConsumerTraceStepNarrativeVi — khung §5.3 theo milestone consumer; bổ sung lỗi kỹ thuật nếu có.
func buildQueueConsumerTraceStepNarrativeVi(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, dn DomainNarrative, processErr error) string {
	base := PublishCatalogUserViForQueueConsumerMilestone(evt, queueMilestoneToE2EKey(ms))
	if ms == QueueMilestoneHandlerError && processErr != nil {
		return PublishWithSituation(base, "Lỗi: "+truncateRunes(processErr.Error(), 220))
	}
	return base
}

func refPairsToMap(pairs []traceRefPair) map[string]interface{} {
	out := make(map[string]interface{})
	for _, p := range pairs {
		if len(out) >= maxTraceRefKeys {
			break
		}
		k := strings.TrimSpace(p.k)
		if k == "" {
			continue
		}
		switch t := p.v.(type) {
		case string:
			s := strings.TrimSpace(t)
			if s == "" {
				continue
			}
			out[k] = truncateRunes(s, maxTraceRefStrRunes)
		case bool:
			out[k] = t
		case int:
			out[k] = t
		case int64:
			out[k] = t
		default:
			continue
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
