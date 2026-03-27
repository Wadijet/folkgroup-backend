// Package worker — Một cửa sau hook: mọi event datachanged → consumer gọi đây trước dispatch.
// Quyết định ingest CRM / report / ads / refresh metrics — không tách luồng song song ngoài AI Decision.
package worker

import (
	"context"
	"strings"
	"time"

	"meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/crmingest"
	"meta_commerce/internal/api/aidecision/eventintake"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/api/events"
	conversationintel "meta_commerce/internal/api/conversationintel"
	metahooks "meta_commerce/internal/api/meta/hooks"
	reportsvc "meta_commerce/internal/api/report/service"
	"meta_commerce/internal/global"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// buildDataChangeEventFromSource đọc bản ghi Mongo hiện tại — dùng cho apply và flush side-effect trì hoãn.
func buildDataChangeEventFromSource(ctx context.Context, src, idHex, op string) (events.DataChangeEvent, bool) {
	idHex = strings.TrimSpace(idHex)
	src = strings.TrimSpace(src)
	if src == "" || idHex == "" {
		return events.DataChangeEvent{}, false
	}
	oid, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return events.DataChangeEvent{}, false
	}
	coll, ok := global.RegistryCollections.Get(src)
	if !ok || coll == nil {
		return events.DataChangeEvent{}, false
	}
	var raw bson.M
	if err := coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&raw); err != nil {
		return events.DataChangeEvent{}, false
	}
	if op == "" {
		op = events.OpUpdate
	}
	return events.DataChangeEvent{
		CollectionName:   src,
		Operation:        op,
		Document:         raw,
		PreviousDocument: nil,
	}, true
}

// applyDatachangedSideEffects hydrate payload → CRM ingest / Report / Ads → (tuỳ collection) xếp job refresh metrics.
// Không đăng ký OnDataChanged riêng; không gọi EnqueueCrmIngest / EmitCrmIntelligenceRefresh từ orchestrate khác.
// Trì hoặc (trailing) theo mức nghiệp vụ (eventintake.ClassifyDatachangedBusinessUrgency) + env BUSINESS_DEFER_* / DEFER_*.
func applyDatachangedSideEffects(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil || evt.EventSource != "datachanged" {
		return nil
	}
	if svc != nil {
		svc.HydrateDatachangedPayload(ctx, evt)
	}
	src, _ := evt.Payload["sourceCollection"].(string)
	if src == "" {
		return nil
	}
	idHex, _ := evt.Payload["normalizedRecordUid"].(string)
	if idHex == "" {
		idHex = evt.EntityID
	}
	idHex = strings.TrimSpace(idHex)
	if idHex == "" {
		return nil
	}
	op, _ := evt.Payload["dataChangeOperation"].(string)
	if op == "" {
		op = events.OpUpdate
	}
	e, ok := buildDataChangeEventFromSource(ctx, src, idHex, op)
	if !ok {
		return nil
	}
	orgHex := evt.OrgID
	if orgHex == "" && !evt.OwnerOrganizationID.IsZero() {
		orgHex = evt.OwnerOrganizationID.Hex()
	}
	dec := eventintake.EvaluateDatachangedSideEffects(evt, src, idHex, orgHex)
	ingestWin, reportWin, refreshWin, ruleOK := eventintake.ResolveDatachangedDeferWindowsViaRule(ctx, evt, src, op)
	if !ruleOK {
		urgency := eventintake.ClassifyDatachangedBusinessUrgency(evt, src, op)
		ingestWin = eventintake.DeferWindowFor(urgency, eventintake.DeferChannelCRMIngest)
		reportWin = eventintake.DeferWindowFor(urgency, eventintake.DeferChannelReport)
		refreshWin = eventintake.DeferWindowFor(urgency, eventintake.DeferChannelCRMRefresh)
	}

	if dec.AllowCRMIngest {
		if ingestWin > 0 {
			eventintake.ScheduleDeferredSideEffect(eventintake.DeferredKindCrmIngest, orgHex, src, idHex, ingestWin)
		} else {
			crmingest.EnqueueFromDatachangedEvent(ctx, e)
		}
	}

	if dec.AllowReport {
		if reportWin > 0 {
			eventintake.ScheduleDeferredSideEffect(eventintake.DeferredKindReport, orgHex, src, idHex, reportWin)
		} else {
			reportsvc.RecordReportTouchFromDataChange(ctx, e)
		}
	}

	if dec.AllowAds {
		metahooks.ProcessDataChangeForAdsProfile(ctx, e)
	}

	if src == global.MongoDB_ColNames.FbMessageItems {
		conversationintel.ProcessDataChangeForMessageItem(ctx, e, evt.TraceID, evt.CorrelationID, evt.OrgID)
	}

	if refreshWin > 0 {
		eventintake.ScheduleDeferredSideEffect(eventintake.DeferredKindCrmRefresh, orgHex, src, idHex, refreshWin)
		return nil
	}
	return emitCrmIntelligenceRefreshAfterDatachanged(ctx, evt)
}

