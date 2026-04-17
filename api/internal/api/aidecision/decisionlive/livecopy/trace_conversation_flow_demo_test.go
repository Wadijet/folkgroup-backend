//go:build demotrace

// Demo in luồng timeline (stdout). Chạy từ thư mục api/:
//
//	go test -tags=demotrace -v -run TestDemoTraceConversationFlow_Print ./internal/api/aidecision/decisionlive/livecopy
package livecopy

import (
	"fmt"
	"strings"
	"testing"

	"meta_commerce/internal/api/aidecision/decisionlive"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

func formatLiveEventDemo(ev decisionlive.DecisionLiveEvent) string {
	var b strings.Builder
	fmt.Fprintf(&b, "phase (mã): %s\n", ev.Phase)
	fmt.Fprintf(&b, "summary: %s\n", ev.Summary)
	fmt.Fprintf(&b, "reasoningSummary: %s\n", ev.ReasoningSummary)
	if ev.Step != nil {
		fmt.Fprintf(&b, "step.kind: %s | step.title: %s\n", ev.Step.Kind, ev.Step.Title)
	}
	fmt.Fprintf(&b, "detailBullets:\n")
	for _, line := range ev.DetailBullets {
		fmt.Fprintf(&b, "  • %s\n", line)
	}
	for _, sec := range ev.DetailSections {
		fmt.Fprintf(&b, "section «%s»:\n", sec.Title)
		for _, it := range sec.Items {
			fmt.Fprintf(&b, "  - %s\n", it)
		}
	}
	return b.String()
}

func TestDemoTraceConversationFlow_Print(t *testing.T) {
	evt := &aidecisionmodels.DecisionEvent{
		EventID:       "evt_ggoisu0q9goe",
		EventType:     eventtypes.MessageChanged,
		EventSource:   eventtypes.EventSourceDatachanged,
		CorrelationID: "corr_demo_1",
		TraceID:       "trace_lo35demo",
		Payload: map[string]interface{}{
			"conversationId": "pzl_u_712413543211467438_6277388297931268688",
			"customerId":     "cust_demo_1",
		},
	}

	caseDoc := &aidecisionmodels.DecisionCase{
		DecisionCaseID: "dcs_demo_conversation",
		CorrelationID:  "corr_demo_1",
	}

	steps := []struct {
		title string
		ev    decisionlive.DecisionLiveEvent
	}{
		{"Bước 1 — queue_processing (consumer nhận job)", BuildQueueConsumerEvent(evt, QueueMilestoneProcessingStart, nil, nil, nil)},
		{"Bước 2 — datachanged_effects (sau CRUD: tác vụ phụ)", BuildQueueConsumerEvent(evt, QueueMilestoneDatachangedDone, nil, nil, nil)},
		{"Bước 3 — orchestrate (điều phối case hội thoại)", BuildOrchestrateConversationEvent(evt, caseDoc, false,
			"pzl_u_712413543211467438_6277388297931268688", "cust_demo_1", "facebook", "", true, true)},
		{"Bước 4 — queue_done (consumer xong job)", BuildQueueConsumerEvent(evt, QueueMilestoneHandlerDone, nil, nil, nil)},
	}

	var out strings.Builder
	for _, s := range steps {
		out.WriteString(strings.Repeat("=", 72))
		out.WriteString("\n")
		out.WriteString(s.title)
		out.WriteString("\n")
		out.WriteString(strings.Repeat("=", 72))
		out.WriteString("\n")
		out.WriteString(formatLiveEventDemo(s.ev))
		out.WriteString("\n")
	}
	fmt.Print(out.String())
}
