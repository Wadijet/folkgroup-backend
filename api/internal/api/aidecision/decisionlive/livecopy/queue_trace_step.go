// queue_trace_step — Phương án B: điền TraceStep (inputRef / reasoning / outputRef) cho mốc consumer queue.
package livecopy

import (
	"fmt"
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/eventtypes"
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
	return &decisionlive.TraceStep{
		Index:     0,
		Kind:      "queue",
		Title:     dn.StepTitle,
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

// buildQueueConsumerTraceStepNarrativeVi — Reasoning theo khung: Mục đích / Đầu vào / Đã xét / Kết quả / Tiếp theo (docs bang-pha-buoc-event-e2e).
func buildQueueConsumerTraceStepNarrativeVi(evt *aidecisionmodels.DecisionEvent, ms QueueMilestone, dn DomainNarrative, processErr error) string {
	purpose := queueTraceStepPurposeVi(ms, dn)
	inputSum := queueTraceStepInputSummaryVi(evt)
	logic := queueTraceStepLogicVi(ms, dn, evt, processErr)
	result := queueTraceStepResultSummaryVi(ms, processErr)
	next := queueTraceStepNextHintVi(ms, evt)
	return decisionlive.FormatLiveStepNarrativeVi(purpose, inputSum, logic, result, next)
}

func queueTraceStepPurposeVi(ms QueueMilestone, _ DomainNarrative) string {
	// Một câu — bám khung purpose trong docs/flows/bang-pha-buoc-event-e2e §4.1; bối cảnh nghiệp vụ nằm ở summary / reasoningSummary / title.
	switch ms {
	case QueueMilestoneProcessingStart:
		return "Xác nhận hệ thống đã nhận việc từ hàng đợi nội bộ và sắp xử lý."
	case QueueMilestoneDatachangedDone:
		return "Đọc snapshot mới nhất và chạy các side-effect đã đăng ký sau khi dữ liệu nguồn thay đổi."
	case QueueMilestoneHandlerDone:
		return "Hoàn tất bước nghiệp vụ đã đăng ký cho loại cập nhật này."
	case QueueMilestoneHandlerError:
		return "Ghi nhận lỗi kỹ thuật khi chạy handler nghiệp vụ."
	case QueueMilestoneRoutingSkipped:
		return "Áp dụng quy tắc routing: lần này không chạy handler tự động."
	case QueueMilestoneNoHandler:
		return "Không có handler đăng ký cho loại sự kiện trên consumer."
	default:
		return "Mốc xử lý job trên hàng đợi (G2 — consumer)."
	}
}

func queueTraceStepInputSummaryVi(evt *aidecisionmodels.DecisionEvent) string {
	if evt == nil {
		return "Thiếu envelope queue."
	}
	friendly := queueFriendlyEventLabel(evt)
	et := strings.TrimSpace(evt.EventType)
	es := strings.TrimSpace(evt.EventSource)
	if friendly != "" && et != "" {
		return fmt.Sprintf("Loại việc: %s; mã loại `%s`; nguồn `%s`. Tham chiếu đầy đủ trong inputRef.", friendly, et, es)
	}
	if et != "" {
		return fmt.Sprintf("Mã loại `%s`; nguồn `%s`. Tham chiếu trong inputRef.", et, es)
	}
	return "Tham chiếu đầy đủ trong inputRef."
}

func queueTraceStepLogicVi(ms QueueMilestone, _ DomainNarrative, _ *aidecisionmodels.DecisionEvent, processErr error) string {
	// Bám ý logicSummary §4.1: kiểm tra / thứ tự — không nhồi tên hàm; audit kỹ thuật nằm inputRef/outputRef.
	switch ms {
	case QueueMilestoneRoutingSkipped:
		return "Đã kiểm tra quy tắc routing theo tổ chức và loại sự kiện — lần này bỏ qua dispatch handler."
	case QueueMilestoneNoHandler:
		return "Đã tra bảng handler đăng ký — không có mục phù hợp loại sự kiện."
	case QueueMilestoneHandlerError:
		if processErr != nil {
			return fmt.Sprintf("Handler báo lỗi: %s", truncateRunes(processErr.Error(), 220))
		}
		return "Handler báo lỗi (không có thông điệp chi tiết)."
	case QueueMilestoneDatachangedDone:
		return "Kiểm tra loại nguồn và policy; chạy chuỗi đồng bộ đã đăng ký (chi tiết từng bước trong processTrace)."
	case QueueMilestoneHandlerDone:
		return "Đã dispatch handler đăng ký; handler trả về không lỗi."
	case QueueMilestoneProcessingStart:
		return "Đã giữ lock job trên hàng đợi; bắt đầu processEvent theo thứ tự chuẩn consumer."
	default:
		return ""
	}
}

func queueTraceStepResultSummaryVi(ms QueueMilestone, processErr error) string {
	switch ms {
	case QueueMilestoneProcessingStart:
		return "Job đã được nhận; consumer bắt đầu xử lý (mốc timeline kế tiếp)."
	case QueueMilestoneDatachangedDone:
		return "Đã chạy xong đồng bộ sau lưu và side-effect đã đăng ký."
	case QueueMilestoneHandlerDone:
		return "Handler chạy xong, không lỗi."
	case QueueMilestoneHandlerError:
		if processErr != nil {
			return "Xử lý thất bại; có thể thử lại — xem errorMessage trong outputRef."
		}
		return "Xử lý thất bại ở handler."
	case QueueMilestoneRoutingSkipped:
		return "Không gọi handler; mốc được ghi trên timeline theo chính sách bỏ qua."
	case QueueMilestoneNoHandler:
		return "Không chạy handler; dữ liệu nguồn vẫn được lưu bình thường."
	default:
		return ""
	}
}

func queueTraceStepNextHintVi(ms QueueMilestone, evt *aidecisionmodels.DecisionEvent) string {
	switch ms {
	case QueueMilestoneProcessingStart:
		if evt != nil && eventtypes.IsL1DatachangedEventSource(evt.EventSource) {
			return "Chuyển sang bước đồng bộ sau lưu (nếu có), rồi routing và dispatch handler."
		}
		return "Chuyển sang routing và dispatch handler (nếu có)."
	case QueueMilestoneDatachangedDone:
		return "Chuyển sang routing / handler nghiệp vụ, hoặc kết thúc nếu bỏ qua hoặc không có handler."
	case QueueMilestoneHandlerDone, QueueMilestoneHandlerError, QueueMilestoneRoutingSkipped, QueueMilestoneNoHandler:
		return "Kết thúc bước consumer; cập nhật trạng thái job trên hàng đợi."
	default:
		return ""
	}
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
