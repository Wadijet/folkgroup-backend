// Package crmqueue — bản sao envelope bus AID + meta miền lên document job hàng đợi (chuẩn hóa khi gộp event một nguồn).
package crmqueue

import (
	"strings"

	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
)

// Mã module thực thi job (worker/queue sở hữu) — khớp tinh thần businessDomain / mã miền §1.2 doc E2E.
const (
	ProcessorDomainCRM   = "crm"
	ProcessorDomainOrder = "order"
	ProcessorDomainCIX   = "cix"
	ProcessorDomainAds   = "ads"
)

// Module (hoặc lớp hệ thống) phát lệnh ghi job vào queue miền.
const (
	EnqueueSourceAIDecision        = "aidecision"
	EnqueueSourceCRMDataChanged    = "crm_datachanged"
	EnqueueSourceOrderIntel        = "orderintel"
	EnqueueSourceConversationIntel = "conversationintel"
	EnqueueSourceSystemDebounce    = "system_debounce"
)

// DomainQueueBusFields — meta đồng bộ bus + phân vai trò tạo / xử lý (đề phòng gộp mọi event một collection).
type DomainQueueBusFields struct {
	EventType     string
	EventSource   string
	PipelineStage string
	// OwnerDomain — chủ nghiệp vụ payload (từ payload.ownerDomain hoặc bổ sung sau).
	OwnerDomain string
	// ProcessorDomain — module/worker thực thi job (crm | order | cix | ads).
	ProcessorDomain string
	// EnqueueSourceDomain — nơi phát lệnh enqueue (aidecision | crm_datachanged | …).
	EnqueueSourceDomain string
	// E2EStage / E2EStepID — neo catalog G1–G6 trên decision_events_queue (đồng bộ khi gộp event).
	E2EStage  string
	E2EStepID string
}

func (d DomainQueueBusFields) isEmpty() bool {
	return strings.TrimSpace(d.EventType) == "" &&
		strings.TrimSpace(d.EventSource) == "" &&
		strings.TrimSpace(d.PipelineStage) == "" &&
		strings.TrimSpace(d.OwnerDomain) == "" &&
		strings.TrimSpace(d.ProcessorDomain) == "" &&
		strings.TrimSpace(d.EnqueueSourceDomain) == "" &&
		strings.TrimSpace(d.E2EStage) == "" &&
		strings.TrimSpace(d.E2EStepID) == ""
}

// OwnerDomainFromDecisionPayload đọc payload.ownerDomain (dùng chung miền khi không có envelope đầy đủ).
func OwnerDomainFromDecisionPayload(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	v, ok := m["ownerDomain"]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// DomainQueueBusFieldsPtrFromDecisionEvent trích field từ envelope queue AID + ownerDomain payload; toàn rỗng → nil.
func DomainQueueBusFieldsPtrFromDecisionEvent(evt *aidecisionmodels.DecisionEvent) *DomainQueueBusFields {
	if evt == nil {
		return nil
	}
	et := strings.TrimSpace(evt.EventType)
	es := strings.TrimSpace(evt.EventSource)
	ps := strings.TrimSpace(evt.PipelineStage)
	od := OwnerDomainFromDecisionPayload(evt.Payload)
	e2eS := strings.TrimSpace(evt.E2EStage)
	e2eStep := strings.TrimSpace(evt.E2EStepID)
	if et == "" && es == "" && ps == "" && od == "" && e2eS == "" && e2eStep == "" {
		return nil
	}
	return &DomainQueueBusFields{
		EventType:     et,
		EventSource:   es,
		PipelineStage: ps,
		OwnerDomain:   od,
		E2EStage:      e2eS,
		E2EStepID:     e2eStep,
	}
}

// CompleteDomainJobBus gộp base (từ bus/evt) với processorDomain và enqueueSourceDomain; rỗng hoàn toàn → nil.
func CompleteDomainJobBus(base *DomainQueueBusFields, processorDomain, enqueueSourceDomain string) *DomainQueueBusFields {
	var out DomainQueueBusFields
	if base != nil {
		out = *base
	}
	if p := strings.TrimSpace(processorDomain); p != "" {
		out.ProcessorDomain = p
	}
	if e := strings.TrimSpace(enqueueSourceDomain); e != "" {
		out.EnqueueSourceDomain = e
	}
	if out.isEmpty() {
		return nil
	}
	return &out
}
