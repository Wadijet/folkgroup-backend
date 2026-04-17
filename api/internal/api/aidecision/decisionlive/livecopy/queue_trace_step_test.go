package livecopy

import (
	"errors"
	"strings"
	"testing"

	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBuildQueueConsumerTraceStep_RefsHandlerDone(t *testing.T) {
	org := primitive.NewObjectID()
	evt := &aidecisionmodels.DecisionEvent{
		EventID:             "evt_1",
		EventType:           eventtypes.MessageChanged,
		EventSource:         eventtypes.EventSourceDatachanged,
		PipelineStage:       eventtypes.PipelineStageAfterL1Change,
		TraceID:             "tr_1",
		CorrelationID:       "co_1",
		OwnerOrganizationID: org,
		Payload: map[string]interface{}{
			"sourceCollection":    "fb_message_items",
			"normalizedRecordUid": "abc123",
			"dataChangeOperation": "update",
			"conversationId":      "conv_x",
		},
	}
	dn := DomainNarrativeFromQueueEvent(evt)
	step := buildQueueConsumerTraceStep(evt, QueueMilestoneHandlerDone, dn, nil)
	if step == nil {
		t.Fatal("step nil")
	}
	if step.Kind != "queue" {
		t.Fatalf("kind: %q", step.Kind)
	}
	if step.InputRef == nil || step.OutputRef == nil {
		t.Fatalf("inputRef/outputRef: in=%v out=%v", step.InputRef, step.OutputRef)
	}
	if step.InputRef["traceStepSchema"] != "1" {
		t.Fatalf("schema: %v", step.InputRef["traceStepSchema"])
	}
	if step.InputRef["queueMilestone"] != "handler_done" {
		t.Fatalf("milestone in: %v", step.InputRef["queueMilestone"])
	}
	if step.InputRef["eventId"] != "evt_1" || step.InputRef["sourceCollection"] != "fb_message_items" {
		t.Fatalf("input: %+v", step.InputRef)
	}
	if step.OutputRef["consumerPhase"] != "handler_completed" {
		t.Fatalf("output: %+v", step.OutputRef)
	}
	if !strings.Contains(step.Reasoning, "dispatch handler") || !strings.Contains(step.Reasoning, "không lỗi") {
		t.Fatalf("reasoning: %q", step.Reasoning)
	}
}

func TestBuildQueueConsumerTraceStep_RoutingSkipped(t *testing.T) {
	evt := &aidecisionmodels.DecisionEvent{
		EventID:     "evt_r",
		EventType:   eventtypes.CixAnalysisRequested,
		EventSource: "aidecision",
	}
	dn := DomainNarrativeFromQueueEvent(evt)
	step := buildQueueConsumerTraceStep(evt, QueueMilestoneRoutingSkipped, dn, nil)
	if step.OutputRef["consumerPhase"] != "routing_skipped" {
		t.Fatalf("out: %+v", step.OutputRef)
	}
	if !strings.Contains(step.Reasoning, "bỏ qua dispatch") {
		t.Fatalf("reasoning: %q", step.Reasoning)
	}
}

func TestBuildQueueConsumerTraceStep_HandlerError(t *testing.T) {
	evt := &aidecisionmodels.DecisionEvent{EventID: "e", EventType: "x", EventSource: "y"}
	dn := DomainNarrativeFromQueueEvent(evt)
	err := errors.New("thử nghiệm lỗi handler")
	step := buildQueueConsumerTraceStep(evt, QueueMilestoneHandlerError, dn, err)
	if step.OutputRef["errorMessage"] == "" {
		t.Fatalf("missing error: %+v", step.OutputRef)
	}
}

func TestBuildQueueConsumerEvent_HasStructuredStep(t *testing.T) {
	evt := &aidecisionmodels.DecisionEvent{
		EventID:     "evt_x",
		EventType:   eventtypes.OrderChanged,
		EventSource: eventtypes.EventSourceDatachanged,
	}
	ev := BuildQueueConsumerEvent(evt, QueueMilestoneHandlerDone, nil, nil, nil)
	if ev.Step == nil || ev.Step.InputRef == nil {
		t.Fatalf("Step refs: %+v", ev.Step)
	}
}
