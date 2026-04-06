// Package worker — Điều phối datachanged: một cửa build ApplyContext + datachangedsidefx.Run; logic enqueue từng miền nằm trong */datachanged/sidefx_register.go.
package worker

import (
	"context"
	"strings"
	"time"

	"meta_commerce/internal/api/aidecision/crmqueue"
	"meta_commerce/internal/api/aidecision/datachangedrouting"
	"meta_commerce/internal/api/aidecision/datachangedsidefx"
	"meta_commerce/internal/api/aidecision/eventintake"
	"meta_commerce/internal/api/aidecision/eventtypes"
	aidecisionmodels "meta_commerce/internal/api/aidecision/models"
	aidecisionsvc "meta_commerce/internal/api/aidecision/service"
	cixdec "meta_commerce/internal/api/conversationintel/datachanged"
	crmdec "meta_commerce/internal/api/crm/datachanged"
	crmvc "meta_commerce/internal/api/crm/service"
	"meta_commerce/internal/api/events"
	_ "meta_commerce/internal/api/meta/datachanged" // sidefx_register: meta_ads_profile
	orderdatachanged "meta_commerce/internal/api/order/datachanged"
	orderinteldec "meta_commerce/internal/api/orderintel/datachanged"
	rptdec "meta_commerce/internal/api/report/datachanged"
	"meta_commerce/internal/global"
	"meta_commerce/internal/logger"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// orderIntelTrailingDebounce — Order intelligence từ pc_pos_orders: mặc định 5 phút; Realtime/gấp → ngay.
const orderIntelTrailingDebounce = 5 * time.Minute

// cixIntelTrailingDebounce — CIX từ fb_message_items: mặc định 1 phút.
const cixIntelTrailingDebounce = 1 * time.Minute

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

// applyDatachangedSideEffects — chỉ điều phối: policy/debounce → chỉ xếp crm_pending_merge; intel CRM sau merge worker + crm.intelligence.recompute_requested.
func applyDatachangedSideEffects(ctx context.Context, svc *aidecisionsvc.AIDecisionService, evt *aidecisionmodels.DecisionEvent) error {
	if evt == nil || evt.Payload == nil || evt.EventSource != eventtypes.EventSourceDatachanged {
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
	// Chiếu đơn Pancake → commerce_orders (canonical) trước policy intel — Order Intel đọc từ commerce_orders.
	if src == global.MongoDB_ColNames.PcPosOrders {
		if err := orderdatachanged.SyncCommerceOrderFromPancakeDataChange(ctx, e); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"eventId": evt.EventID, "sourceCollection": src,
			}).Warn("📋 [COMMERCE_ORDER] Không đồng bộ commerce_orders từ pc_pos_orders")
		}
	}
	orgHex := evt.OrgID
	if orgHex == "" && !evt.OwnerOrganizationID.IsZero() {
		orgHex = evt.OwnerOrganizationID.Hex()
	}
	dec := eventintake.EvaluateDatachangedSideEffects(evt, src, idHex, orgHex)
	route := datachangedrouting.Resolve(src)
	datachangedrouting.LogApplied(ctx, evt, orgHex, dec, route)

	ingestWin, reportWin, refreshWin, ruleOK := eventintake.ResolveDatachangedDeferWindowsViaRule(ctx, evt, src, op)
	if !ruleOK {
		urgency := eventintake.ClassifyDatachangedBusinessUrgency(evt, src, op)
		ingestWin = eventintake.DeferWindowFor(urgency, eventintake.DeferChannelCrmMergeQueue)
		reportWin = eventintake.DeferWindowFor(urgency, eventintake.DeferChannelReport)
		refreshWin = eventintake.DeferWindowFor(urgency, eventintake.DeferChannelCRMRefresh)
	}
	if crmdec.IsCustomerIntelligenceSourceCollection(src) {
		if eventintake.ClassifyDatachangedBusinessUrgency(evt, src, op) == eventintake.UrgencyRealtime {
			// Giữ refreshWin từ rule / DeferWindowFor
		} else {
			refreshWin = crmdec.CustomerIntelTrailingDebounce
		}
	}

	cixIntelDefer := time.Duration(0)
	if src == global.MongoDB_ColNames.FbMessageItems {
		var rawMsg bson.M
		if m, ok := e.Document.(bson.M); ok {
			rawMsg = m
		}
		cixIntelDefer = cixIntelDeferWindow(evt, rawMsg)
	}
	orderIntelDefer := time.Duration(0)
	if src == global.MongoDB_ColNames.PcPosOrders {
		orderIntelDefer = orderIntelDeferWindow(evt, src, op)
	}

	ac := &datachangedsidefx.ApplyContext{
		Ctx:             ctx,
		Evt:             evt,
		E:               e,
		Src:             src,
		Op:              op,
		IDHex:           idHex,
		OrgHex:          orgHex,
		Dec:             dec,
		Route:           route,
		IngestWin:       ingestWin,
		ReportWin:       reportWin,
		RefreshWin:      refreshWin,
		CixIntelDefer:   cixIntelDefer,
		OrderIntelDefer: orderIntelDefer,
	}
	datachangedsidefx.Run(ac)
	// Tính lại CRM intelligence: không enqueue trực tiếp từ đây — chỉ sau CrmPendingMergeWorker (crm.intelligence.recompute_requested + debounce consumer).
	return nil
}

