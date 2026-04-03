// publish_queue_live — Đẩy từng mốc xử lý job trên decision_events_queue lên timeline AI Decision Live
// (WebSocket / GET replay): bắt đầu xử lý, xong bước chuẩn bị sau datachanged, xong handler, lỗi, v.v.
// Job loại execute_requested không dùng chuỗi mốc này — timeline execute do engine publish riêng.
package worker

import (
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// traceIDForQueueLive — Chọn trace để gom mốc trên timeline: ưu tiên TraceID trên envelope,
// rồi traceId/trace_id trong payload, cuối cùng mã giả queue_evt_<eventId> nếu thiếu.
func traceIDForQueueLive(evt *aidecisionmodels.DecisionEvent) string {
	if evt == nil {
		return ""
	}
	if t := strings.TrimSpace(evt.TraceID); t != "" {
		return t
	}
	if evt.Payload != nil {
		for _, key := range []string{"traceId", "trace_id"} {
			if v, ok := evt.Payload[key].(string); ok {
				if s := strings.TrimSpace(v); s != "" {
					return s
				}
			}
		}
	}
	eid := strings.TrimSpace(evt.EventID)
	if eid != "" {
		return "queue_evt_" + eid
	}
	return ""
}

// shouldSkipConsumerLiveSpan — true với execute_requested vì timeline đã được engine ghi đủ — tránh trùng mốc queue.
func shouldSkipConsumerLiveSpan(evt *aidecisionmodels.DecisionEvent) bool {
	if evt == nil {
		return true
	}
	return evt.EventType == aidecisionsvc.EventTypeExecuteRequested
}

func publishQueueLivePhase(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent, ms livecopy.QueueMilestone, processErr error, extraBullets []string) {
	if ownerOrgID.IsZero() || evt == nil {
		return
	}
	tid := traceIDForQueueLive(evt)
	if tid == "" {
		return
	}
	ev := livecopy.BuildQueueConsumerEvent(evt, ms, processErr, extraBullets)
	decisionlive.Publish(ownerOrgID, tid, ev)
}

// publishQueueConsumerLifecycleStart — Mốc «bắt đầu xử lý job» (sau khi worker đã lease, trước processEvent).
func publishQueueConsumerLifecycleStart(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent) {
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneProcessingStart, nil, nil)
}

// publishQueueConsumerLifecycleEnd — Mốc «kết thúc xử lý» sau processEvent (thành công hoặc lỗi).
// Nếu kind là routing_skipped hoặc no_handler thì không gửi HandlerDone — các trường hợp đó đã có mốc riêng, tránh hiểu nhầm là đã chạy handler đầy đủ.
func publishQueueConsumerLifecycleEnd(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent, processErr error, kind aidecisionmodels.ConsumerCompletionKind) {
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	if processErr != nil {
		publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneHandlerError, processErr, nil)
		return
	}
	switch kind {
	case aidecisionmodels.ConsumerCompletionKindRoutingSkipped, aidecisionmodels.ConsumerCompletionKindNoHandler:
		return
	default:
		publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneHandlerDone, nil, nil)
	}
}

// publishQueueDatachangedEffectsDone — Mốc «đã xong bước chuẩn bị sau khi dữ liệu đổi» (chỉ EventSource = datachanged).
func publishQueueDatachangedEffectsDone(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent) {
	if evt == nil || ownerOrgID.IsZero() || evt.EventSource != "datachanged" {
		return
	}
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneDatachangedDone, nil, nil)
}

// publishQueueRoutingSkipped — Mốc «bỏ qua theo quy tắc routing» (noop — không gọi handler nghiệp vụ).
func publishQueueRoutingSkipped(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent) {
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneRoutingSkipped, nil, nil)
}

// publishQueueNoRegisteredHandler — Mốc «chưa có handler» — eventType chưa được đăng ký trong consumer.
func publishQueueNoRegisteredHandler(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent) {
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneNoHandler, nil, nil)
}