// flushDeferredDatachangedSideEffects chạy các side-effect đã đến hạn (gọi mỗi tick consumer, trước khi lease event mới).
func flushDeferredDatachangedSideEffects(ctx context.Context, svc *aidecisionsvc.AIDecisionService) {
	jobs := eventintake.TakeDueDeferredSideEffectJobs(time.Now())
	if len(jobs) == 0 {
		return
	}
	for _, j := range jobs {
		ownerOID, err := primitive.ObjectIDFromHex(strings.TrimSpace(j.OrgHex))
		if err != nil || ownerOID.IsZero() {
			continue
		}
		evt := &aidecisionmodels.DecisionEvent{
			EventSource:           "datachanged",
			OrgID:                 j.OrgHex,
			OwnerOrganizationID:   ownerOID,
			Payload: map[string]interface{}{
				"sourceCollection":    j.Coll,
				"normalizedRecordUid": j.IDHex,
				"dataChangeOperation": events.OpUpdate,
			},
		}
		if svc != nil {
			svc.HydrateDatachangedPayload(ctx, evt)
		}
		e, ok := buildDataChangeEventFromSource(ctx, j.Coll, j.IDHex, events.OpUpdate)
		if !ok {
			continue
		}
		switch j.Kind {
		case eventintake.DeferredKindReport:
			reportsvc.RecordReportTouchFromDataChange(ctx, e)
		case eventintake.DeferredKindCrmIngest:
			crmingest.EnqueueFromDatachangedEvent(ctx, e)
		case eventintake.DeferredKindCrmRefresh:
			_ = emitCrmIntelligenceRefreshAfterDatachanged(ctx, evt)
		}
	}
}

// emitCrmIntelligenceRefreshAfterDatachanged xếp crm.intelligence.compute_requested (RefreshMetrics) — chỉ gọi từ applyDatachangedSideEffects.
func emitCrmIntelligenceRefreshAfterDatachanged(ctx context.Context, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil {
		return nil
	}
	ownerOrgID := evt.OwnerOrganizationID
	if ownerOrgID.IsZero() {
		return nil
	}
	src, _ := evt.Payload["sourceCollection"].(string)
	switch src {
	case global.MongoDB_ColNames.CrmCustomers:
		uid := strings.TrimSpace(payloadStrCRMRefresh(evt.Payload, "unifiedId"))
		if uid == "" {
			return nil
		}
		_, err := crmqueue.EmitCrmIntelligenceRefreshRequested(ctx, uid, ownerOrgID)
		return err
	case global.MongoDB_ColNames.PcPosOrders,
		global.MongoDB_ColNames.FbConvesations,
		global.MongoDB_ColNames.FbMessages,
		global.MongoDB_ColNames.PcPosCustomers,
		global.MongoDB_ColNames.FbCustomers:
		return emitCrmRefreshByCustomerID(ctx, evt, ownerOrgID)
	default:
		return nil
	}
}

func emitCrmRefreshByCustomerID(ctx context.Context, evt *aidecisionmodels.DecisionEvent, ownerOrgID primitive.ObjectID) error {
	custID := strings.TrimSpace(payloadStrCRMRefresh(evt.Payload, "customerId"))
	if custID == "" {
		return nil
	}
	svc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return err
	}
	unifiedID, ok := svc.ResolveUnifiedId(ctx, custID, ownerOrgID)
	if !ok || unifiedID == "" {
		return nil
	}
	_, err = crmqueue.EmitCrmIntelligenceRefreshRequested(ctx, unifiedID, ownerOrgID)
	return err
}

func payloadStrCRMRefresh(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}