// resolveUnifiedIDForCrmIntelRecompute — từ payload datachanged đã hydrate (customerId / unifiedId).
func resolveUnifiedIDForCrmIntelRecompute(ctx context.Context, ownerOrgID primitive.ObjectID, payload map[string]interface{}) string {
	if ownerOrgID.IsZero() || payload == nil {
		return ""
	}
	if u, ok := payload["unifiedId"].(string); ok {
		if s := strings.TrimSpace(u); s != "" {
			return s
		}
	}
	custID := ""
	if c, ok := payload["customerId"].(string); ok {
		custID = strings.TrimSpace(c)
	}
	if custID == "" {
		return ""
	}
	svc, err := crmvc.NewCrmCustomerService()
	if err != nil {
		return ""
	}
	uid, ok := svc.ResolveUnifiedId(ctx, custID, ownerOrgID)
	if !ok {
		return ""
	}
	return strings.TrimSpace(uid)
}

// flushCrmIntelAfterIngestDue xếp crm_intel_compute (refresh) sau debounce crm.intelligence.recompute_requested.
func flushCrmIntelAfterIngestDue(ctx context.Context) {
	for _, j := range eventintake.TakeDueCrmIntelAfterIngestJobs(time.Now()) {
		ownerOID, err := primitive.ObjectIDFromHex(strings.TrimSpace(j.OrgHex))
		if err != nil || ownerOID.IsZero() {
			continue
		}
		unified := strings.TrimSpace(j.UnifiedID)
		if unified == "" {
			continue
		}
		parentID := strings.TrimSpace(j.ParentEventID)
		if parentID == "" {
			parentID = "crm_intel_after_ingest_debounce"
		}
		payload := map[string]interface{}{
			"operation":     crmqueue.CrmComputeOpRefresh,
			"unifiedId":     unified,
			"ownerOrgIdHex": ownerOID.Hex(),
		}
		if j.CausalOrderingAtMs > 0 {
			payload[crmqueue.PayloadKeyCausalOrderingAtMs] = j.CausalOrderingAtMs
		}
		if err := crmvc.EnqueueCrmIntelComputeFromDecisionEvent(ctx, parentID, ownerOID, payload); err != nil {
			logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
				"orgHex": j.OrgHex, "unifiedId": unified,
			}).Warn("📋 [CRM_INTEL_DEBOUNCE] Không xếp job crm_intel_compute sau debounce ingest")
		}
	}
}

// flushDeferredDatachangedSideEffects chạy các side-effect đã đến hạn (gọi mỗi tick consumer, trước khi lease event mới).
func flushDeferredDatachangedSideEffects(ctx context.Context, svc *aidecisionsvc.AIDecisionService) {
	flushCrmIntelAfterIngestDue(ctx)
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
			EventSource:         eventtypes.EventSourceDatachanged,
			OrgID:               j.OrgHex,
			OwnerOrganizationID: ownerOID,
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
			rptdec.RecordTouchFromDataChange(ctx, e)
		case eventintake.DeferredKindCrmMergeQueue:
			crmdec.EnqueueCrmMergeFromDataChange(ctx, e)
		case eventintake.DeferredKindCrmRefresh:
			uid := resolveUnifiedIDForCrmIntelRecompute(ctx, ownerOID, evt.Payload)
			if uid != "" {
				causalMs := events.ExtractUpdatedAtFromDoc(j.Coll, e.Document)
				if causalMs <= 0 {
					causalMs = time.Now().UnixMilli()
				}
				if _, err := crmqueue.EmitCrmIntelligenceRecomputeRequested(ctx, uid, ownerOID, j.Coll, "", causalMs); err != nil {
					logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
						"orgHex": j.OrgHex, "coll": j.Coll, "unifiedId": uid,
					}).Warn("📋 [CRM] Flush defer CRM refresh: không emit crm.intelligence.recompute_requested")
				}
			}
		case eventintake.DeferredKindOrderIntelCompute:
			if err := orderinteldec.EnqueueIntelligenceFromParentEvent(ctx, evt); err != nil {
				logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
					"orgHex": j.OrgHex, "coll": j.Coll, "idHex": j.IDHex,
				}).Warn("📋 [ORDER_INTEL] Flush defer: không xếp job order_intel_compute")
			}
		case eventintake.DeferredKindCixIntelCompute:
			if err := cixdec.EnqueueCixComputeFromDataChange(ctx, e, j.IDHex); err != nil {
				logger.GetAppLogger().WithError(err).WithFields(map[string]interface{}{
					"orgHex": j.OrgHex, "coll": j.Coll, "idHex": j.IDHex,
				}).Warn("📋 [CIX_INTEL] Flush defer: không xếp job cix_intel_compute")
			}
		}
	}
}

func orderIntelDeferWindow(evt *aidecisionmodels.DecisionEvent, src, op string) time.Duration {
	if evt != nil && evt.Payload != nil && eventintake.PayloadMarksIntelUrgent(evt.Payload) {
		return 0
	}
	if eventintake.ClassifyDatachangedBusinessUrgency(evt, src, op) == eventintake.UrgencyRealtime {
		return 0
	}
	return orderIntelTrailingDebounce
}

func cixIntelDeferWindow(evt *aidecisionmodels.DecisionEvent, rawMsg bson.M) time.Duration {
	if evt != nil && evt.Payload != nil && eventintake.PayloadMarksIntelUrgent(evt.Payload) {
		return 0
	}
	if eventintake.MessageTextMarksIntelUrgent(eventintake.ExtractFbMessageItemTextLower(rawMsg)) {
		return 0
	}
	return cixIntelTrailingDebounce
}
