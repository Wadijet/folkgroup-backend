// publish_queue_live — DecisionLiveEvent cho mọi job decision_events_queue (ngoài pipeline execute_requested).
package worker

import (
	"strings"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/decisionlive/livecopy"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// traceIDForQueueLive: ưu tiên envelope TraceID, sau đó payload traceId/trace_id, cuối cùng synthetic queue_evt_<eventId>.
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

// shouldSkipConsumerLiveSpan: execute_requested đã Publish đầy đủ trong ProcessExecuteRequested / engine — tránh trùng timeline.
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

// publishQueueConsumerLifecycleStart — sau RecordConsumerWorkBegin, trước processEvent.
func publishQueueConsumerLifecycleStart(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent) {
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneProcessingStart, nil, nil)
}

// publishQueueConsumerLifecycleEnd — sau processEvent (thành công / lỗi).
// kind: khi thành công — routing_skipped / no_handler đã publish milestone riêng, không gửi thêm HandlerDone (tránh lẫn với đã chạy handler).
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

// publishQueueDatachangedEffectsDone — sau applyDatachangedSideEffects (chỉ eventSource datachanged).
func publishQueueDatachangedEffectsDone(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent) {
	if evt == nil || ownerOrgID.IsZero() || evt.EventSource != "datachanged" {
		return
	}
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneDatachangedDone, nil, nil)
}

// publishQueueRoutingSkipped — rule routing noop dispatch.
func publishQueueRoutingSkipped(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent) {
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneRoutingSkipped, nil, nil)
}

// publishQueueNoRegisteredHandler — eventType chưa đăng ký consumer handler.
func publishQueueNoRegisteredHandler(ownerOrgID primitive.ObjectID, evt *aidecisionmodels.DecisionEvent) {
	if shouldSkipConsumerLiveSpan(evt) {
		return
	}
	publishQueueLivePhase(ownerOrgID, evt, livecopy.QueueMilestoneNoHandler, nil, nil)
}
