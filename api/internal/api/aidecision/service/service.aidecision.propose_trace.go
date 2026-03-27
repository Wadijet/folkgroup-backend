// Package aidecisionsvc — Bổ sung trace queue → payload propose (Learning / audit E2E).
package aidecisionsvc

import (
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	"meta_commerce/internal/approval"
)

// MergeQueueEnvelopeIntoProposePayload đưa traceId, correlationId, eventId queue vào payload trước khi gọi approval.Propose.
// Không ghi đè khóa đã có trong payload (ưu tiên dữ liệu domain).
func MergeQueueEnvelopeIntoProposePayload(evt *aidecisionmodels.DecisionEvent, input *approval.ProposeInput) {
	if input == nil || evt == nil {
		return
	}
	if input.Payload == nil {
		input.Payload = make(map[string]interface{})
	}
	p := input.Payload
	if _, has := p["traceId"]; !has && strings.TrimSpace(evt.TraceID) != "" {
		p["traceId"] = strings.TrimSpace(evt.TraceID)
	}
	if _, has := p["correlationId"]; !has && strings.TrimSpace(evt.CorrelationID) != "" {
		p["correlationId"] = strings.TrimSpace(evt.CorrelationID)
	}
	if _, has := p["aidecisionProposeEventId"]; !has && strings.TrimSpace(evt.EventID) != "" {
		p["aidecisionProposeEventId"] = strings.TrimSpace(evt.EventID)
	}
	if pid := strings.TrimSpace(evt.ParentEventID); pid != "" {
		if _, ok := p["parentEventId"]; !ok {
			p["parentEventId"] = pid
		}
	}
	if rid := strings.TrimSpace(evt.RootEventID); rid != "" {
		if _, ok := p["rootEventId"]; !ok {
			p["rootEventId"] = rid
		}
	}
}
